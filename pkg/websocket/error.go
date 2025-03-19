package websocket

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrBadUpgrade           = errors.New("websocket: bad upgrade")
	ErrBadHandshake         = errors.New("websocket: bad handshake")
	ErrMethodNotAllowed     = errors.New("websocket: method not allowed")
	ErrBadOpcode            = errors.New("websocket: bad opcode")
	ErrUnexpectedPayloadLen = errors.New("websocket: unexpected payloadLen")
)

func wrapError(err error) error {
	if strings.HasPrefix(err.Error(), "websocket:") {
		return err
	}
	return fmt.Errorf("websocket: %w", err)
}
