package grpcclient

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lucap9056/magic-conch-shell/core/v3/structs"
	"github.com/lucap9056/magic-conch-shell/core/v3/structs/languages"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type fakeAssistantServer struct {
	structs.UnimplementedAssistantServiceServer
	gotRequest *structs.Request
	reply      string
	err        error
	delay      time.Duration
}

func (s *fakeAssistantServer) Chat(ctx context.Context, req *structs.Request) (*structs.Response, error) {
	s.gotRequest = req
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return nil, status.FromContextError(ctx.Err()).Err()
		}
	}
	if s.err != nil {
		return nil, s.err
	}
	return &structs.Response{Reply: s.reply}, nil
}

func newTestClient(t *testing.T, fake *fakeAssistantServer, opts ...ClientOption) *Client {
	t.Helper()
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	structs.RegisterAssistantServiceServer(server, fake)
	go server.Serve(listener)
	t.Cleanup(server.Stop)

	dialer := grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
		return listener.DialContext(ctx)
	})
	client, err := NewAssistantClient("passthrough:///bufnet", append(opts, WithDialOptions(dialer))...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func TestChatSendsRequest(t *testing.T) {
	fake := &fakeAssistantServer{reply: "Yes"}
	client := newTestClient(t, fake)

	current := &structs.PromptMessage{Parts: []*structs.PromptPart{structs.NewTextPart("Should I?")}}
	history := []*structs.PromptMessage{{Parts: []*structs.PromptPart{structs.NewTextPart("earlier")}}}

	reply, err := client.Chat(context.Background(), languages.ZH, current, history)
	if err != nil {
		t.Fatal(err)
	}
	if reply != "Yes" {
		t.Errorf("reply = %q, want Yes", reply)
	}
	if got := fake.gotRequest.GetLanguage(); got != structs.Language_LANGUAGE_ZH {
		t.Errorf("language = %v, want LANGUAGE_ZH", got)
	}
	if got := fake.gotRequest.GetCurrentMessage().GetParts()[0].GetContent(); got != "Should I?" {
		t.Errorf("current message = %q", got)
	}
	if len(fake.gotRequest.GetHistoryMessages()) != 1 {
		t.Errorf("history length = %d, want 1", len(fake.gotRequest.GetHistoryMessages()))
	}
}

func TestChatReturnsStatusMessageAsReply(t *testing.T) {
	fake := &fakeAssistantServer{err: status.Error(codes.ResourceExhausted, "You are asking too fast!")}
	client := newTestClient(t, fake)

	reply, err := client.Chat(context.Background(), languages.EN, &structs.PromptMessage{}, nil)
	if err == nil {
		t.Fatal("Chat() succeeded, want error")
	}
	if reply != "You are asking too fast!" {
		t.Errorf("reply = %q, want status message", reply)
	}
	if code := status.Code(err); code != codes.ResourceExhausted {
		t.Errorf("code = %v, want ResourceExhausted", code)
	}
}

func TestChatAppliesTimeout(t *testing.T) {
	fake := &fakeAssistantServer{reply: "late", delay: 2 * time.Second}
	client := newTestClient(t, fake, WithTimeout(100*time.Millisecond))

	start := time.Now()
	_, err := client.Chat(context.Background(), languages.EN, &structs.PromptMessage{}, nil)
	if code := status.Code(err); code != codes.DeadlineExceeded {
		t.Fatalf("code = %v (err %v), want DeadlineExceeded", code, err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("Chat() took %v, timeout not applied", elapsed)
	}
}

func TestBuildDialOptionsTLS(t *testing.T) {
	missingFile := filepath.Join(t.TempDir(), "missing.pem")
	invalidPEM := filepath.Join(t.TempDir(), "invalid.pem")
	if err := os.WriteFile(invalidPEM, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		opts    []ClientOption
		wantErr bool
	}{
		{name: "insecure by default"},
		{name: "server name only enables tls", opts: []ClientOption{WithTLSServerName("core.internal")}},
		{name: "missing ca file", opts: []ClientOption{WithTLSCA(missingFile)}, wantErr: true},
		{name: "invalid ca pem", opts: []ClientOption{WithTLSCA(invalidPEM)}, wantErr: true},
		{name: "cert without key", opts: []ClientOption{WithTLSCert(missingFile, "")}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := newDefaultConfig()
			for _, opt := range tt.opts {
				opt(cfg)
			}
			_, err := cfg.buildDialOptions()
			if (err != nil) != tt.wantErr {
				t.Fatalf("buildDialOptions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildTLSConfigUsesRootCAs(t *testing.T) {
	cfg := newDefaultConfig()
	WithTLSServerName("core.internal")(cfg)
	tlsConfig, err := cfg.buildTLSConfig()
	if err != nil {
		t.Fatal(err)
	}
	if tlsConfig.ServerName != "core.internal" {
		t.Errorf("ServerName = %q", tlsConfig.ServerName)
	}
	if tlsConfig.ClientCAs != nil || tlsConfig.ClientAuth != 0 {
		t.Error("client tls config must not set server-side ClientCAs/ClientAuth")
	}
}
