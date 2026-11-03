package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lucap9056/magic-conch-shell/fileupload/internal/uploader"
)

type fakeUploader struct {
	gotMimeType string
	gotHash     string
	err         error
}

func (f *fakeUploader) Upload(ctx context.Context, file io.Reader, mimeType, fileHash string) (*uploader.Message, error) {
	f.gotMimeType = mimeType
	f.gotHash = fileHash
	if f.err != nil {
		return nil, f.err
	}
	return &uploader.Message{Key: "mcsft1.k1.token", MimeType: mimeType}, nil
}

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func uploadRequest(t *testing.T, field, filename string, content []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(field, filename)
	if err != nil {
		t.Fatal(err)
	}
	part.Write(content)
	writer.Close()
	req := httptest.NewRequest(http.MethodPost, "/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func serve(handler http.Handler, req *http.Request) (int, map[string]any) {
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	var resp map[string]any
	json.NewDecoder(recorder.Body).Decode(&resp)
	return recorder.Code, resp
}

func TestUploadPNG(t *testing.T) {
	fake := &fakeUploader{}
	code, resp := serve(newMux(fake), uploadRequest(t, "file", "a.png", pngBytes(t)))
	if code != http.StatusOK || resp["success"] != true {
		t.Fatalf("response = %d %v", code, resp)
	}
	message := resp["message"].(map[string]any)
	if message["key"] != "mcsft1.k1.token" || message["mime_type"] != "image/png" {
		t.Errorf("message = %v", message)
	}
	if fake.gotMimeType != "image/png" || len(fake.gotHash) != 64 {
		t.Errorf("uploader got mime %q hash %q", fake.gotMimeType, fake.gotHash)
	}
}

func TestUploadRejected(t *testing.T) {
	tests := []struct {
		name    string
		request func(t *testing.T) *http.Request
	}{
		{name: "unsupported type", request: func(t *testing.T) *http.Request {
			return uploadRequest(t, "file", "a.txt", []byte("hello world"))
		}},
		{name: "missing file field", request: func(t *testing.T) *http.Request {
			return uploadRequest(t, "other", "a.png", pngBytes(t))
		}},
		{name: "not multipart", request: func(t *testing.T) *http.Request {
			return httptest.NewRequest(http.MethodPost, "/upload", bytes.NewReader([]byte("{}")))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, resp := serve(newMux(&fakeUploader{}), tt.request(t))
			if code != http.StatusBadRequest || resp["success"] != false {
				t.Fatalf("response = %d %v", code, resp)
			}
		})
	}
}

func TestUploadErrorIsMasked(t *testing.T) {
	fake := &fakeUploader{err: errors.New("upload to gemini failed: secret detail")}
	code, resp := serve(newMux(fake), uploadRequest(t, "file", "a.png", pngBytes(t)))
	if code != http.StatusInternalServerError || resp["message"] != "internal server error" {
		t.Fatalf("response = %d %v", code, resp)
	}
}
