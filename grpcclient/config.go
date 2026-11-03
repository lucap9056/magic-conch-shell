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

func WithMaxMsgSize(recvSize, sendSize int) ClientOption {
	return func(c *clientConfig) {
		c.MaxRecvMsgSize = recvSize
		c.MaxSendMsgSize = sendSize
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

func WithTLSServerName(serverName string) ClientOption {
	return func(c *clientConfig) {
		c.TLS.ServerName = serverName
	}
}

func WithDialOptions(opts ...grpc.DialOption) ClientOption {
	return func(c *clientConfig) {
		c.ExtraOptions = append(c.ExtraOptions, opts...)
	}
}

func (c *clientConfig) tlsEnabled() bool {
	return c.TLS.CAPath != "" || c.TLS.CertPath != "" || c.TLS.KeyPath != "" || c.TLS.ServerName != ""
}

func (c *clientConfig) buildTLSConfig() (*tls.Config, error) {
	tlsConfig := &tls.Config{
		ServerName: c.TLS.ServerName,
		MinVersion: tls.VersionTLS12,
	}

	if c.TLS.CertPath != "" || c.TLS.KeyPath != "" {
		if c.TLS.CertPath == "" || c.TLS.KeyPath == "" {
			return nil, fmt.Errorf("both client cert and key are required for mTLS")
		}
		cert, err := tls.LoadX509KeyPair(c.TLS.CertPath, c.TLS.KeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load client key pair: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
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
		tlsConfig.RootCAs = certPool
	}

	return tlsConfig, nil
}

func (c *clientConfig) buildDialOptions() ([]grpc.DialOption, error) {
	opts := []grpc.DialOption{
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(c.MaxRecvMsgSize),
			grpc.MaxCallSendMsgSize(c.MaxSendMsgSize),
		),
	}

	if c.tlsEnabled() {
		tlsConfig, err := c.buildTLSConfig()
		if err != nil {
			return nil, err
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	return append(opts, c.ExtraOptions...), nil
}
