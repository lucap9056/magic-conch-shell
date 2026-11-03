package filetoken

import (
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const MinSecretSize = 32

var (
	ErrInvalidKeyID     = errors.New("filetoken: key id must match [A-Za-z0-9_-]{1,32}")
	ErrSecretTooShort   = fmt.Errorf("filetoken: secret must be at least %d bytes", MinSecretSize)
	ErrInvalidKeyFormat = errors.New("filetoken: key must be formatted as <id>:<base64url secret>")
)

var keyIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)

type Key struct {
	ID     string
	Secret []byte
}

func (k Key) validate() error {
	if !keyIDPattern.MatchString(k.ID) {
		return ErrInvalidKeyID
	}
	if len(k.Secret) < MinSecretSize {
		return ErrSecretTooShort
	}
	return nil
}

func ParseKey(s string) (Key, error) {
	id, encodedSecret, ok := strings.Cut(strings.TrimSpace(s), ":")
	if !ok {
		return Key{}, ErrInvalidKeyFormat
	}
	secret, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(encodedSecret, "="))
	if err != nil {
		return Key{}, fmt.Errorf("%w: %w", ErrInvalidKeyFormat, err)
	}
	key := Key{ID: id, Secret: secret}
	if err := key.validate(); err != nil {
		return Key{}, err
	}
	return key, nil
}

func ParseKeys(s string) ([]Key, error) {
	var keys []Key
	for entry := range strings.SplitSeq(s, ",") {
		if strings.TrimSpace(entry) == "" {
			continue
		}
		key, err := ParseKey(entry)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, nil
}
