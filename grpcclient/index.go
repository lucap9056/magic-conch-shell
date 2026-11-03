package grpcclient

import (
	"context"
	"fmt"
	"time"

	"github.com/lucap9056/magic-conch-shell/core/v3/structs"
	"github.com/lucap9056/magic-conch-shell/core/v3/structs/languages"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

type Client struct {
	conn    *grpc.ClientConn
	service structs.AssistantServiceClient
	timeout time.Duration
}

func NewAssistantClient(address string, opts ...ClientOption) (*Client, error) {
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

	return &Client{
		conn:    conn,
		service: structs.NewAssistantServiceClient(conn),
		timeout: cfg.Timeout,
	}, nil
}

func (c *Client) Chat(ctx context.Context, language languages.Language, currentMessage *structs.PromptMessage, historyMessages []*structs.PromptMessage) (string, error) {
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	resp, err := c.service.Chat(ctx, &structs.Request{
		Language:        languages.ToProto(language),
		CurrentMessage:  currentMessage,
		HistoryMessages: historyMessages,
	})
	if err != nil {
		return status.Convert(err).Message(), err
	}

	return resp.GetReply(), nil
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
