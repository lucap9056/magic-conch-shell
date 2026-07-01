package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/google/uuid"
	"github.com/lucap9056/go-envfile/envfile"
	"github.com/lucap9056/go-lifecycle/lifecycle"
	"github.com/lucap9056/magic-conch-shell/httpserver/httputil"
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
	CROSOrigins  string
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
		CROSOrigins:  os.Getenv("CORS_ALLOWED_ORIGINS"),
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

	_ = mime.AddExtensionType(".png", "image/png")
	_ = mime.AddExtensionType(".jpg", "image/jpeg")
	_ = mime.AddExtensionType(".jpeg", "image/jpeg")
	_ = mime.AddExtensionType(".webp", "image/webp")

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
			sendJSON(w, r, false, "file too large or invalid multipart form", http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			sendJSON(w, r, false, "missing file field", http.StatusBadRequest)
			return
		}
		defer file.Close()

		buf := make([]byte, 512)
		n, err := file.Read(buf)
		if err != nil && err != io.EOF {
			sendJSON(w, r, false, fmt.Sprintf("failed to read file buffer: %v", err), http.StatusInternalServerError)
			return
		}

		mimeType := http.DetectContentType(buf[:n])
		if mimeType == "" {
			mimeType = mime.TypeByExtension(filepath.Ext(header.Filename))
		}

		if mimeType == "" {
			sendJSON(w, r, false, "unknown mime type", http.StatusBadRequest)
			return
		}

		_, err = file.Seek(0, io.SeekStart)
		if err != nil {
			sendJSON(w, r, false, fmt.Sprintf("failed to reset file pointer: %v", err), http.StatusInternalServerError)
			return
		}

		uploadCtx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		geminiFile, err := genaiClient.UploadFile(uploadCtx, "", file, &genai.UploadFileOptions{
			MIMEType: mimeType,
		})
		if err != nil {
			sendJSON(w, r, false, fmt.Sprintf("upload to gemini failed: %v", err), http.StatusInternalServerError)
			return
		}

		rdxKey := "rdx://" + cfg.RdxHostname + "/" + uuid.New().String()
		hash := sha256.Sum256([]byte(rdxKey))
		cacheKey := redisKeyPrefix + hex.EncodeToString(hash[:])

		setCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := redisClient.Set(setCtx, cacheKey, geminiFile.URI, cacheTTL).Err(); err != nil {
			sendJSON(w, r, false, fmt.Sprintf("redis store failed: %v", err), http.StatusInternalServerError)
			return
		}

		sendJSON(w, r, true, UploadMessage{Key: rdxKey, MimeType: mimeType}, http.StatusOK)
	})

	listener, err := httputil.NewListener(cfg.HTTPAddress)
	if err != nil {
		life.Exitln(err.Error())
		return
	}
	defer listener.Close()

	server := &http.Server{
		Addr:         cfg.HTTPAddress,
		Handler:      httputil.CORS(mux, cfg.CROSOrigins),
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

func sendJSON[T any](w http.ResponseWriter, r *http.Request, success bool, message T, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if code == http.StatusInternalServerError {
		log.Printf("[ERROR] %s %s (%s): %v", r.Method, r.URL.Path, r.RemoteAddr, message)
		json.NewEncoder(w).Encode(Response[string]{Success: false, Message: "internal server error"})
		return
	}
	json.NewEncoder(w).Encode(Response[T]{Success: success, Message: message})
}
