package grpcserver

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/lucap9056/magic-conch-shell/core/v3/structs"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
)

func freeTCPAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	listener.Close()
	return addr
}

func TestHealthCheck(t *testing.T) {
	server, err := NewGRPCServer(&mockAssistantClient{})
	if err != nil {
		t.Fatal(err)
	}
	addr := freeTCPAddress(t)
	runErr := make(chan error, 1)
	go func() { runErr <- server.Run(addr) }()
	t.Cleanup(func() {
		server.Stop()
		if err := <-runErr; err != nil {
			t.Errorf("Run() error = %v", err)
		}
	})

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := healthgrpc.NewHealthClient(conn)

	for _, service := range []string{"", structs.AssistantService_ServiceDesc.ServiceName} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		resp, err := client.Check(ctx, &healthgrpc.HealthCheckRequest{Service: service}, grpc.WaitForReady(true))
		cancel()
		if err != nil {
			t.Fatalf("Check(%q) error = %v", service, err)
		}
		if resp.Status != healthgrpc.HealthCheckResponse_SERVING {
			t.Errorf("Check(%q) status = %v, want SERVING", service, resp.Status)
		}
	}
}
