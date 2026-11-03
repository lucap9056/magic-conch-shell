package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"embed"

	"github.com/lucap9056/magic-conch-shell/core/v3/internal/imagecache"

	"github.com/lucap9056/magic-conch-shell/core/v3/assistant/randomizer"
	"github.com/lucap9056/magic-conch-shell/core/v3/structs"
	"github.com/lucap9056/magic-conch-shell/core/v3/structs/languages"

	"google.golang.org/genai"
)

//go:embed systemInstruction
var systemInstructionFS embed.FS

// Client represents the LLM service provider and its internal state for chat sessions.
type Client struct {
	genaiClient        *genai.Client
	imageCache         *imagecache.Cache
	modelName          string
	systemInstructions map[languages.Language]string
}

// NewClient initializes a new LLM client with a predefined system prompt and conversation history.
func NewClient(apiKey string, modelName string, allowedImageDomains string, opts ...Option) (*Client, error) {
	options := defaultOptions()
	for _, opt := range opts {
		opt(options)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	// Initialize Google AI SDK
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Google AI client: %w", err)
	}

	imageCache, err := imagecache.NewCache(options.cachePath, client, allowedImageDomains)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize image cache: %w", err)
	}

	sysInstructions := make(map[languages.Language]string)
	supportedLanguages := []languages.Language{languages.EN, languages.ZH}
	for _, lang := range supportedLanguages {
		fileName := fmt.Sprintf("systemInstruction/%s.txt", lang)
		data, err := systemInstructionFS.ReadFile(fileName)
		if err != nil {
			return nil, fmt.Errorf("failed to load system instruction for language %s: %w", lang, err)
		}
		sysInstructions[lang] = string(data)
	}

	return &Client{
		genaiClient:        client,
		imageCache:         imageCache,
		modelName:          modelName,
		systemInstructions: sysInstructions,
	}, nil
}

func (c *Client) GenerateResponse(ctx context.Context, language languages.Language, newMessage *structs.PromptMessage, historyMessages []*structs.PromptMessage) (string, error) {

	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{genai.NewPartFromText(c.systemInstructions[language])},
		},
		ResponseMIMEType: "application/json",
		ResponseSchema:   callsSchema,
	}

	contents := make([]*genai.Content, 0, len(historyMessages)+1)

	for i, historyMessage := range historyMessages {
		message, err := c.convertMessageToContent(historyMessage)
		if err != nil {
			log.Printf("error: failed to convert history message: %v", err)
			return "System error: Failed to process conversation history.", fmt.Errorf("failed to convert history message at index %d: %w", i, err)
		}
		contents = append(contents, message)
	}

	msg, err := c.convertMessageToContent(newMessage)
	if err != nil {
		log.Printf("error: failed to convert new message: %v", err)
		return "System error: Failed to process the user message.", fmt.Errorf("failed to convert new message: %w", err)
	}

	contents = append(contents, msg)

	// Request generation from Gemini
	resp, err := c.genaiClient.Models.GenerateContent(ctx, c.modelName, contents, config)
	if err != nil {
		return c.handleAPIError(err)
	}

	// Validate response content
	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil || len(resp.Candidates[0].Content.Parts) == 0 {
		return "The shell remains silent. Please try rephrasing your question.", nil
	}

	// Extract text from parts
	var sb strings.Builder
	for _, part := range resp.Candidates[0].Content.Parts {
		if part != nil && !part.Thought {
			sb.WriteString(part.Text)
		}
	}

	return executeCalls(sb.String())
}

func executeCalls(llmOutput string) (string, error) {
	var calls []randomizer.Call
	if err := json.Unmarshal([]byte(llmOutput), &calls); err != nil {
		return "Failed to process selection logic.", fmt.Errorf("failed to decode calls: %w", err)
	}

	result, err := randomizer.Execute(calls)
	if errors.Is(err, randomizer.ErrNoResult) {
		return "I don't know how to choose from that.", nil
	}
	if err != nil {
		return "Failed to process selection logic.", err
	}
	return result, nil
}

func (c *Client) VerifyModel(ctx context.Context) error {
	target := strings.TrimPrefix(c.modelName, "models/")
	for model, err := range c.genaiClient.Models.All(ctx) {
		if err != nil {
			return fmt.Errorf("failed to list models: %w", err)
		}
		if strings.TrimPrefix(model.Name, "models/") == target {
			return nil
		}
	}
	return fmt.Errorf("model %q is not available for this API key", c.modelName)
}

// convertMessageToContent maps domain Message structs to Google AI SDK Content structs.
func (c *Client) convertMessageToContent(message *structs.PromptMessage) (*genai.Content, error) {
	parts := make([]*genai.Part, len(message.Parts))

	for j, part := range message.Parts {
		switch part.Type {
		case structs.PartTypeText:
			parts[j] = genai.NewPartFromText(part.Content)
		case structs.PartTypeImage:
			fileData, err := c.fetchAndCacheFileData(part.MimeType, part.Content)
			if err != nil {
				return nil, fmt.Errorf("failed to process image part: %w", err)
			}
			parts[j] = fileData
		default:
			return nil, fmt.Errorf("unsupported part type: %s", part.Type)
		}
	}

	return genai.NewContentFromParts(parts, genai.RoleUser), nil
}

// fetchAndCacheFileData retrieves image data from URL and returns a file URI part for the SDK.
func (c *Client) fetchAndCacheFileData(mimeType string, url string) (*genai.Part, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	fileUri, err := c.imageCache.Fetch(ctx, mimeType, url)
	if err != nil {
		log.Printf("error: failed to fetch file data from cache for URL %s: %v", url, err)
		return nil, fmt.Errorf("failed to fetch file data from cache: %w", err)
	}

	return genai.NewPartFromURI(fileUri, mimeType), nil
}

// handleAPIError converts Google API errors into user-friendly messages and logs technical details.
func (c *Client) handleAPIError(err error) (string, error) {
	var gErr genai.APIError
	if errors.As(err, &gErr) {
		log.Printf("error: Google API error: Code %d, Message: %s", gErr.Code, gErr.Message)
		switch gErr.Code {
		case 429:
			return "You are asking too fast! Please wait a moment.", err
		case 503:
			return "The AI service is currently overloaded. Try again later.", err
		case 400:
			return "Invalid request format sent to AI. Check your input.", err
		}
	}
	log.Printf("error: unexpected communication error: %v", err)
	return "An unexpected error occurred while communicating with the AI.", err
}
