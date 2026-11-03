package grpcserver

import (
	"context"
	"errors"
	"testing"

	"github.com/lucap9056/magic-conch-shell/core/v3/structs"
	"github.com/lucap9056/magic-conch-shell/core/v3/structs/languages"
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
		name    string
		reply   string
		err     error
		wantErr bool
	}{
		{
			name:    "Success",
			reply:   "Hello",
			err:     nil,
			wantErr: false,
		},
		{
			name:    "Error",
			reply:   "",
			err:     errors.New("assistant error"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockAssistantClient{reply: tt.reply, err: tt.err}
			service := NewService(mock)

			req := &structs.Request{
				CurrentMessage:  &structs.PromptMessage{},
				HistoryMessages: []*structs.PromptMessage{},
			}

			resp, err := service.Chat(context.Background(), req)
			if (err != nil) != tt.wantErr {
				t.Errorf("AssistantService.Chat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if resp == nil {
				t.Fatal("Response is nil")
			}
			if resp.Reply != tt.reply {
				t.Errorf("AssistantService.Chat() got reply = %v, want %v", resp.Reply, tt.reply)
			}
		})
	}
}

func TestChatNormalizesLanguage(t *testing.T) {
	tests := map[string]languages.Language{
		"en":    languages.EN,
		"EN":    languages.EN,
		"zh":    languages.ZH,
		"zh-TW": languages.ZH,
		"zh_CN": languages.ZH,
		"fr":    languages.EN,
		"":      languages.EN,
	}
	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			mock := &mockAssistantClient{}
			req := &structs.Request{Language: input, CurrentMessage: &structs.PromptMessage{}}
			if _, err := NewService(mock).Chat(context.Background(), req); err != nil {
				t.Fatal(err)
			}
			if mock.gotLanguage != want {
				t.Errorf("language %q forwarded as %q, want %q", input, mock.gotLanguage, want)
			}
		})
	}
}
