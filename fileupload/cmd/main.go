package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/google/uuid"
	"github.com/lucap9056/go-envfile/envfile"
	"github.com/lucap9056/go-lifecycle/lifecycle"
	"github.com/redis/go-redis/v9"
	"google.golang.org/api/option"
)

const (
	redisKeyPrefix     = "image_cache:"
	cacheTTL           = 24 * time.Hour
	maxFileSize        = 20 * 1024 * 1024
	defaultRdxHostname = "rediscache"
)

type Config struct {
	HTTPAddress  string
	GeminiAPIKey string
	RedisURL     string
	RdxHostname  string
}

func loadConfig() Config {
	rdxHostname := os.Getenv("RDX_HOSTNAME")
	if rdxHostname == "" {
		rdxHostname = defaultRdxHostname
	}
	return Config{
		HTTPAddress:  os.Getenv("HTTP_ADDRESS"),
		GeminiAPIKey: os.Getenv("GEMINI_API_KEY"),
		RedisURL:     os.Getenv("REDIS_URL"),
		RdxHostname:  rdxHostname,
	}
}

type Response[T any] struct {
	Success bool `json:"success"`
	Message T    `json:"message"`
}

type UploadMessage struct {
	Key      string `json:"key"`
	MimeType string `json:"mime_type"`
}

func main() {
	envfile.Load()
	life := lifecycle.New()

	cfg := loadConfig()

	ctx := context.Background()

	genaiClient, err := genai.NewClient(ctx, option.WithAPIKey(cfg.GeminiAPIKey))
	if err != nil {
		life.Exitln(err.Error())
		return
	}
	defer genaiClient.Close()

	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		life.Exitln(err.Error())
		return
	}
	redisClient := redis.NewClient(redisOpts)
	defer redisClient.Close()

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	mux.HandleFunc("POST /upload", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxFileSize)
		if err := r.ParseMultipartForm(maxFileSize); err != nil {
			sendJSON(w, false, "file too large or invalid multipart form", http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			sendJSON(w, false, "missing file field", http.StatusBadRequest)
			return
		}
		defer file.Close()

		mimeType := header.Header.Get("Content-Type")
		if mimeType == "" {
			mimeType = "image/jpeg"
		}

		uploadCtx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		geminiFile, err := genaiClient.UploadFile(uploadCtx, "", file, &genai.UploadFileOptions{
			MIMEType: mimeType,
		})
		if err != nil {
			sendJSON(w, false, fmt.Sprintf("upload to gemini failed: %v", err), http.StatusInternalServerError)
			return
		}

		rdxKey := "rdx://" + cfg.RdxHostname + "/" + uuid.New().String()
		hash := sha256.Sum256([]byte(rdxKey))
		cacheKey := redisKeyPrefix + hex.EncodeToString(hash[:])

		setCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := redisClient.Set(setCtx, cacheKey, geminiFile.URI, cacheTTL).Err(); err != nil {
			sendJSON(w, false, fmt.Sprintf("redis store failed: %v", err), http.StatusInternalServerError)
			return
		}

		sendJSON(w, true, UploadMessage{Key: rdxKey, MimeType: mimeType}, http.StatusOK)
	})

	listener, err := newListener(cfg.HTTPAddress)
	if err != nil {
		life.Exitln(err.Error())
		return
	}
	defer listener.Close()

	server := &http.Server{
		Addr:         cfg.HTTPAddress,
		Handler:      mux,
		ReadTimeout:  35 * time.Second,
		WriteTimeout: 40 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			life.Exitln(err.Error())
		}
	}()

	life.OnExit(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Println(err.Error())
			server.Close()
		}
	})

	life.Wait()
}

func newListener(addr string) (net.Listener, error) {
	if after, ok := strings.CutPrefix(addr, "unix://"); ok {
		if err := os.MkdirAll(filepath.Dir(after), 0777); err != nil {
			return nil, err
		}
		temp := after + ".temp"
		os.Remove(temp)
		os.Remove(after)
		l, err := net.Listen("unix", temp)
		if err != nil {
			return nil, err
		}
		if err := os.Chmod(temp, 0666); err != nil {
			l.Close()
			os.Remove(temp)
			return nil, err
		}
		if err := os.Rename(temp, after); err != nil {
			l.Close()
			os.Remove(temp)
			return nil, err
		}
		return l, nil
	}
	return net.Listen("tcp", addr)
}

func sendJSON[T any](w http.ResponseWriter, success bool, message T, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(Response[T]{Success: success, Message: message})
}
