package websocket

import (
	"bytes"
	"testing"

	"github.com/prafitradimas/websocket/pkg/websocket"
)

func TestFrame(t *testing.T) {
	buf := bytes.NewBuffer(make([]byte, 0, 512))
	writer := websocket.NewFrameWriter(websocket.OpcodeTextFrame, buf, make([]byte, 512), false)

	data := []byte("Hello WebSocket")
	errCh := make(chan error, 1)
	go func(errCh chan error) {
		t.Logf("Sending data: %s\n", data)
		_, err := writer.Write(data)
		errCh <- err
	}(errCh)

	if err := <-errCh; err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Log("Reading data")
	reader := websocket.NewFrameReader(buf)
	if reader == nil {
		t.Fatal("reader should not be nil")
	}

	if reader.Err() != nil {
		t.Fatalf("unexpected error: %v", reader.Err())
	}

	if bytes.Equal(data, reader.Data()) {
		t.Fatalf("expected %s, got %s", data, reader.Data())
	}
}
