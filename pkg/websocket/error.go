package websocket

import (
	"errors"
	"fmt"
)

var (
	ErrBadHandshake     = errors.New("websocket: bad handshake")
	ErrMethodNotAllowed = errors.New("websocket: method not allowed")
)

func wrapError(err error) error {
	return fmt.Errorf("websocket: %w", err)
}
