package uploader

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeStore struct {
	uploads atomic.Int32
	delay   time.Duration
	err     error
}

func (s *fakeStore) Upload(ctx context.Context, file io.Reader, mimeType string) (string, error) {
	s.uploads.Add(1)
	time.Sleep(s.delay)
	if s.err != nil {
		return "", s.err
	}
	io.Copy(io.Discard, file)
	return "https://generativelanguage.googleapis.com/v1beta/files/abc", nil
}

type fakeSigner struct{}

func (fakeSigner) Sign(uri, mimeType string) (string, error) {
	return "mcsft1.k1." + mimeType + "." + uri, nil
}

type fakeCache struct {
	mu     sync.Mutex
	tokens map[string]string
	ttls   map[string]time.Duration
}

func newFakeCache() *fakeCache {
	return &fakeCache{tokens: map[string]string{}, ttls: map[string]time.Duration{}}
}

func (c *fakeCache) Get(ctx context.Context, key string) (string, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	token, ok := c.tokens[key]
	return token, ok, nil
}

func (c *fakeCache) Set(ctx context.Context, key string, token string, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tokens[key] = token
	c.ttls[key] = ttl
	return nil
}

func TestFirstUploadReturnsSignedToken(t *testing.T) {
	u := New(&fakeStore{}, fakeSigner{}, nil, time.Hour)

	msg, err := u.Upload(context.Background(), strings.NewReader("png"), "image/png", "hash1")
	if err != nil {
		t.Fatal(err)
	}
	if want := "mcsft1.k1.image/png.https://generativelanguage.googleapis.com/v1beta/files/abc"; msg.Key != want || msg.MimeType != "image/png" {
		t.Errorf("msg = %+v", msg)
	}
}

func TestCachedTokenSkipsUpload(t *testing.T) {
	store := &fakeStore{}
	cache := newFakeCache()
	u := New(store, fakeSigner{}, cache, time.Hour)

	first, err := u.Upload(context.Background(), strings.NewReader("png"), "image/png", "hash1")
	if err != nil {
		t.Fatal(err)
	}
	second, err := u.Upload(context.Background(), strings.NewReader("png"), "image/png", "hash1")
	if err != nil {
		t.Fatal(err)
	}
	if store.uploads.Load() != 1 {
		t.Errorf("uploads = %d, want 1", store.uploads.Load())
	}
	if first.Key != second.Key {
		t.Errorf("cached token %q differs from original %q", second.Key, first.Key)
	}
	for key, ttl := range cache.ttls {
		if ttl != time.Hour-cacheTTLSafety {
			t.Errorf("cache ttl for %s = %v, want token ttl minus safety margin", key, ttl)
		}
	}
}

func TestDifferentMimeTypesAreCachedSeparately(t *testing.T) {
	store := &fakeStore{}
	u := New(store, fakeSigner{}, newFakeCache(), time.Hour)

	png, _ := u.Upload(context.Background(), strings.NewReader("x"), "image/png", "same")
	jpeg, _ := u.Upload(context.Background(), strings.NewReader("x"), "image/jpeg", "same")
	if store.uploads.Load() != 2 || png.Key == jpeg.Key {
		t.Errorf("uploads = %d, png = %q, jpeg = %q", store.uploads.Load(), png.Key, jpeg.Key)
	}
}

func TestConcurrentUploadsAreDeduplicated(t *testing.T) {
	store := &fakeStore{delay: 100 * time.Millisecond}
	u := New(store, fakeSigner{}, nil, time.Hour)

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if _, err := u.Upload(context.Background(), strings.NewReader("png"), "image/png", "hash1"); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if store.uploads.Load() != 1 {
		t.Errorf("uploads = %d, want 1", store.uploads.Load())
	}
}

func TestCanceledRequestDoesNotAbortUpload(t *testing.T) {
	u := New(&fakeStore{}, fakeSigner{}, nil, time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := u.Upload(ctx, strings.NewReader("png"), "image/png", "hash1"); err != nil {
		t.Fatalf("Upload() with canceled request context error = %v", err)
	}
}

func TestUploadErrorIsNotCached(t *testing.T) {
	cache := newFakeCache()
	u := New(&fakeStore{err: errors.New("quota exceeded")}, fakeSigner{}, cache, time.Hour)

	if _, err := u.Upload(context.Background(), strings.NewReader("png"), "image/png", "hash1"); err == nil {
		t.Fatal("Upload() succeeded, want error")
	}
	if len(cache.tokens) != 0 {
		t.Errorf("cache = %v, want empty", cache.tokens)
	}
}
