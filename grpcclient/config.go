package grpcclient

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

type clientConfig struct {
	Timeout        time.Duration
	MaxRecvMsgSize int
	MaxSendMsgSize int

	TLS struct {
		ServerName string
		CAPath     string
		CertPath   string
		KeyPath    string
	}

	ExtraOptions []grpc.DialOption
}

type ClientOption func(*clientConfig)

func newDefaultConfig() *clientConfig {
	return &clientConfig{
		Timeout:        5 * time.Second,
		MaxRecvMsgSize: 4 * 1024 * 1024, // 4MB
		MaxSendMsgSize: 4 * 1024 * 1024, // 4MB
	}
}

func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *clientConfig) {
		c.Timeout = timeout
	}
}

func WithTLSCA(caPath string) ClientOption {
	return func(c *clientConfig) {
		c.TLS.CAPath = caPath
	}
}

func WithTLSCert(certPath, keyPath string) ClientOption {
	return func(c *clientConfig) {
		c.TLS.CertPath = certPath
		c.TLS.KeyPath = keyPath
	}
}

func WithDialOptions(opts ...grpc.DialOption) ClientOption {
	return func(c *clientConfig) {
		c.ExtraOptions = append(c.ExtraOptions, opts...)
	}
}

func (c *clientConfig) buildDialOptions() ([]grpc.DialOption, error) {
	opts := []grpc.DialOption{
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(c.MaxRecvMsgSize),
			grpc.MaxCallSendMsgSize(c.MaxSendMsgSize),
		),
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
		opts = append(opts, grpc.WithTransportCredentials(creds))

	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	opts = append(opts, c.ExtraOptions...)

	return opts, nil
}
