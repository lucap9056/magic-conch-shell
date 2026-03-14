package main

import (
	"os"

	"github.com/lucap9056/magic-conch-shell/core/assistant"
	"github.com/lucap9056/magic-conch-shell/discord/discordclient"

	"github.com/lucap9056/go-envfile/envfile"
	"github.com/lucap9056/go-lifecycle/lifecycle"
)

func main() {
	envfile.Load()

	life := lifecycle.New()

	apiKey := os.Getenv("LLM_API_KEY")
	modelName := os.Getenv("MODEL_NAME")
	allowedImageDomains := os.Getenv("ALLOWED_IMAGE_DOMAINS")
	discordToken := os.Getenv("DISCORD_TOKEN")

	asst, err := assistant.NewClient(apiKey, modelName, allowedImageDomains)
	if err != nil {
		life.Exitln(err)
		return
	}

	discord, err := discordclient.New(discordToken, asst.GenerateResponse)
	if err != nil {
		life.Exitln(err)
		return
	}

	if err := discord.Open(); err != nil {
		life.Exitln(err)
		return
	}

	defer discord.Close()

	life.Wait()
}
