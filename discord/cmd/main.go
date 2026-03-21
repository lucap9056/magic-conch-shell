package main

import (
	"context"
	"os"
	"time"

	"github.com/lucap9056/magic-conch-shell/discord/discordclient"
	"github.com/lucap9056/magic-conch-shell/grpcclient/v2"

	"github.com/lucap9056/go-envfile/envfile"
	"github.com/lucap9056/go-lifecycle/lifecycle"
)

func main() {
	envfile.Load()

	life := lifecycle.New()

	discordToken := os.Getenv("DISCORD_TOKEN")
	grpcAddress := os.Getenv("GRPC_ADDRESS")
	grpcCAPath := os.Getenv("GRPC_TLS_CA")
	grpcCertPath := os.Getenv("GRPC_TLS_CERT")
	grpcKeyPath := os.Getenv("GRPC_TLS_KEY")

	assistantClientOptions := []grpcclient.ClientOption{}
	if grpcCAPath != "" {
		assistantClientOptions = append(assistantClientOptions, grpcclient.WithTLSCA(grpcCAPath))
	}
	if grpcCertPath != "" && grpcKeyPath != "" {
		assistantClientOptions = append(assistantClientOptions, grpcclient.WithTLSCert(grpcCertPath, grpcKeyPath))
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	llm, err := grpcclient.NewAssistantClient(ctx, grpcAddress, assistantClientOptions...)
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
