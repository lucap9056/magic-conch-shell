package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/lucap9056/magic-conch-shell/filetoken"
	"github.com/lucap9056/magic-conch-shell/fileupload/internal/uploader"
	"github.com/lucap9056/magic-conch-shell/httpserver/httputil"

	"github.com/lucap9056/go-lifecycle/v2/lifecycle"
	"github.com/lucap9056/go-lifecycle/v2/runner"
	"github.com/redis/go-redis/v9"
	"google.golang.org/genai"
)

type Config struct {
	HTTPAddress  string
	GeminiAPIKey string
	RedisURL     string
	FileTokenKey string
	CORSOrigins  string
}

func loadConfig() Config {
	return Config{
		HTTPAddress:  os.Getenv("HTTP_ADDRESS"),
		GeminiAPIKey: os.Getenv("GEMINI_API_KEY"),
		RedisURL:     os.Getenv("REDIS_URL"),
		FileTokenKey: os.Getenv("FILE_TOKEN_KEY"),
		CORSOrigins:  os.Getenv("CORS_ALLOWED_ORIGINS"),
	}
}

func main() {
	if err := runner.Run(run); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func run(life *lifecycle.Coordinator) error {
	cfg := loadConfig()

	key, err := filetoken.ParseKey(cfg.FileTokenKey)
	if err != nil {
		return fmt.Errorf("invalid FILE_TOKEN_KEY: %w", err)
	}
	signer, err := filetoken.NewSigner(key)
	if err != nil {
		return fmt.Errorf("invalid FILE_TOKEN_KEY: %w", err)
	}

	genaiClient, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  cfg.GeminiAPIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize Google AI client: %w", err)
	}

	var tokenCache uploader.TokenCache
	if cfg.RedisURL != "" {
		redisOpts, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			return fmt.Errorf("invalid REDIS_URL: %w", err)
		}
		redisClient := redis.NewClient(redisOpts)
		life.OnExit(func() {
			if err := redisClient.Close(); err != nil {
				log.Println(err)
			}
		})
		tokenCache = uploader.NewRedisTokenCache(redisClient)
	}

	imgUploader := uploader.New(uploader.NewGeminiStore(genaiClient), signer, tokenCache, filetoken.DefaultTTL)

	listener, err := httputil.NewListener(cfg.HTTPAddress)
	if err != nil {
		return err
	}

	server := &http.Server{
		Addr:         cfg.HTTPAddress,
		Handler:      httputil.CORS(newMux(imgUploader), cfg.CORSOrigins),
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

	return nil
}
