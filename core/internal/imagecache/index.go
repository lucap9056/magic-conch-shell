package imagecache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	badger "github.com/dgraph-io/badger/v4"
	"github.com/google/generative-ai-go/genai"
)

type Cache struct {
	db          *badger.DB
	client      *genai.Client
	expiration  time.Duration
	maxFileSize int64
	whitelist   *DomainWhitelist
}

func NewCache(dir string, client *genai.Client, allowedDomains string) (*Cache, error) {
	opts := badger.DefaultOptions(dir).WithLogger(nil)
	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open badger db: %w", err)
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

func (c *Cache) Close() error {
	return c.db.Close()
}

func (c *Cache) Fetch(ctx context.Context, mimeType string, urlString string) (string, error) {
	u, err := url.Parse(urlString)
	if err != nil {
		return "", fmt.Errorf("invalid url format: %w", err)
	}
	if !c.whitelist.IsAllowed(u.Hostname()) {
		return "", fmt.Errorf("domain %s is not in whitelist", u.Hostname())
	}

	key, err := c.urlToKey(urlString)
	if err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}

	var fileURI string
	err = c.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			fileURI = string(val)
			return nil
		})
	})

	if err == nil {
		return fileURI, nil
	}
	if err != badger.ErrKeyNotFound {
		return "", fmt.Errorf("badger db error: %w", err)
	}

	return c.downloadAndUploadToGemini(ctx, mimeType, urlString, key)
}

func (c *Cache) urlToKey(urlString string) (string, error) {
	u, err := url.Parse(urlString)
	if err != nil {
		return "", err
	}
	u.RawQuery = ""
	hash := sha256.Sum256([]byte(u.String()))
	return hex.EncodeToString(hash[:]), nil
}

func (c *Cache) downloadAndUploadToGemini(ctx context.Context, mimeType string, urlStr string, key string) (string, error) {
	resp, err := http.Get(urlStr)
	if err != nil {
		return "", fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status: %s", resp.Status)
	}

	if resp.ContentLength > c.maxFileSize {
		return "", fmt.Errorf("file is too large: %d bytes", resp.ContentLength)
	}

	limitReader := io.LimitReader(resp.Body, c.maxFileSize)

	file, err := c.client.UploadFile(ctx, "", limitReader, &genai.UploadFileOptions{
		DisplayName: key,
		MIMEType:    mimeType,
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to gemini: %w", err)
	}

	err = c.db.Update(func(txn *badger.Txn) error {
		e := badger.NewEntry([]byte(key), []byte(file.URI)).WithTTL(c.expiration)
		return txn.SetEntry(e)
	})
	if err != nil {
		return "", fmt.Errorf("failed to save to badger: %w", err)
	}

	return file.URI, nil
}
