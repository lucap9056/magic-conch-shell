package imagecache

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"testing/iotest"
)

func TestSizeLimitedReader(t *testing.T) {
	tests := []struct {
		name         string
		size         int
		limit        int64
		wantExceeded bool
	}{
		{name: "below limit", size: 10, limit: 16},
		{name: "exactly at limit", size: 16, limit: 16},
		{name: "one byte over", size: 17, limit: 16, wantExceeded: true},
		{name: "far over", size: 1 << 20, limit: 16, wantExceeded: true},
		{name: "empty", size: 0, limit: 16},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := bytes.Repeat([]byte{'x'}, tt.size)
			for name, source := range map[string]io.Reader{
				"plain":    bytes.NewReader(data),
				"one-byte": iotest.OneByteReader(bytes.NewReader(data)),
			} {
				reader := newSizeLimitedReader(source, tt.limit)
				got, err := io.ReadAll(reader)
				if tt.wantExceeded {
					if !errors.Is(err, ErrFileTooLarge) || !reader.exceeded {
						t.Fatalf("%s: err = %v, exceeded = %v, want ErrFileTooLarge", name, err, reader.exceeded)
					}
					if int64(len(got)) > tt.limit {
						t.Fatalf("%s: read %d bytes, limit %d", name, len(got), tt.limit)
					}
					continue
				}
				if err != nil || reader.exceeded {
					t.Fatalf("%s: err = %v, exceeded = %v", name, err, reader.exceeded)
				}
				if !bytes.Equal(got, data) {
					t.Fatalf("%s: read %d bytes, want %d", name, len(got), len(data))
				}
			}
		})
	}
}
