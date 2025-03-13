package websocket_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/prafitradimas/websocket/pkg/websocket"
)

// mustParseURL is a helper that panics on URL parsing errors.
func mustParseURL(rawurl string) *url.URL {
	u, err := url.Parse(rawurl)
	if err != nil {
		panic(err)
	}
	return u
}

func TestE2EHTTP(t *testing.T) {
	// Create a WebSocketServer instance.
	wsServer := &websocket.WebSocketServer{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		Subprotocols:    []string{"chat"},
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		wsConn, err := wsServer.Upgrade(w, r)
		if err != nil {
			http.Error(w, "upgrade failed", http.StatusInternalServerError)
			t.Errorf("Server: Upgrade failed: %v", err)
			return
		}

		msg := wsConn.ReadMessage()
		if msg.Err != nil {
			t.Errorf("Server: ReadMessage failed: %v", msg.Err)
			return
		}

		if err := wsConn.WriteMessage(msg.Opcode, msg.Data); err != nil {
			t.Errorf("Server: WriteMessage failed: %v", err)
		}
	}

	ts := httptest.NewServer(http.HandlerFunc(handler))
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")

	client := &websocket.WebSocketClient{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		Subprotocols:    []string{"chat"},
	}

	ctx := context.Background()
	wsConn, err := client.DialWithContext(ctx, mustParseURL(wsURL), nil)
	if err != nil {
		t.Fatalf("Client: Dial failed: %v", err)
	}

	testMessage := []byte("hello server")
	if err := wsConn.WriteMessage(websocket.OpcodeTextFrame, testMessage); err != nil {
		t.Fatalf("Client: WriteMessage failed: %v", err)
	}

	echoMsg := wsConn.ReadMessage()
	if echoMsg.Err != nil {
		t.Fatalf("Client: ReadMessage failed: %v", echoMsg.Err)
	}

	if !bytes.Equal(echoMsg.Data, testMessage) {
		t.Fatalf("Client: Expected echo %q, got %q", testMessage, echoMsg.Data)
	}
}
