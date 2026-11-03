package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/lucap9056/go-lifecycle/v2/lifecycle"
	"github.com/lucap9056/go-lifecycle/v2/runner"
	"github.com/lucap9056/magic-conch-shell/grpcclient/v3"
	"github.com/lucap9056/magic-conch-shell/httpserver/httputil"
)

func main() {
	if err := runner.Run(run); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func run(life *lifecycle.Coordinator) error {
	httpAddress := os.Getenv("HTTP_ADDRESS")
	grpcAddress := os.Getenv("GRPC_ADDRESS")
	corsOrigins := os.Getenv("CORS_ALLOWED_ORIGINS")

	var grpcOpts []grpcclient.ClientOption
	if cert := os.Getenv("GRPC_TLS_CERT"); cert != "" {
		grpcOpts = append(grpcOpts, grpcclient.WithTLSCert(cert, os.Getenv("GRPC_TLS_KEY")))
	}
	if ca := os.Getenv("GRPC_TLS_CA"); ca != "" {
		grpcOpts = append(grpcOpts, grpcclient.WithTLSCA(ca))
	}
	if serverName := os.Getenv("GRPC_TLS_SERVER_NAME"); serverName != "" {
		grpcOpts = append(grpcOpts, grpcclient.WithTLSServerName(serverName))
	}

	assistant, err := grpcclient.NewAssistantClient(grpcAddress, grpcOpts...)
	if err != nil {
		return err
	}
	life.OnExit(func() {
		if err := assistant.Close(); err != nil {
			log.Println(err)
		}
	})

	server := &http.Server{
		Addr:         httpAddress,
		Handler:      httputil.CORS(newMux(assistant), corsOrigins),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	listener, err := httputil.NewListener(httpAddress)
	if err != nil {
		return err
	}

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			life.Exitln(err.Error())
		}
	}()

	life.OnExit(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Println(err.Error())
			server.Close()
		}
	})

	return nil
}
