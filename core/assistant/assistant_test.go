package assistant

import (
	"bytes"
	"errors"
	"testing"
	"time"

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
		part, err := clientAt(issuedAt, key).resolveImagePart("image/jpeg", token)
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
	}{
		{
			name:      "expired token",
			client:    clientAt(issuedAt.Add(time.Hour), key),
			content:   token,
			wantReply: "An image in this conversation has expired. Please upload it again.",
		},
		{
			name:      "unknown key",
			client:    clientAt(issuedAt, otherKey),
			content:   token,
			wantReply: "The image reference is invalid. Please upload it again.",
		},
		{
			name:      "tampered token",
			client:    clientAt(issuedAt, key),
			content:   token + "x",
			wantReply: "The image reference is invalid. Please upload it again.",
		},
		{
			name:      "verifier not configured",
			client:    &Client{},
			content:   token,
			wantReply: "The image reference is invalid. Please upload it again.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.client.resolveImagePart("image/png", tt.content)
			if err == nil {
				t.Fatal("resolveImagePart() succeeded, want error")
			}
			if got := imageErrorReply(err, "fallback"); got != tt.wantReply {
				t.Errorf("imageErrorReply() = %q, want %q", got, tt.wantReply)
			}
		})
	}
}

func TestImageErrorReplyFallback(t *testing.T) {
	if got := imageErrorReply(errors.New("network down"), "fallback"); got != "fallback" {
		t.Errorf("imageErrorReply() = %q, want fallback", got)
	}
}
