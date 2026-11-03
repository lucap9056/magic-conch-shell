package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lucap9056/magic-conch-shell/core/v3/structs"
	"github.com/lucap9056/magic-conch-shell/core/v3/structs/languages"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeAssistant struct {
	reply       string
	err         error
	called      bool
	gotLanguage languages.Language
	gotCurrent  *structs.PromptMessage
	gotHistory  []*structs.PromptMessage
}

func (f *fakeAssistant) Chat(ctx context.Context, language languages.Language, currentMessage *structs.PromptMessage, historyMessages []*structs.PromptMessage) (string, error) {
	f.called = true
	f.gotLanguage = language
	f.gotCurrent = currentMessage
	f.gotHistory = historyMessages
	return f.reply, f.err
}

func post(t *testing.T, assistant Assistant, body string) (int, Response) {
	t.Helper()
	recorder := httptest.NewRecorder()
	newMux(assistant).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
	var resp Response
	if err := json.NewDecoder(recorder.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return recorder.Code, resp
}

func TestChatRequestDecoding(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		wantLanguage languages.Language
	}{
		{name: "enum name", body: `{"language":"LANGUAGE_ZH","current_message":{"parts":[{"type":"text","content":"hi"}]}}`, wantLanguage: languages.ZH},
		{name: "enum number", body: `{"language":2,"current_message":{"parts":[{"type":"text","content":"hi"}]}}`, wantLanguage: languages.ZH},
		{name: "camelCase field names", body: `{"language":"LANGUAGE_EN","currentMessage":{"parts":[{"type":"text","content":"hi"}]},"historyMessages":[{"parts":[]}]}`, wantLanguage: languages.EN},
		{name: "language omitted defaults to en", body: `{"current_message":{"parts":[{"type":"text","content":"hi"}]}}`, wantLanguage: languages.EN},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assistant := &fakeAssistant{reply: "Yes"}
			code, resp := post(t, assistant, tt.body)
			if code != http.StatusOK || !resp.Success || resp.Message != "Yes" {
				t.Fatalf("response = %d %+v", code, resp)
			}
			if assistant.gotLanguage != tt.wantLanguage {
				t.Errorf("language = %q, want %q", assistant.gotLanguage, tt.wantLanguage)
			}
			if assistant.gotCurrent.GetParts()[0].GetContent() != "hi" {
				t.Errorf("current message = %v", assistant.gotCurrent)
			}
		})
	}
}

func TestChatRequestRejected(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantCode int
	}{
		{name: "invalid json", body: `{`, wantCode: http.StatusBadRequest},
		{name: "unknown field", body: `{"current_message":{"parts":[]},"extra":1}`, wantCode: http.StatusBadRequest},
		{name: "legacy string language", body: `{"language":"zh","current_message":{"parts":[]}}`, wantCode: http.StatusBadRequest},
		{name: "missing current_message", body: `{"language":"LANGUAGE_EN"}`, wantCode: http.StatusBadRequest},
		{name: "body too large", body: `{"current_message":{"parts":[{"type":"text","content":"` + strings.Repeat("x", maxRequestBodySize) + `"}]}}`, wantCode: http.StatusRequestEntityTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assistant := &fakeAssistant{}
			code, resp := post(t, assistant, tt.body)
			if code != tt.wantCode || resp.Success {
				t.Fatalf("response = %d %+v, want %d", code, resp, tt.wantCode)
			}
			if assistant.called {
				t.Error("assistant must not be called for rejected requests")
			}
		})
	}
}

func TestChatErrorMapping(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		reply       string
		wantCode    int
		wantMessage string
	}{
		{name: "invalid argument keeps reply", err: status.Error(codes.InvalidArgument, "bad image"), reply: "bad image", wantCode: http.StatusBadRequest, wantMessage: "bad image"},
		{name: "expired image keeps reply", err: status.Error(codes.FailedPrecondition, "expired"), reply: "expired", wantCode: http.StatusBadRequest, wantMessage: "expired"},
		{name: "rate limited keeps reply", err: status.Error(codes.ResourceExhausted, "slow down"), reply: "slow down", wantCode: http.StatusTooManyRequests, wantMessage: "slow down"},
		{name: "unavailable is masked", err: status.Error(codes.Unavailable, "connection error: dial unix /var/run/core.sock"), reply: "connection error: dial unix /var/run/core.sock", wantCode: http.StatusServiceUnavailable, wantMessage: "service temporarily unavailable"},
		{name: "deadline is masked", err: status.Error(codes.DeadlineExceeded, "context deadline exceeded"), reply: "context deadline exceeded", wantCode: http.StatusGatewayTimeout, wantMessage: "request timed out"},
		{name: "internal is masked", err: status.Error(codes.Internal, "panic details"), reply: "panic details", wantCode: http.StatusInternalServerError, wantMessage: "internal server error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, resp := post(t, &fakeAssistant{reply: tt.reply, err: tt.err}, `{"current_message":{"parts":[]}}`)
			if code != tt.wantCode || resp.Success || resp.Message != tt.wantMessage {
				t.Fatalf("response = %d %+v, want %d %q", code, resp, tt.wantCode, tt.wantMessage)
			}
		})
	}
}

func TestHealth(t *testing.T) {
	recorder := httptest.NewRecorder()
	newMux(&fakeAssistant{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != "OK" {
		t.Fatalf("health = %d %q", recorder.Code, recorder.Body.String())
	}
}
