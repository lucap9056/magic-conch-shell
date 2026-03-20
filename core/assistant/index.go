/*
Package assistant provides a wrapper for the Google Generative AI (Gemini) SDK.
It is designed to act as an "Option Analyzer," processing user prompts
and returning structured results interpreted through an embedded Lua execution engine.
*/
package assistant

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	_ "embed"

	"github.com/lucap9056/magic-conch-shell/core/internal/imagecache"

	"github.com/lucap9056/magic-conch-shell/core/structs"

	"github.com/google/generative-ai-go/genai"
	lua "github.com/yuin/gopher-lua"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

//go:embed systemInstruction.txt
var systemInstruction string

//go:embed script.lua
var luaRuntimeScript string

// Client represents the LLM service provider and its internal state for chat sessions.
type Client struct {
	genaiClient *genai.Client
	imageCache  *imagecache.Cache
	modelName   string
}

// NewClient initializes a new LLM client with a predefined system prompt and conversation history.
func NewClient(apiKey string, modelName string, allowedImageDomains string) (*Client, error) {
	ctx := context.Background()

	// Initialize Google AI SDK
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Google AI client: %w", err)
	}

	imageCache, err := imagecache.NewCache("image_cache", client, allowedImageDomains)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize image cache: %w", err)
	}

	return &Client{
		genaiClient: client,
		imageCache:  imageCache,
		modelName:   modelName,
	}, nil
}

// GenerateResponse sends a message (and optional images) to the LLM and processes the Lua-formatted output.
func (c *Client) GenerateResponse(ctx context.Context, newMessage *structs.PromptMessage, historyMessages []*structs.PromptMessage) (string, error) {

	model := c.genaiClient.GenerativeModel(c.modelName)

	model.SystemInstruction = &genai.Content{
		Role:  "system",
		Parts: []genai.Part{genai.Text(systemInstruction)},
	}

	chat := model.StartChat()

	chat.History = make([]*genai.Content, len(historyMessages))

	for i, historyMessage := range historyMessages {
		message, err := c.convertMessageToContent(historyMessage)
		if err != nil {
			log.Printf("error: failed to convert history message: %v", err)
			return "System error: Failed to process conversation history.", fmt.Errorf("failed to convert history message at index %d: %w", i, err)
		}
		chat.History[i] = message
	}

	msg, err := c.convertMessageToContent(newMessage)
	if err != nil {
		log.Printf("error: failed to convert new message: %v", err)
		return "System error: Failed to process the user message.", fmt.Errorf("failed to convert new message: %w", err)
	}

	// Request generation from Gemini
	resp, err := chat.SendMessage(ctx, msg.Parts...)
	if err != nil {
		return c.handleAPIError(err)
	}

	// Validate response content
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "The shell remains silent. Please try rephrasing your question.", nil
	}

	// Extract text from parts
	var sb strings.Builder
	for _, part := range resp.Candidates[0].Content.Parts {
		if text, ok := part.(genai.Text); ok {
			sb.WriteString(string(text))
		}
	}

	llmOutput := sb.String()
	if strings.TrimSpace(llmOutput) == "unknown" {
		return "I don't know how to choose from that.", nil
	}

	return c.safeExecuteLua(llmOutput)
}

func (c *Client) safeExecuteLua(luaCode string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	L := lua.NewState(lua.Options{
		SkipOpenLibs:        true,
		CallStackSize:       128,
		MinimizeStackMemory: true,
	})
	defer L.Close()
	L.SetContext(ctx)

	L.SetMx(10 * 1024 * 1024)

	for _, lib := range []struct {
		name string
		open lua.LGFunction
	}{
		{lua.BaseLibName, lua.OpenBase},
		{lua.TabLibName, lua.OpenTable},
		{lua.StringLibName, lua.OpenString},
		{lua.MathLibName, lua.OpenMath},
	} {
		L.Push(L.NewFunction(lib.open))
		L.Push(lua.LString(lib.name))
		L.Call(1, 0)
	}

	L.GetField(L.GetGlobal("math"), "randomseed")
	L.Push(lua.LNumber(time.Now().UnixNano()))
	L.Call(1, 0)

	if err := L.DoString(luaRuntimeScript); err != nil {
		return "Failed to initialize execution engine.", err
	}

	if err := L.DoString(luaCode); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "Script timed out.", err
		}
		return "Failed to process selection logic.", err
	}

	return c.parseLuaResults(L)
}

// convertMessageToContent maps domain Message structs to Google AI SDK Content structs.
func (c *Client) convertMessageToContent(message *structs.PromptMessage) (*genai.Content, error) {
	parts := make([]genai.Part, len(message.Parts))

	for j, part := range message.Parts {
		switch part.Type {
		case structs.PartTypeText:
			parts[j] = genai.Text(part.Content)
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

	role := "user"

	return &genai.Content{
		Role:  role,
		Parts: parts,
	}, nil
}

// fetchAndCacheFileData retrieves image data from URL and returns a FileData struct for the SDK.
func (c *Client) fetchAndCacheFileData(mimeType string, url string) (genai.FileData, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	fileUri, err := c.imageCache.Fetch(ctx, mimeType, url)
	if err != nil {
		log.Printf("error: failed to fetch file data from cache for URL %s: %v", url, err)
		return genai.FileData{}, fmt.Errorf("failed to fetch file data from cache: %w", err)
	}

	return genai.FileData{
		MIMEType: mimeType,
		URI:      fileUri,
	}, nil
}

// handleAPIError converts Google API errors into user-friendly messages and logs technical details.
func (c *Client) handleAPIError(err error) (string, error) {
	var gErr *googleapi.Error
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

// parseLuaResults iterates through the Lua global table 'global_result' and formats it as a string.
func (c *Client) parseLuaResults(L *lua.LState) (string, error) {
	resultTable := L.GetGlobal("global_result")
	table, ok := resultTable.(*lua.LTable)

	if !ok || table.Len() == 0 {
		return "No options were generated.", nil
	}

	var output strings.Builder
	table.ForEach(func(_, val lua.LValue) {
		if lineTable, ok := val.(*lua.LTable); ok {
			if output.Len() > 0 {
				output.WriteByte('\n')
			}

			isFirstInLine := true
			lineTable.ForEach(func(_, item lua.LValue) {
				if !isFirstInLine {
					output.WriteByte(' ')
				}
				output.WriteString(item.String())
				isFirstInLine = false
			})
		}
	})

	finalStr := output.String()
	if finalStr == "" {
		return "The shell is empty. Try a different question.", nil
	}

	return finalStr, nil
}
