package main

import (
	"os"

	"github.com/lucap9056/magic-conch-shell/discord/discordclient"
	"github.com/lucap9056/magic-conch-shell/grpcclient"

	"github.com/lucap9056/go-envfile/envfile"
	"github.com/lucap9056/go-lifecycle/lifecycle"
)

func main() {
	envfile.Load()

	life := lifecycle.New()

	discordToken := os.Getenv("DISCORD_TOKEN")
	grpcAddress := os.Getenv("GRPC_ADDRESS")

	llm, err := grpcclient.NewAssistantClient(grpcAddress)
	if err != nil {
		life.Exitln(err)
		return
	}

	discord, err := discordclient.New(discordToken, llm.Chat)
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
