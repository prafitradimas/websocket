package websocket

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/prafitradimas/websocket/pkg/websocket"
)

func TestFrame(t *testing.T) {
	buf := make([]byte, 512)
	buffer := bytes.NewBuffer(buf)
	writer := websocket.NewFrameWriter(websocket.OpcodeTextFrame, bufio.NewWriter(buffer), true)

	data := []byte("Hello WebSocket")
	n, err := writer.Write(data)
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}
	if n != len(data) {
		t.Errorf("expected %d bytes written, got %d", len(data), n)
	}

	reader, err := websocket.NewFrameReader(bufio.NewReader(buffer))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if reader == nil {
		t.Fatal("reader should not be nil")
	}

	if reader.Len() != n {
		t.Errorf("expected %d bytes read, got %d", len(data), reader.Len())
	}

	readData := make([]byte, 0, reader.Len())
	if _, err := reader.Read(readData); err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}

	if bytes.Equal(data, readData) {
		t.Fatalf("expected %s, got %s", data, readData)
	}
}
