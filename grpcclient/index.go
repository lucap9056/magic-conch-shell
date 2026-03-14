package grpcclient

import (
	"context"
	"fmt"

	"github.com/lucap9056/magic-conch-shell/core/structs"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	service structs.AssistantServiceClient
}

func NewAssistantClient(address string) (*Client, error) {

	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("did not connect: %w", err)
	}

	client := &Client{structs.NewAssistantServiceClient(conn)}
	return client, nil
}

func (c *Client) Chat(ctx context.Context, currentMessage *structs.PromptMessage, historyMessages []*structs.PromptMessage) (string, error) {
	resp, err := c.service.Chat(ctx, &structs.Request{
		CurrentMessage:  currentMessage,
		HistoryMessages: historyMessages,
	})

	return resp.Reply, err
}
