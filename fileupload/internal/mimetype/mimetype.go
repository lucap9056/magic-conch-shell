package mimetype

import (
	"io"
	"mime"
	"net/http"
	"path/filepath"
)

func init() {
	_ = mime.AddExtensionType(".png", "image/png")
	_ = mime.AddExtensionType(".jpg", "image/jpeg")
	_ = mime.AddExtensionType(".jpeg", "image/jpeg")
	_ = mime.AddExtensionType(".webp", "image/webp")
	_ = mime.AddExtensionType(".heif", "image/heif")
	_ = mime.AddExtensionType(".heic", "image/heic")
}

var allowed = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/webp": true,
	"image/heif": true,
	"image/heic": true,
}

func Detect(rs io.ReadSeeker, filename string) (string, error) {
	buf := make([]byte, 512)
	n, err := rs.Read(buf)
	if err != nil && err != io.EOF {
		return "", err
	}

	if _, err := rs.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	mimeType := http.DetectContentType(buf[:n])
	if mimeType == "application/octet-stream" {
		mimeType = mime.TypeByExtension(filepath.Ext(filename))
	}
	return mimeType, nil
}

func IsAllowed(mimeType string) bool {
	return allowed[mimeType]
}
