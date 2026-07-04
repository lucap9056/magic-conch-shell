package uploader

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/lucap9056/magic-conch-shell/fileupload/internal/hashutil"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

const (
	redisKeyPrefix = "image_cache:"
	cacheTTL       = 24 * time.Hour
)

type Message struct {
	Key      string `json:"key"`
	MimeType string `json:"mime_type"`
}

type Uploader struct {
	redisClient *redis.Client
	genaiClient *genai.Client
	rdxHostname string
	group       singleflight.Group
}

func New(redisClient *redis.Client, genaiClient *genai.Client, rdxHostname string) *Uploader {
	return &Uploader{
		redisClient: redisClient,
		genaiClient: genaiClient,
		rdxHostname: rdxHostname,
	}
}

func (u *Uploader) Upload(file io.Reader, mimeType, fileHash string) (*Message, error) {
	rdxKey := "rdx://" + u.rdxHostname + "/" + fileHash
	cacheKey := redisKeyPrefix + hashutil.SHA256HexString(rdxKey)

	v, err, _ := u.group.Do(fileHash, func() (any, error) {
		getCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, getErr := u.redisClient.Get(getCtx, cacheKey).Result()
		cancel()
		switch getErr {
		case nil:
			return &Message{Key: rdxKey, MimeType: mimeType}, nil
		case redis.Nil:
		default:
			return nil, fmt.Errorf("redis lookup failed: %w", getErr)
		}

		uploadCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		geminiFile, upErr := u.genaiClient.UploadFile(uploadCtx, "", file, &genai.UploadFileOptions{
			MIMEType: mimeType,
		})
		if upErr != nil {
			return nil, fmt.Errorf("upload to gemini failed: %w", upErr)
		}

		setCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if setErr := u.redisClient.Set(setCtx, cacheKey, geminiFile.URI, cacheTTL).Err(); setErr != nil {
			return nil, fmt.Errorf("redis store failed: %w", setErr)
		}

		return Message{Key: rdxKey, MimeType: mimeType}, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*Message), nil
}
