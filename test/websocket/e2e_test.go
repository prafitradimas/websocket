package websocket_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/prafitradimas/websocket/pkg/websocket"
)

type wsserver struct {
	URL    string
	Wg     sync.WaitGroup
	server *httptest.Server
}

func NewServer(t *testing.T) *wsserver {
	s := &wsserver{}
	s.server = httptest.NewServer(wshandler{Server: s, Test: t})
	s.URL = "ws" + strings.TrimPrefix(s.server.URL, "http")

	return s
}

func (s *wsserver) Close() {
	s.Wg.Wait()
	s.server.Close()
}

type wshandler struct {
	Test   *testing.T
	Server *wsserver
}

func (h wshandler) messageLoop(conn websocket.WebSocket) {
	defer h.Server.Wg.Done()
	defer conn.Close()

	for {
		msg := conn.ReadMessage()
		if msg.Err != nil {
			h.Test.Fatalf("ReadMessage: %v", msg.Err)
			return
		}

		switch msg.Opcode {
		case websocket.OpcodePingFrame:
			if bytes.Equal(msg.Data, []byte("ping")) {
				if err := conn.WriteMessage(websocket.OpcodePongFrame, []byte("pong")); err != nil {
					h.Test.Fatalf("WriteMessage: %v", err)
					return
				}
			} else {
				h.Test.Fatalf("OpcodePingFrame: expect ping, found %s", msg.Data)
				return
			}
		case websocket.OpcodeTextFrame:
			if err := conn.WriteMessage(websocket.OpcodeTextFrame, msg.Data); err != nil {
				h.Test.Fatalf("WriteMessage: %v", err)
				return
			}
		case websocket.OpcodeCloseFrame:
			h.Test.Logf("Received close frame, closing connection")
			if err := conn.WriteCloseMessage(websocket.CloseNormalClosure, []byte("closed")); err != nil {
				h.Test.Logf("close failed: %v\n", err)
			} else {
				h.Test.Log("closed successfully")
			}
			return
		default:
			if msg.Err != nil {
				h.Test.Logf("default loop error: %v\n", msg.Err)
			} else {
				h.Test.Fatalf("Received unhandled opcode %s, data: %s, err: %v", msg.Opcode, msg.Data, msg.Err)
			}
			return
		}
	}
}

func (h wshandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.Server.Wg.Add(1)

	ws := &websocket.WebSocketServer{}
	conn, err := ws.Upgrade(w, r)
	if err != nil {
		h.Test.Fatalf("Upgrade: %v", err)
	}

	go h.messageLoop(conn)
}

func newURL(rawurl string) *url.URL {
	u, err := url.Parse(rawurl)
	if err != nil {
		panic(err)
	}
	return u
}

func TestEcho(t *testing.T) {
	s := NewServer(t)
	c := &websocket.WebSocketClient{}
	defer s.Close()

	ctx := t.Context()
	conn, err := c.DialWithContext(ctx, newURL(s.URL), nil)
	if err != nil {
		t.Fatalf("DialWithContext: %v", err)
	}
	defer conn.Close()

	echoMessage := []byte("Hello, World!")

	// errCh := make(chan error, 1)
	// go func(errCh chan error) {
	// 	t.Log("Sending echo message")
	// 	errCh <- conn.WriteMessage(websocket.OpcodeTextFrame, echoMessage)
	// }(errCh)

	// if err := <-errCh; err != nil {
	// 	t.Fatalf("Failed to send echo message: %v", err)
	// }

	err = conn.WriteMessage(websocket.OpcodeTextFrame, echoMessage)
	if err != nil {
		t.Fatalf("Failed to send echo message: %v", err)
	}

	t.Log("Reading echo message")
	msg := conn.ReadMessage()
	if msg.Err != nil {
		t.Fatalf("Failed to read echo message: %v", msg.Err)
	}

	if msg.Opcode != websocket.OpcodeTextFrame {
		t.Fatalf("expect opcode %s found %s", websocket.OpcodeTextFrame, msg.Opcode)
	}

	if !bytes.Equal(echoMessage, msg.Data) {
		t.Fatalf("expect data %s found %s", echoMessage, msg.Data)
	}

	err = conn.WriteCloseMessage(websocket.CloseGoingAway, []byte("close frame"))
	if err != nil {
		t.Fatalf("Failed to write close message: %v", err)
	}

	msg = conn.ReadMessage()
	if msg.Err != nil && !msg.Opcode.IsClose() {
		t.Fatalf("expect error or close frame, found: %s", msg.Opcode)
	} else {
		t.Logf("Close message: %s\n", msg.Data)
	}
}

func TestPingPong(t *testing.T) {
	s := NewServer(t)
	c := &websocket.WebSocketClient{}
	defer s.Close()

	ctx := t.Context()
	conn, err := c.DialWithContext(ctx, newURL(s.URL), nil)
	if err != nil {
		t.Fatalf("DialWithContext: %v", err)
	}

	t.Log("Sending 'ping' message")
	err = conn.WriteMessage(websocket.OpcodePingFrame, []byte("ping"))
	if err != nil {
		t.Fatalf("Failed to write ping message: %v", err)
	}
	defer conn.Close()

	t.Log("Reading 'pong' message")
	msg := conn.ReadMessage()
	if msg.Err != nil {
		t.Fatalf("Failed to read pong message: %v", msg.Err)
	}

	if msg.Opcode != websocket.OpcodePongFrame {
		t.Fatalf("expect opcode %s found: %s", websocket.OpcodePongFrame, msg.Opcode)
	}

	expectedPongMessage := []byte("pong")
	if !bytes.Equal(msg.Data, expectedPongMessage) {
		t.Fatalf("expect data: %s found %s", expectedPongMessage, msg.Data)
	}

	t.Log("Sending close frame")
	err = conn.WriteCloseMessage(websocket.CloseAbnormalClosure, []byte("close"))
	if err != nil {
		t.Fatalf("Failed to write close message: %v", err)
	}

	t.Log("Reading close frame")
	msg = conn.ReadMessage()
	if msg.Err != nil {
		t.Fatalf("Failed to read close message: %v", msg.Err)
	}

	if !msg.Opcode.IsClose() {
		t.Fatalf("expect opcode %s found %s", websocket.OpcodeCloseFrame, msg.Opcode)
	}

	t.Logf("Close message: %s\n", msg.Data)
}
