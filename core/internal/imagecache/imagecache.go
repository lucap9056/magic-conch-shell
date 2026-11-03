package imagecache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/lucap9056/magic-conch-shell/core/v3/internal/imagecache/embedcache"
	"github.com/lucap9056/magic-conch-shell/core/v3/internal/imagecache/rediscache"
	"google.golang.org/genai"
)

const expiration = 24 * time.Hour

var ErrFileTooLarge = errors.New("file is too large")

type Database interface {
	Get(key string) (string, error)
	Set(key string, value string) error
	Close() error
}

type Cache struct {
	db          Database
	client      *genai.Client
	expiration  time.Duration
	maxFileSize int64
	whitelist   *DomainWhitelist
}

func NewCache(path string, client *genai.Client, allowedDomains string) (*Cache, error) {
	db, err := initDB(path)
	if err != nil {
		return nil, err
	}

	var whitelist *DomainWhitelist
	if domains := strings.Split(allowedDomains, ","); len(domains) > 0 {
		whitelist = NewDomainWhitelist(domains)
	}

	return &Cache{
		db:          db,
		client:      client,
		expiration:  24 * time.Hour,
		maxFileSize: 20 * 1024 * 1024,
		whitelist:   whitelist,
	}, nil
}

func initDB(path string) (Database, error) {
	if strings.HasPrefix(path, "redis://") {
		return rediscache.NewStore(path, expiration)
	}
	return embedcache.NewStore(path, expiration)
}

func (c *Cache) Close() error {
	return c.db.Close()
}

func (c *Cache) Fetch(ctx context.Context, mimeType string, urlString string) (string, error) {
	u, err := url.Parse(urlString)
	if err != nil {
		return "", fmt.Errorf("invalid url format: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("unsupported url scheme %q", u.Scheme)
	}
	if !c.whitelist.IsAllowed(u.Hostname()) {
		return "", fmt.Errorf("domain %s is not in whitelist", u.Hostname())
	}

	key, err := c.urlToKey(u)
	if err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}

	fileURI, err := c.db.Get(key)
	if err == nil {
		return fileURI, nil
	}
	if err != os.ErrNotExist {
		return "", fmt.Errorf("db error: %w", err)
	}

	return c.downloadAndUploadToGemini(ctx, mimeType, urlString, key)
}

func (c *Cache) urlToKey(url *url.URL) (string, error) {
	url.RawQuery = ""
	hash := sha256.Sum256([]byte(url.String()))
	return hex.EncodeToString(hash[:]), nil
}

func (c *Cache) downloadAndUploadToGemini(ctx context.Context, mimeType string, urlStr string, key string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return "", fmt.Errorf("failed to build download request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status: %s", resp.Status)
	}

	if resp.ContentLength > c.maxFileSize {
		return "", fmt.Errorf("%w: %d bytes", ErrFileTooLarge, resp.ContentLength)
	}

	body := newSizeLimitedReader(resp.Body, c.maxFileSize)

	file, err := c.client.Files.Upload(ctx, body, &genai.UploadFileConfig{
		DisplayName: key,
		MIMEType:    mimeType,
	})
	if body.exceeded {
		if file != nil {
			c.client.Files.Delete(context.WithoutCancel(ctx), file.Name, nil)
		}
		return "", fmt.Errorf("%w: exceeds %d bytes", ErrFileTooLarge, c.maxFileSize)
	}
	if err != nil {
		return "", fmt.Errorf("failed to upload to gemini: %w", err)
	}

	if err := c.db.Set(key, file.URI); err != nil {
		return "", fmt.Errorf("failed to save to db: %w", err)
	}

	return file.URI, nil
}
