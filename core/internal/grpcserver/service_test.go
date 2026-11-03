package grpcserver

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/lucap9056/magic-conch-shell/core/v3/assistant"

	"github.com/lucap9056/magic-conch-shell/core/v3/structs"
	"github.com/lucap9056/magic-conch-shell/core/v3/structs/languages"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockAssistantClient struct {
	reply       string
	err         error
	gotLanguage languages.Language
}

func (m *mockAssistantClient) GenerateResponse(ctx context.Context, language languages.Language, newMessage *structs.PromptMessage, historyMessages []*structs.PromptMessage) (string, error) {
	m.gotLanguage = language
	return m.reply, m.err
}

func TestChat(t *testing.T) {
	tests := []struct {
		name        string
		reply       string
		err         error
		wantCode    codes.Code
		wantMessage string
	}{
		{name: "success", reply: "Hello", wantCode: codes.OK},
		{name: "rate limited", reply: "slow down", err: fmt.Errorf("%w: quota", assistant.ErrRateLimited), wantCode: codes.ResourceExhausted, wantMessage: "slow down"},
		{name: "model unavailable", reply: "overloaded", err: fmt.Errorf("%w: 503", assistant.ErrModelUnavailable), wantCode: codes.Unavailable, wantMessage: "overloaded"},
		{name: "invalid request", reply: "bad input", err: fmt.Errorf("%w: part", assistant.ErrInvalidRequest), wantCode: codes.InvalidArgument, wantMessage: "bad input"},
		{name: "invalid image", reply: "bad image", err: fmt.Errorf("%w: sig", assistant.ErrInvalidImage), wantCode: codes.InvalidArgument, wantMessage: "bad image"},
		{name: "image expired", reply: "expired", err: fmt.Errorf("%w: exp", assistant.ErrImageExpired), wantCode: codes.FailedPrecondition, wantMessage: "expired"},
		{name: "deadline exceeded", reply: "timeout", err: fmt.Errorf("call: %w", context.DeadlineExceeded), wantCode: codes.DeadlineExceeded, wantMessage: "timeout"},
		{name: "canceled", reply: "canceled", err: context.Canceled, wantCode: codes.Canceled, wantMessage: "canceled"},
		{name: "unknown error", reply: "oops", err: errors.New("boom"), wantCode: codes.Internal, wantMessage: "oops"},
		{name: "empty reply falls back", err: errors.New("boom"), wantCode: codes.Internal, wantMessage: fallbackErrorMessage},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewService(&mockAssistantClient{reply: tt.reply, err: tt.err})
			req := &structs.Request{CurrentMessage: &structs.PromptMessage{}}

			resp, err := service.Chat(context.Background(), req)
			if tt.wantCode == codes.OK {
				if err != nil {
					t.Fatalf("Chat() error = %v", err)
				}
				if resp.Reply != tt.reply {
					t.Errorf("Chat() reply = %q, want %q", resp.Reply, tt.reply)
				}
				return
			}

			if resp != nil {
				t.Errorf("Chat() resp = %v, want nil", resp)
			}
			st, ok := status.FromError(err)
			if !ok {
				t.Fatalf("Chat() error %v is not a gRPC status", err)
			}
			if st.Code() != tt.wantCode || st.Message() != tt.wantMessage {
				t.Errorf("Chat() status = (%v, %q), want (%v, %q)", st.Code(), st.Message(), tt.wantCode, tt.wantMessage)
			}
		})
	}
}

func TestChatLanguage(t *testing.T) {
	tests := []struct {
		name string
		lang structs.Language
		want languages.Language
	}{
		{name: "en", lang: structs.Language_LANGUAGE_EN, want: languages.EN},
		{name: "zh", lang: structs.Language_LANGUAGE_ZH, want: languages.ZH},
		{name: "unspecified defaults to en", lang: structs.Language_LANGUAGE_UNSPECIFIED, want: languages.EN},
		{name: "unknown value defaults to en", lang: structs.Language(99), want: languages.EN},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockAssistantClient{}
			req := &structs.Request{Language: tt.lang, CurrentMessage: &structs.PromptMessage{}}
			if _, err := NewService(mock).Chat(context.Background(), req); err != nil {
				t.Fatal(err)
			}
			if mock.gotLanguage != tt.want {
				t.Errorf("forwarded language = %q, want %q", mock.gotLanguage, tt.want)
			}
		})
	}
}
