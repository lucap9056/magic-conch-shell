package hashutil

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
)

func SHA256Hex(rs io.ReadSeeker) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, rs); err != nil {
		return "", err
	}

	if _, err := rs.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
