package grpcserver

import (
	"context"
	"errors"

	"github.com/lucap9056/magic-conch-shell/core/v3/assistant"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const fallbackErrorMessage = "An unexpected error occurred."

func statusCodeFor(err error) codes.Code {
	switch {
	case errors.Is(err, assistant.ErrRateLimited):
		return codes.ResourceExhausted
	case errors.Is(err, assistant.ErrModelUnavailable):
		return codes.Unavailable
	case errors.Is(err, assistant.ErrInvalidRequest),
		errors.Is(err, assistant.ErrInvalidImage):
		return codes.InvalidArgument
	case errors.Is(err, assistant.ErrImageExpired):
		return codes.FailedPrecondition
	case errors.Is(err, context.DeadlineExceeded):
		return codes.DeadlineExceeded
	case errors.Is(err, context.Canceled):
		return codes.Canceled
	default:
		return codes.Internal
	}
}

func toStatusError(err error, reply string) error {
	if reply == "" {
		reply = fallbackErrorMessage
	}
	return status.Error(statusCodeFor(err), reply)
}
