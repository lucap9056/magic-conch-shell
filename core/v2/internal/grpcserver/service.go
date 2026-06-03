package grpcserver

import (
	"context"
	"log"

	"github.com/lucap9056/magic-conch-shell/core/v2/structs"
	"github.com/lucap9056/magic-conch-shell/core/v2/structs/languages"
)

type AssistantClient interface {
	GenerateResponse(ctx context.Context, language languages.Language, newMessage *structs.PromptMessage, historyMessages []*structs.PromptMessage) (string, error)
}

type AssistantService struct {
	structs.UnimplementedAssistantServiceServer
	asst AssistantClient
}

func NewService(asst AssistantClient) *AssistantService {
	return &AssistantService{asst: asst}
}

func (s *AssistantService) Chat(ctx context.Context, req *structs.Request) (*structs.Response, error) {

	reply, err := s.asst.GenerateResponse(ctx, req.Language, req.CurrentMessage, req.HistoryMessages)
	if err != nil {
		log.Printf("[gRPC Server] Error: %v\n", err)
	}

	resp := &structs.Response{Reply: reply}

	return resp, nil
}
