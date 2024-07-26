package define

import (
	"errors"
)

var (
	ErrStreamIsNil = errors.New("stream is nil")

	ErrServerStopped = errors.New("server stopped")

	ErrUnsupportGame = errors.New("unsupport game")
)
