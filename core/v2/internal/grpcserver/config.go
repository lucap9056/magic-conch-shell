package grpcserver

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

type serverConfig struct {
	MaxRecvMsgSize int
	MaxSendMsgSize int

	KeepaliveParams keepalive.ServerParameters
	KeepalivePolicy keepalive.EnforcementPolicy

	TLS struct {
		ServerName string
		CertPath   string
		KeyPath    string
		CAPath     string
	}
}

type ServerOption func(*serverConfig)

func newDefaultConfig() *serverConfig {
	return &serverConfig{
		MaxRecvMsgSize: 4 * 1024 * 1024, // 4MB
		MaxSendMsgSize: 4 * 1024 * 1024, // 4MB
		KeepaliveParams: keepalive.ServerParameters{
			Time:    2 * time.Hour,
			Timeout: 20 * time.Second,
		},
		KeepalivePolicy: keepalive.EnforcementPolicy{
			MinTime:             5 * time.Minute,
			PermitWithoutStream: false,
		},
	}
}

func WithCA(caPath string) ServerOption {
	return func(c *serverConfig) {
		c.TLS.CAPath = caPath
	}
}

func WithTLS(certPath, keyPath string) ServerOption {
	return func(c *serverConfig) {
		c.TLS.CertPath = certPath
		c.TLS.KeyPath = keyPath
	}
}

func WithKeepalive(params keepalive.ServerParameters) ServerOption {
	return func(c *serverConfig) {
		c.KeepaliveParams = params
	}
}

func WithMaxMsgSize(recvSize, sendSize int) ServerOption {
	return func(c *serverConfig) {
		c.MaxRecvMsgSize = recvSize
		c.MaxSendMsgSize = sendSize
	}
}

func (c *serverConfig) buildServerOptions() ([]grpc.ServerOption, error) {
	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(c.MaxRecvMsgSize),
		grpc.MaxSendMsgSize(c.MaxSendMsgSize),
		grpc.KeepaliveParams(c.KeepaliveParams),
		grpc.KeepaliveEnforcementPolicy(c.KeepalivePolicy),
	}

	if c.TLS.CertPath != "" && c.TLS.KeyPath != "" {
		cert, err := tls.LoadX509KeyPair(c.TLS.CertPath, c.TLS.KeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load server key pair: %w", err)
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
		}

		if c.TLS.CAPath != "" {
			caCert, err := os.ReadFile(c.TLS.CAPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read CA cert: %w", err)
			}

			certPool := x509.NewCertPool()
			if !certPool.AppendCertsFromPEM(caCert) {
				return nil, fmt.Errorf("failed to append CA cert to pool")
			}

			tlsConfig.ClientCAs = certPool
			tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		}

		creds := credentials.NewTLS(tlsConfig)
		opts = append(opts, grpc.Creds(creds))

	} else {
		log.Println("WARNING: gRPC server is starting in INSECURE mode (No TLS certificates provided)")
	}

	return opts, nil
}
