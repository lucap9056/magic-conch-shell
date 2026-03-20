package grpcclient_test

import (
	"context"
	"testing"
	"time"

	"github.com/lucap9056/magic-conch-shell/core/structs"
	"github.com/lucap9056/magic-conch-shell/grpcclient"
)

func TestAssistantClient(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()

	client, err := grpcclient.NewAssistantClient(ctx, "localhost:50051")
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	message := &structs.PromptMessage{
		Parts: []*structs.PromptPart{structs.NewTextPart("Ok or Fail")},
	}

	reply, err := client.Chat(ctx, message, nil)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(reply)
}
