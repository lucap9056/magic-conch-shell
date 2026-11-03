package uploader

import (
	"context"
	"fmt"
	"io"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	uploadTimeout  = 30 * time.Second
	cacheTimeout   = 5 * time.Second
	cacheTTLSafety = 5 * time.Minute
	cacheKeyPrefix = "fileupload:token:"
)

type Message struct {
	Key      string `json:"key"`
	MimeType string `json:"mime_type"`
}

type FileStore interface {
	Upload(ctx context.Context, file io.Reader, mimeType string) (string, error)
}

type TokenSigner interface {
	Sign(uri, mimeType string) (string, error)
}

type TokenCache interface {
	Get(ctx context.Context, key string) (string, bool, error)
	Set(ctx context.Context, key string, token string, ttl time.Duration) error
}

type Uploader struct {
	store    FileStore
	signer   TokenSigner
	cache    TokenCache
	cacheTTL time.Duration
	group    singleflight.Group
}

func New(store FileStore, signer TokenSigner, cache TokenCache, tokenTTL time.Duration) *Uploader {
	return &Uploader{
		store:    store,
		signer:   signer,
		cache:    cache,
		cacheTTL: tokenTTL - cacheTTLSafety,
	}
}

func (u *Uploader) Upload(ctx context.Context, file io.Reader, mimeType, fileHash string) (*Message, error) {
	cacheKey := cacheKeyPrefix + mimeType + ":" + fileHash

	v, err, _ := u.group.Do(cacheKey, func() (any, error) {
		detached := context.WithoutCancel(ctx)

		if token, found, err := u.lookup(detached, cacheKey); err != nil {
			return nil, err
		} else if found {
			return &Message{Key: token, MimeType: mimeType}, nil
		}

		uploadCtx, cancel := context.WithTimeout(detached, uploadTimeout)
		defer cancel()
		uri, err := u.store.Upload(uploadCtx, file, mimeType)
		if err != nil {
			return nil, fmt.Errorf("upload to gemini failed: %w", err)
		}

		token, err := u.signer.Sign(uri, mimeType)
		if err != nil {
			return nil, fmt.Errorf("sign file token failed: %w", err)
		}

		if u.cache != nil && u.cacheTTL > 0 {
			setCtx, cancel := context.WithTimeout(detached, cacheTimeout)
			defer cancel()
			if err := u.cache.Set(setCtx, cacheKey, token, u.cacheTTL); err != nil {
				return nil, fmt.Errorf("cache store failed: %w", err)
			}
		}

		return &Message{Key: token, MimeType: mimeType}, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*Message), nil
}

func (u *Uploader) lookup(ctx context.Context, cacheKey string) (string, bool, error) {
	if u.cache == nil {
		return "", false, nil
	}
	getCtx, cancel := context.WithTimeout(ctx, cacheTimeout)
	defer cancel()
	token, found, err := u.cache.Get(getCtx, cacheKey)
	if err != nil {
		return "", false, fmt.Errorf("cache lookup failed: %w", err)
	}
	return token, found, nil
}
