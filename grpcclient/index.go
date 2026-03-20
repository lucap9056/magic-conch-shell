package grpcclient

import (
	"context"
	"fmt"

	"github.com/lucap9056/magic-conch-shell/core/structs"

	"google.golang.org/grpc"
)

type Client struct {
	conn    *grpc.ClientConn
	service structs.AssistantServiceClient
}

func NewAssistantClient(ctx context.Context, address string, opts ...clientOption) (*Client, error) {

	cfg := newDefaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	dialOpts, err := cfg.buildDialOptions()
	if err != nil {
		return nil, err
	}

	conn, err := grpc.NewClient(address, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("did not connect: %w", err)
	}

	client := &Client{conn, structs.NewAssistantServiceClient(conn)}
	return client, nil
}

func (c *Client) Chat(ctx context.Context, currentMessage *structs.PromptMessage, historyMessages []*structs.PromptMessage) (string, error) {
	resp, err := c.service.Chat(ctx, &structs.Request{
		CurrentMessage:  currentMessage,
		HistoryMessages: historyMessages,
	})

	if err != nil {
		return "", err
	}

	return resp.Reply, nil
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
