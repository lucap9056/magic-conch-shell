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

func NewGRPCServer(asst *assistant.Client, opts ...ServerOption) (*Server, error) {
	cfg := newDefaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	serverOpts, err := cfg.buildServerOptions()
	if err != nil {
		return nil, err
	}

	server := grpc.NewServer(serverOpts...)
	stopChan := make(chan struct{})
	structs.RegisterAssistantServiceServer(server, NewService(asst))
	return &Server{
		asst:     asst,
		stopChan: stopChan,
		server:   server,
	}, nil
}

func (s *Server) Run(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

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
