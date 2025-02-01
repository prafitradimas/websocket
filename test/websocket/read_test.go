package websocket_test

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/prafitradimas/websocket/pkg/websocket"
)

type MockReader struct {
	*bufio.Reader
	*bufio.Writer
}

func NewMockReader(data []byte) *MockReader {
	return &MockReader{
		Reader: bufio.NewReader(bytes.NewReader(data)),
	}
}

const (
	opcodeMask     byte = 0xF  // 00001111
	finMask        byte = 0x80 // 1 << 7 // 10000000
	rsv1Mask       byte = 0x40 // 1 << 6
	rsv2Mask       byte = 0x20 // 1 << 5
	rsv3Mask       byte = 0x10 // 1 << 4
	payloadLenMask byte = 0x7F
)

func createFrame(fin bool, opcode websocket.Opcode, payload []byte) []byte {
	var frame []byte
	var payloadLen uint64
	var header byte

	if fin {
		header = (1 << 7) | byte(opcode)
	} else {
		header = byte(opcode)
	}

	frame = append(frame, header)
	frame = append(frame, 0)

	payloadLen = uint64(len(payload))
	if payloadLen <= 125 {
		frame[1] = byte(payloadLen) | (1 << 7)
	} else if payloadLen <= 65535 {
		frame[1] = 126 | (1 << 7)
		extLen := make([]byte, 2)
		binary.BigEndian.PutUint16(extLen, uint16(payloadLen))
		frame = append(frame, extLen...)
	} else {
		frame[1] = 127 | (1 << 7)
		extLen := make([]byte, 8)
		binary.BigEndian.PutUint64(extLen, payloadLen)
		frame = append(frame, extLen...)
	}

	maskingKey := []byte{0x12, 0x34, 0x56, 0x78}
	frame = append(frame, maskingKey...)

	for i := 0; i < len(payload); i++ {
		frame = append(frame, payload[i]^maskingKey[i%4])
	}

	return frame
}

func TestReadMessagePingPong(t *testing.T) {
	pingPayload := []byte("ping")
	pongPayload := []byte("pong")

	pingFrame := createFrame(true, websocket.OpcodePingFrame, pingPayload)
	pongFrame := createFrame(true, websocket.OpcodePongFrame, pongPayload)

	mockData := append(pingFrame, pongFrame...)

	readwriter := NewMockReader(mockData)

	conn := websocket.NewConn(nil, readwriter.Reader, 512, 512)

	opcode, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opcode != 0x9 || string(message) != "ping" {
		t.Fatalf("expected Ping frame with 'ping' payload, got opcode: %v, message: %s", opcode, message)
	}

	opcode, message, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opcode != 0xA || string(message) != "pong" {
		t.Fatalf("expected Pong frame with 'pong' payload, got opcode: %v, message: %s", opcode, message)
	}
}

func TestReadMessageFragmentedText(t *testing.T) {
	frame1 := createFrame(false, 0x1, []byte("Hello "))
	frame2 := createFrame(true, 0x1, []byte("WebSocket!"))

	mockData := append(frame1, frame2...)

	readwriter := NewMockReader(mockData)

	conn := websocket.NewConn(nil, readwriter.Reader, 512, 512)

	opcode, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opcode != 0x1 || string(message) != "Hello WebSocket!" {
		t.Fatalf("expected Text frame with 'WebSocket!', got opcode: %v, message: %s", opcode, message)
	}
}
