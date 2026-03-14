package grpcserver

import (
	"context"
	"log"

	"github.com/lucap9056/magic-conch-shell/core/assistant"
	"github.com/lucap9056/magic-conch-shell/core/structs"
)

type AssistantService struct {
	structs.UnimplementedAssistantServiceServer
	asst *assistant.Client
}

func NewService(asst *assistant.Client) *AssistantService {
	return &AssistantService{asst: asst}
}

func (s *AssistantService) Chat(ctx context.Context, req *structs.Request) (*structs.Response, error) {

	reply, err := s.asst.GenerateResponse(ctx, req.CurrentMessage, req.HistoryMessages)
	if err != nil {
		log.Printf("[gRPC Server] Error: %v\n", err)
	}

	resp := &structs.Response{Reply: reply}

	return resp, nil
}
