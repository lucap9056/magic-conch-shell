package uploader

import (
	"context"
	"io"

	"google.golang.org/genai"
)

type GeminiStore struct {
	client *genai.Client
}

func NewGeminiStore(client *genai.Client) *GeminiStore {
	return &GeminiStore{client: client}
}

func (s *GeminiStore) Upload(ctx context.Context, file io.Reader, mimeType string) (string, error) {
	uploaded, err := s.client.Files.Upload(ctx, file, &genai.UploadFileConfig{MIMEType: mimeType})
	if err != nil {
		return "", err
	}
	return uploaded.URI, nil
}
