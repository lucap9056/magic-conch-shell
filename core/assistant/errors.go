package assistant

import "errors"

var (
	ErrRateLimited      = errors.New("rate limited by model provider")
	ErrModelUnavailable = errors.New("model provider unavailable")
	ErrInvalidRequest   = errors.New("invalid request")
	ErrImageExpired     = errors.New("image expired")
	ErrInvalidImage     = errors.New("invalid image")
)
