package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/lucap9056/magic-conch-shell/fileupload/internal/hashutil"
	"github.com/lucap9056/magic-conch-shell/fileupload/internal/mimetype"
	"github.com/lucap9056/magic-conch-shell/fileupload/internal/uploader"
)

const maxFileSize = 20 * 1024 * 1024

type Response[T any] struct {
	Success bool `json:"success"`
	Message T    `json:"message"`
}

type ImageUploader interface {
	Upload(ctx context.Context, file io.Reader, mimeType, fileHash string) (*uploader.Message, error)
}

func newMux(imgUploader ImageUploader) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	mux.HandleFunc("POST /upload", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxFileSize)
		if err := r.ParseMultipartForm(maxFileSize); err != nil {
			sendJSON(w, r, false, "file too large or invalid multipart form", http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			sendJSON(w, r, false, "missing file field", http.StatusBadRequest)
			return
		}
		defer file.Close()

		mimeType, err := mimetype.Detect(file, header.Filename)
		if err != nil {
			sendJSON(w, r, false, fmt.Sprintf("failed to read file: %v", err), http.StatusInternalServerError)
			return
		}
		if !mimetype.IsAllowed(mimeType) {
			sendJSON(w, r, false, fmt.Sprintf("unsupported file type: %s. Only PNG, JPEG, WEBP, HEIF, and HEIC are allowed", mimeType), http.StatusBadRequest)
			return
		}

		fileHash, err := hashutil.SHA256Hex(file)
		if err != nil {
			sendJSON(w, r, false, fmt.Sprintf("failed to hash file: %v", err), http.StatusInternalServerError)
			return
		}

		uploadMsg, err := imgUploader.Upload(r.Context(), file, mimeType, fileHash)
		if err != nil {
			sendJSON(w, r, false, err.Error(), http.StatusInternalServerError)
			return
		}

		sendJSON(w, r, true, uploadMsg, http.StatusOK)
	})

	return mux
}

func sendJSON[T any](w http.ResponseWriter, r *http.Request, success bool, message T, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if code == http.StatusInternalServerError {
		log.Printf("[ERROR] %s %s (%s): %v", r.Method, r.URL.Path, r.RemoteAddr, message)
		json.NewEncoder(w).Encode(Response[string]{Success: false, Message: "internal server error"})
		return
	}
	json.NewEncoder(w).Encode(Response[T]{Success: success, Message: message})
}
