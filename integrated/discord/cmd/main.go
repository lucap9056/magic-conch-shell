package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/lucap9056/magic-conch-shell/core/v3/assistant"
	"github.com/lucap9056/magic-conch-shell/discord/v2/discordclient"
	"github.com/lucap9056/magic-conch-shell/filetoken"

	"github.com/lucap9056/go-lifecycle/v2/lifecycle"
	"github.com/lucap9056/go-lifecycle/v2/runner"
)

func main() {
	if err := runner.Run(run); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func run(life *lifecycle.Coordinator) error {
	apiKey := os.Getenv("LLM_API_KEY")
	modelName := os.Getenv("MODEL_NAME")
	allowedImageDomains := os.Getenv("ALLOWED_IMAGE_DOMAINS")
	imageCachePath := os.Getenv("IMAGE_CACHE_PATH")
	fileTokenKeys := os.Getenv("FILE_TOKEN_KEYS")
	discordToken := os.Getenv("DISCORD_TOKEN")
	language := os.Getenv("LANG")

	var options []assistant.Option
	if imageCachePath != "" {
		options = append(options, assistant.WithCachePath(imageCachePath))
	}
	if fileTokenKeys != "" {
		keys, err := filetoken.ParseKeys(fileTokenKeys)
		if err != nil {
			return fmt.Errorf("invalid FILE_TOKEN_KEYS: %w", err)
		}
		verifier, err := filetoken.NewVerifier(keys)
		if err != nil {
			return fmt.Errorf("invalid FILE_TOKEN_KEYS: %w", err)
		}
		options = append(options, assistant.WithFileTokenVerifier(verifier))
	}

	asst, err := assistant.NewClient(apiKey, modelName, allowedImageDomains, options...)
	if err != nil {
		return err
	}
	life.OnExit(func() {
		if err := asst.Close(); err != nil {
			log.Printf("failed to close assistant: %v\n", err)
		}
	})

	verifyCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := asst.VerifyModel(verifyCtx); err != nil {
		return fmt.Errorf("model verification failed: %w", err)
	}
	log.Printf("Model verified: %s\n", modelName)

	discord, err := discordclient.New(discordToken, language, asst.GenerateResponse)
	if err != nil {
		return err
	}
	life.OnExit(discord.Close)

	return discord.Open()
}
