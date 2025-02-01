package websocket

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"net"
)

const (
	defaultReadBuffSize  = 4096
	defaultWriteBuffSize = 4096
)

type Payload struct {
	Opcode Opcode
	Data   []byte
	Err    error
}

type Conn interface {
	IsWsConn() bool
	ReadMessage() (opc Opcode, b []byte, err error)
}

func NewConn(connection net.Conn, reader *bufio.Reader, readBuffSize, writeBuffSize int) Conn {
	if reader == nil {
		if readBuffSize == 0 {
			readBuffSize = defaultReadBuffSize
		}

		reader = bufio.NewReaderSize(connection, readBuffSize)
	}

	return &conn{
		conn:          connection,
		reader:        reader,
		readBuffSize:  readBuffSize,
		writeBuffSize: writeBuffSize,
	}
}

type conn struct {
	conn   net.Conn
	reader *bufio.Reader

	readBuffSize  int
	writeBuffSize int
}

func (c *conn) IsWsConn() bool {
	return true
}

const (
	opcodeMask     byte = 0xF  // 00001111
	finMask        byte = 0x80 // 1 << 7 // 10000000
	rsv1Mask       byte = 0x40 // 1 << 6
	rsv2Mask       byte = 0x20 // 1 << 5
	rsv3Mask       byte = 0x10 // 1 << 4
	payloadLenMask byte = 0x7F
)

// https://datatracker.ietf.org/doc/html/rfc6455#section-5.2
func (c *conn) ReadMessage() (opc Opcode, b []byte, err error) {
	var headerBytes []byte

	reader := c.reader
	fin, masked := false, false
	var remainingRead int64 = 0

	for err == nil && !fin {
		var buf []byte
		headerBytes, err = readN(reader, 2)
		if err != nil {
			return opc, b, fmt.Errorf("websocket: %v", err)
		}

		opc = Opcode(headerBytes[0] & opcodeMask)
		fin = (headerBytes[0] & finMask) == (1 << 7)

		rsv1 := (headerBytes[0] & rsv1Mask) != 0
		rsv2 := (headerBytes[0] & rsv2Mask) != 0
		rsv3 := (headerBytes[0] & rsv3Mask) != 0
		if rsv1 || rsv2 || rsv3 {
			return opc, b, fmt.Errorf("websocket: rsv bit is set")
		}

		masked = (headerBytes[1] & (1 << 7)) == (1 << 7)
		remainingRead = int64(headerBytes[1] & payloadLenMask)

		if !opc.Valid() {
			return opc, b, fmt.Errorf("websocket: invalid opcode: %d", opc)
		}

		var maskingKey []byte
		if masked {
			maskingKey, err = readN(reader, 4)
			if err != nil {
				return opc, b, fmt.Errorf("websocket: %v", err)
			}
		}

		var extPayloadLen []byte
		if remainingRead == 126 {
			extPayloadLen, err = readN(reader, 2)
			if err != nil {
				return opc, b, fmt.Errorf("websocket: %v", err)
			}
			remainingRead = int64(binary.BigEndian.Uint16(extPayloadLen))
		} else if remainingRead == 127 {
			extPayloadLen, err = readN(reader, 8)
			if err != nil {
				return opc, b, fmt.Errorf("websocket: %v", err)
			}
			remainingRead = int64(binary.BigEndian.Uint64(extPayloadLen))
		} else if remainingRead > 127 {
			return opc, b, fmt.Errorf("websocket: invalid payload length: %d", remainingRead)
		}

		buf, err = readN(reader, int(remainingRead))
		if err != nil {
			return opc, b, fmt.Errorf("websocket: %v", err)
		}

		if masked && maskingKey != nil {
			for i := range buf {
				buf[i] ^= maskingKey[i%4]
			}
		}

		b = append(b, buf...)
	}

	return opc, b, err
}

// https://github.com/golang/go/issues/17064
func readN(r *bufio.Reader, n int) ([]byte, error) {
	buf, err := r.Peek(n)
	if err != nil {
		return buf, err
	}
	r.Discard(n)
	return buf, err
}
