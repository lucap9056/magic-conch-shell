package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/lucap9056/magic-conch-shell/core/v3/structs"
	"github.com/lucap9056/magic-conch-shell/core/v3/structs/languages"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

const maxRequestBodySize = 1 << 20

type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type Assistant interface {
	Chat(ctx context.Context, language languages.Language, currentMessage *structs.PromptMessage, historyMessages []*structs.PromptMessage) (string, error)
}

func newMux(assistant Assistant) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	mux.HandleFunc("POST /", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBodySize))
		if err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				sendJSONResponse(w, false, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			sendJSONResponse(w, false, "failed to read request body", http.StatusBadRequest)
			return
		}

		var req structs.Request
		if err := protojson.Unmarshal(body, &req); err != nil {
			sendJSONResponse(w, false, err.Error(), http.StatusBadRequest)
			return
		}

		if req.GetCurrentMessage() == nil {
			sendJSONResponse(w, false, "current_message is required", http.StatusBadRequest)
			return
		}

		language, _ := languages.FromProto(req.GetLanguage())
		reply, err := assistant.Chat(r.Context(), language, req.GetCurrentMessage(), req.GetHistoryMessages())
		if err != nil {
			httpStatus, message := chatErrorResponse(err, reply)
			log.Printf("[ERROR] %s %s (%s): %v", r.Method, r.URL.Path, r.RemoteAddr, err)
			sendJSONResponse(w, false, message, httpStatus)
			return
		}

		sendJSONResponse(w, true, reply, http.StatusOK)
	})

	return mux
}

func chatErrorResponse(err error, reply string) (int, string) {
	switch status.Code(err) {
	case codes.InvalidArgument, codes.FailedPrecondition:
		return http.StatusBadRequest, reply
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests, reply
	case codes.Unavailable:
		return http.StatusServiceUnavailable, "service temporarily unavailable"
	case codes.DeadlineExceeded, codes.Canceled:
		return http.StatusGatewayTimeout, "request timed out"
	default:
		return http.StatusInternalServerError, "internal server error"
	}
}

func sendJSONResponse(w http.ResponseWriter, success bool, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(&Response{
		Success: success,
		Message: message,
	})
}
