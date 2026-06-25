package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lucap9056/go-envfile/envfile"
	"github.com/lucap9056/go-lifecycle/lifecycle"
	"github.com/lucap9056/magic-conch-shell/core/v2/structs"
	grpcclient "github.com/lucap9056/magic-conch-shell/grpcclient/v2"
)

type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func main() {
	envfile.Load()
	life := lifecycle.New()

	httpAddress := os.Getenv("HTTP_ADDRESS")
	grpcAddress := os.Getenv("GRPC_ADDRESS")

	assistant, err := grpcclient.NewAssistantClient(context.Background(), grpcAddress)
	if err != nil {
		life.Exitln(err.Error())
		return
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	mux.HandleFunc("POST /", func(w http.ResponseWriter, r *http.Request) {
		var req structs.Request
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			sendJSONResponse(w, false, err.Error(), http.StatusBadRequest)
			return
		}

		if req.CurrentMessage == nil {
			sendJSONResponse(w, false, "current_message is required", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), time.Second*5)
		defer cancel()

		reply, err := assistant.Chat(ctx, req.Language, req.CurrentMessage, req.HistoryMessages)
		if err != nil {
			sendJSONResponse(w, false, err.Error(), http.StatusInternalServerError)
			return
		}

		sendJSONResponse(w, true, reply, http.StatusOK)
	})

	server := &http.Server{
		Addr:         httpAddress,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	listener, err := newListener(httpAddress)
	if err != nil {
		life.Exitln(err.Error())
		return
	}
	defer listener.Close()

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

	life.Wait()
}

func newListener(addr string) (net.Listener, error) {
	if after, ok := strings.CutPrefix(addr, "unix://"); ok {
		if err := os.MkdirAll(filepath.Dir(after), 0777); err != nil {
			return nil, err
		}
		temp := after + ".temp"
		os.Remove(temp)
		os.Remove(after)
		l, err := net.Listen("unix", temp)
		if err != nil {
			return nil, err
		}
		if err := os.Chmod(temp, 0666); err != nil {
			l.Close()
			os.Remove(temp)
			return nil, err
		}
		if err := os.Rename(temp, after); err != nil {
			l.Close()
			os.Remove(temp)
			return nil, err
		}
		return l, nil
	}
	return net.Listen("tcp", addr)
}

func sendJSONResponse(w http.ResponseWriter, success bool, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	response := &Response{
		Success: success,
		Message: message,
	}
	json.NewEncoder(w).Encode(response)
}
