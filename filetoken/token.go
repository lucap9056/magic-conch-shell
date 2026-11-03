package filetoken

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	Prefix     = "mcsft1."
	DefaultTTL = 47 * time.Hour
)

var (
	ErrMalformed        = errors.New("filetoken: malformed token")
	ErrUnknownKey       = errors.New("filetoken: unknown key id")
	ErrInvalidSignature = errors.New("filetoken: invalid signature")
	ErrExpired          = errors.New("filetoken: token expired")
	ErrEmptyClaim       = errors.New("filetoken: uri and mime type must not be empty")
	ErrNoKeys           = errors.New("filetoken: at least one key is required")
	ErrDuplicateKeyID   = errors.New("filetoken: duplicate key id")
	ErrInvalidTTL       = errors.New("filetoken: ttl must be positive")
)

type Claims struct {
	URI       string
	MIMEType  string
	ExpiresAt time.Time
}

type payload struct {
	URI       string `json:"u"`
	MIMEType  string `json:"m"`
	ExpiresAt int64  `json:"e"`
}

func IsToken(s string) bool {
	return strings.HasPrefix(s, Prefix)
}

func sign(secret []byte, signingInput string) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(signingInput))
	return mac.Sum(nil)
}

type Signer struct {
	key Key
	ttl time.Duration
	now func() time.Time
}

type SignerOption func(*Signer)

func WithTTL(ttl time.Duration) SignerOption {
	return func(s *Signer) {
		s.ttl = ttl
	}
}

func WithSignerClock(now func() time.Time) SignerOption {
	return func(s *Signer) {
		s.now = now
	}
}

func NewSigner(key Key, opts ...SignerOption) (*Signer, error) {
	if err := key.validate(); err != nil {
		return nil, err
	}
	s := &Signer{
		key: Key{ID: key.ID, Secret: append([]byte(nil), key.Secret...)},
		ttl: DefaultTTL,
		now: time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	if s.ttl <= 0 {
		return nil, ErrInvalidTTL
	}
	return s, nil
}

func (s *Signer) Sign(uri, mimeType string) (string, error) {
	if uri == "" || mimeType == "" {
		return "", ErrEmptyClaim
	}
	body, err := json.Marshal(payload{
		URI:       uri,
		MIMEType:  mimeType,
		ExpiresAt: s.now().Add(s.ttl).Unix(),
	})
	if err != nil {
		return "", fmt.Errorf("filetoken: encode payload: %w", err)
	}

	signingInput := Prefix + s.key.ID + "." + base64.RawURLEncoding.EncodeToString(body)
	signature := sign(s.key.Secret, signingInput)
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

type Verifier struct {
	secrets map[string][]byte
	now     func() time.Time
}

type VerifierOption func(*Verifier)

func WithVerifierClock(now func() time.Time) VerifierOption {
	return func(v *Verifier) {
		v.now = now
	}
}

func NewVerifier(keys []Key, opts ...VerifierOption) (*Verifier, error) {
	if len(keys) == 0 {
		return nil, ErrNoKeys
	}
	secrets := make(map[string][]byte, len(keys))
	for _, key := range keys {
		if err := key.validate(); err != nil {
			return nil, err
		}
		if _, exists := secrets[key.ID]; exists {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateKeyID, key.ID)
		}
		secrets[key.ID] = append([]byte(nil), key.Secret...)
	}

	v := &Verifier{secrets: secrets, now: time.Now}
	for _, opt := range opts {
		opt(v)
	}
	return v, nil
}

func (v *Verifier) Verify(token string) (Claims, error) {
	rest, ok := strings.CutPrefix(token, Prefix)
	if !ok {
		return Claims{}, ErrMalformed
	}
	segments := strings.Split(rest, ".")
	if len(segments) != 3 {
		return Claims{}, ErrMalformed
	}
	keyID, encodedBody, encodedSignature := segments[0], segments[1], segments[2]

	secret, ok := v.secrets[keyID]
	if !ok {
		return Claims{}, ErrUnknownKey
	}

	signature, err := base64.RawURLEncoding.DecodeString(encodedSignature)
	if err != nil {
		return Claims{}, ErrMalformed
	}
	expected := sign(secret, Prefix+keyID+"."+encodedBody)
	if !hmac.Equal(signature, expected) {
		return Claims{}, ErrInvalidSignature
	}

	body, err := base64.RawURLEncoding.DecodeString(encodedBody)
	if err != nil {
		return Claims{}, ErrMalformed
	}
	var p payload
	if err := json.Unmarshal(body, &p); err != nil {
		return Claims{}, ErrMalformed
	}
	if p.URI == "" || p.MIMEType == "" || p.ExpiresAt == 0 {
		return Claims{}, ErrMalformed
	}

	claims := Claims{
		URI:       p.URI,
		MIMEType:  p.MIMEType,
		ExpiresAt: time.Unix(p.ExpiresAt, 0),
	}
	if !v.now().Before(claims.ExpiresAt) {
		return Claims{}, ErrExpired
	}
	return claims, nil
}
