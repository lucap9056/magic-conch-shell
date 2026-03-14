package grpcserver

import (
	"net"

	"github.com/lucap9056/magic-conch-shell/core/assistant"
	"github.com/lucap9056/magic-conch-shell/core/structs"

	"google.golang.org/grpc"
)

type Server struct {
	structs.UnimplementedAssistantServiceServer
	server   *grpc.Server
	asst     *assistant.Client
	stopChan chan struct{}
}

func NewGRPCServer(asst *assistant.Client) *Server {
	stopChan := make(chan struct{})
	return &Server{
		asst:     asst,
		stopChan: stopChan,
	}
}

func (s *Server) Run(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	server := grpc.NewServer()
	structs.RegisterAssistantServiceServer(server, NewService(s.asst))
	s.server = server
	err = s.server.Serve(listener)
	if err != nil {
		return err
	}
	<-s.stopChan
	listener.Close()
	return nil
}
func (s *Server) Stop() {
	s.server.GracefulStop()
	close(s.stopChan)
}
