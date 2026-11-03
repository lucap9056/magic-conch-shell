package imagecache

import "io"

type sizeLimitedReader struct {
	reader    io.Reader
	remaining int64
	exceeded  bool
}

func newSizeLimitedReader(reader io.Reader, limit int64) *sizeLimitedReader {
	return &sizeLimitedReader{reader: reader, remaining: limit}
}

func (r *sizeLimitedReader) Read(p []byte) (int, error) {
	if r.exceeded {
		return 0, ErrFileTooLarge
	}
	if r.remaining <= 0 {
		var probe [1]byte
		n, err := r.reader.Read(probe[:])
		if n > 0 {
			r.exceeded = true
			return 0, ErrFileTooLarge
		}
		return 0, err
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	n, err := r.reader.Read(p)
	r.remaining -= int64(n)
	return n, err
}
