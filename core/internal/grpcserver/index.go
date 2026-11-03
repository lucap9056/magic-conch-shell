package grpcserver

import (
	"github.com/lucap9056/magic-conch-shell/core/v3/structs"
	"github.com/lucap9056/magic-conch-shell/httpserver/httputil"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
)

type Server struct {
	structs.UnimplementedAssistantServiceServer
	server   *grpc.Server
	health   *health.Server
	asst     AssistantClient
	stopChan chan struct{}
}

func NewGRPCServer(asst AssistantClient, opts ...ServerOption) (*Server, error) {
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

	healthServer := health.NewServer()
	healthServer.SetServingStatus(structs.AssistantService_ServiceDesc.ServiceName, healthgrpc.HealthCheckResponse_SERVING)
	healthgrpc.RegisterHealthServer(server, healthServer)

	return &Server{
		asst:     asst,
		stopChan: stopChan,
		server:   server,
		health:   healthServer,
	}, nil
}

func (s *Server) Run(addr string) error {
	listener, err := httputil.NewListener(addr)
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
	s.health.Shutdown()
	s.server.GracefulStop()
	close(s.stopChan)
}
