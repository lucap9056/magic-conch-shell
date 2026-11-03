package main

import (
	"log"
	"os"

	"github.com/lucap9056/magic-conch-shell/discord/v2/discordclient"
	"github.com/lucap9056/magic-conch-shell/grpcclient/v3"

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
	discordToken := os.Getenv("DISCORD_TOKEN")
	grpcAddress := os.Getenv("GRPC_ADDRESS")
	grpcCAPath := os.Getenv("GRPC_TLS_CA")
	grpcCertPath := os.Getenv("GRPC_TLS_CERT")
	grpcKeyPath := os.Getenv("GRPC_TLS_KEY")
	grpcServerName := os.Getenv("GRPC_TLS_SERVER_NAME")
	language := os.Getenv("LANG")

	assistantClientOptions := []grpcclient.ClientOption{grpcclient.WithTimeout(0)}
	if grpcCAPath != "" {
		assistantClientOptions = append(assistantClientOptions, grpcclient.WithTLSCA(grpcCAPath))
	}
	if grpcCertPath != "" && grpcKeyPath != "" {
		assistantClientOptions = append(assistantClientOptions, grpcclient.WithTLSCert(grpcCertPath, grpcKeyPath))
	}
	if grpcServerName != "" {
		assistantClientOptions = append(assistantClientOptions, grpcclient.WithTLSServerName(grpcServerName))
	}

	llm, err := grpcclient.NewAssistantClient(grpcAddress, assistantClientOptions...)
	if err != nil {
		return err
	}
	life.OnExit(func() {
		if err := llm.Close(); err != nil {
			log.Println(err)
		}
	})

	discord, err := discordclient.New(discordToken, language, llm.Chat)
	if err != nil {
		return err
	}
	life.OnExit(discord.Close)

	return discord.Open()
}
