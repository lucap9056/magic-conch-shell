package assistant

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/lucap9056/magic-conch-shell/core/v3/internal/imagecache"
	"github.com/lucap9056/magic-conch-shell/core/v3/structs"
	"github.com/lucap9056/magic-conch-shell/filetoken"
)

func TestExecuteCalls(t *testing.T) {
	tests := []struct {
		name      string
		llmOutput string
		want      string
		wantErr   bool
	}{
		{
			name:      "single call",
			llmOutput: `[{"fn":"random_text","args":{"is_repeatable":false,"n":1,"options":["only_one"]}}]`,
			want:      "only_one",
		},
		{
			name:      "multiple calls",
			llmOutput: `[{"fn":"random_text","args":{"is_repeatable":false,"n":1,"options":["first"]}},{"fn":"random_num","args":{"is_repeatable":false,"n":1,"min":2,"max":2}}]`,
			want:      "first\n2",
		},
		{
			name:      "empty array means unknown",
			llmOutput: `[]`,
			want:      "I don't know how to choose from that.",
		},
		{
			name:      "malformed json",
			llmOutput: `random_text(false, 1, "a")`,
			want:      "Failed to process selection logic.",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := executeCalls(tt.llmOutput)
			if (err != nil) != tt.wantErr {
				t.Fatalf("executeCalls() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("executeCalls() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveImagePartWithFileToken(t *testing.T) {
	const fileURI = "https://generativelanguage.googleapis.com/v1beta/files/abc123"
	key := filetoken.Key{ID: "k1", Secret: bytes.Repeat([]byte{'a'}, filetoken.MinSecretSize)}
	issuedAt := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)

	signer, err := filetoken.NewSigner(key, filetoken.WithTTL(time.Hour), filetoken.WithSignerClock(func() time.Time { return issuedAt }))
	if err != nil {
		t.Fatal(err)
	}
	token, err := signer.Sign(fileURI, "image/png")
	if err != nil {
		t.Fatal(err)
	}
	otherKey := filetoken.Key{ID: "k2", Secret: bytes.Repeat([]byte{'b'}, filetoken.MinSecretSize)}

	clientAt := func(now time.Time, keys ...filetoken.Key) *Client {
		verifier, err := filetoken.NewVerifier(keys, filetoken.WithVerifierClock(func() time.Time { return now }))
		if err != nil {
			t.Fatal(err)
		}
		return &Client{fileTokenVerifier: verifier}
	}

	t.Run("valid token uses signed uri and mime type", func(t *testing.T) {
		part, err := clientAt(issuedAt, key).resolveImagePart(context.Background(), "image/jpeg", token)
		if err != nil {
			t.Fatal(err)
		}
		if part.FileData == nil || part.FileData.FileURI != fileURI || part.FileData.MIMEType != "image/png" {
			t.Fatalf("part.FileData = %+v", part.FileData)
		}
	})

	tests := []struct {
		name      string
		client    *Client
		content   string
		wantReply string
		wantErr   error
	}{
		{
			name:      "expired token",
			client:    clientAt(issuedAt.Add(time.Hour), key),
			content:   token,
			wantReply: "An image in this conversation has expired. Please upload it again.",
			wantErr:   ErrImageExpired,
		},
		{
			name:      "unknown key",
			client:    clientAt(issuedAt, otherKey),
			content:   token,
			wantReply: "The image reference is invalid. Please upload it again.",
			wantErr:   ErrInvalidImage,
		},
		{
			name:      "tampered token",
			client:    clientAt(issuedAt, key),
			content:   token + "x",
			wantReply: "The image reference is invalid. Please upload it again.",
			wantErr:   ErrInvalidImage,
		},
		{
			name:      "verifier not configured",
			client:    &Client{},
			content:   token,
			wantReply: "The image reference is invalid. Please upload it again.",
			wantErr:   ErrInvalidImage,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.client.resolveImagePart(context.Background(), "image/png", tt.content)
			if err == nil {
				t.Fatal("resolveImagePart() succeeded, want error")
			}
			reply, classified := classifyImageError(err, "fallback")
			if reply != tt.wantReply {
				t.Errorf("classifyImageError() reply = %q, want %q", reply, tt.wantReply)
			}
			if !errors.Is(classified, tt.wantErr) {
				t.Errorf("classifyImageError() error = %v, want %v", classified, tt.wantErr)
			}
		})
	}

	t.Run("expired history image becomes placeholder", func(t *testing.T) {
		message := &structs.PromptMessage{Parts: []*structs.PromptPart{
			structs.NewTextPart("look at this"),
			structs.NewImagePart("image/png", token),
		}}
		content, err := clientAt(issuedAt.Add(time.Hour), key).convertMessageToContent(context.Background(), message, true)
		if err != nil {
			t.Fatal(err)
		}
		if got := content.Parts[1].Text; got != unavailableImagePlaceholder {
			t.Errorf("history image part text = %q, want placeholder", got)
		}
	})

	t.Run("expired current image is an error", func(t *testing.T) {
		message := &structs.PromptMessage{Parts: []*structs.PromptPart{structs.NewImagePart("image/png", token)}}
		_, err := clientAt(issuedAt.Add(time.Hour), key).convertMessageToContent(context.Background(), message, false)
		if !errors.Is(err, filetoken.ErrExpired) {
			t.Fatalf("convertMessageToContent() error = %v, want ErrExpired", err)
		}
	})
}

func TestClassifyImageError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantReply string
		wantErr   error
	}{
		{name: "file too large", err: fmt.Errorf("fetch: %w", imagecache.ErrFileTooLarge), wantReply: "The image is too large.", wantErr: ErrInvalidImage},
		{name: "domain not allowed", err: fmt.Errorf("fetch: %w", imagecache.ErrDomainNotAllowed), wantReply: "The image reference is invalid. Please upload it again.", wantErr: ErrInvalidImage},
		{name: "unsupported scheme", err: fmt.Errorf("fetch: %w", imagecache.ErrUnsupportedScheme), wantReply: "The image reference is invalid. Please upload it again.", wantErr: ErrInvalidImage},
		{name: "unrelated error keeps fallback", err: errors.New("network down"), wantReply: "fallback"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reply, classified := classifyImageError(tt.err, "fallback")
			if reply != tt.wantReply {
				t.Errorf("reply = %q, want %q", reply, tt.wantReply)
			}
			if tt.wantErr != nil && !errors.Is(classified, tt.wantErr) {
				t.Errorf("error = %v, want %v", classified, tt.wantErr)
			}
			if tt.wantErr == nil && classified != tt.err {
				t.Errorf("error = %v, want original %v", classified, tt.err)
			}
			if !errors.Is(classified, tt.err) {
				t.Errorf("classified error lost original cause %v", tt.err)
			}
		})
	}
}

func TestUnsupportedPartTypeIsInvalidRequest(t *testing.T) {
	message := &structs.PromptMessage{Parts: []*structs.PromptPart{{Type: "video", Content: "x"}}}
	_, err := (&Client{}).convertMessageToContent(context.Background(), message, false)
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("error = %v, want ErrInvalidRequest", err)
	}
}
