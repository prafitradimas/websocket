package websocket_test

import (
	"bytes"
	"net"
	"testing"

	"github.com/prafitradimas/websocket/pkg/websocket"
)

func TestWriteMessage(t *testing.T) {
	t.Log("Running: TestWriteMessage")

	t.Log("Creating net.Pipe()")
	clientConn, serverConn := net.Pipe()

	wsClient := websocket.NewConn(clientConn, 512, 512, nil, true)
	wsServer := websocket.NewConn(serverConn, 512, 512, nil, false)

	writeErrCh := make(chan error, 1)
	go func() {
		t.Log("Writing message in goroutine")
		err := wsClient.WriteMessage(websocket.OpcodePingFrame, []byte("ping"))
		writeErrCh <- err
	}()

	t.Log("Reading message")
	msg := wsServer.ReadMessage()
	if msg.Err != nil {
		t.Fatal(msg.Err)
	}

	if msg.Opcode != websocket.OpcodePingFrame {
		t.Fatalf("expect ping frame got: %s", msg.Opcode)
	}

	if !bytes.Equal([]byte("ping"), msg.Data) {
		t.Fatalf("expect data: ping, got: %s", msg.Data)
	}
}
