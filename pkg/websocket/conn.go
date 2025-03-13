package websocket

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"net"
)

const (
	maxInt = int(^uint(0) >> 1)

	defaultReadBuffSize  = 4096
	defaultWriteBuffSize = 4096

	frameMinHeaderSize    = 2
	frameMaskSize         = 4
	frameMaxPayloadLength = 8
	frameMaxHeaderSize    = frameMinHeaderSize + frameMaxPayloadLength + frameMaskSize
)

type WebSocket interface {
	WriteMessage(opc Opcode, data []byte) error

	ReadMessage() Message

	LocalAddr() net.Addr

	RemoteAddr() net.Addr

	IsClosed() bool
}

func NewConn(connection net.Conn, readBuffSize, writeBuffSize int, writeBuf []byte, isClient bool) WebSocket {
	if readBuffSize == 0 {
		readBuffSize = defaultReadBuffSize
	}

	reader := bufio.NewReaderSize(connection, readBuffSize)

	return &webSocketConn{
		conn:          connection,
		reader:        reader,
		readBuffSize:  readBuffSize,
		writeBuffSize: writeBuffSize,
	}
}

type webSocketConn struct {
	conn   net.Conn
	reader *bufio.Reader

	readBuffSize  int
	writeBuffSize int
	writeBuffer   []byte

	isClient bool
	isClosed bool
}

func (c *webSocketConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *webSocketConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *webSocketConn) IsClosed() bool {
	return c.isClosed
}

const (
	opcodeBitMask     byte = 0xF  // 00001111
	finBitMask        byte = 0x80 // 1 << 7 // 10000000
	maskBitMask       byte = 0x80
	rsv1BitMask       byte = 0x40 // 1 << 6
	rsv2BitMask       byte = 0x20 // 1 << 5
	rsv3BitMask       byte = 0x10 // 1 << 4
	payloadLenBitMask byte = 0x7F
)

func frameHeaderSize(payloadLen int, masked bool) int {
	size := frameMinHeaderSize
	if payloadLen > 125 && payloadLen <= 0xFFFF {
		size += 2
	} else if payloadLen > 0xFFFF {
		size += frameMaxPayloadLength
	}

	if masked {
		size += frameMaskSize
	}

	return size
}

func (c *webSocketConn) WriteMessage(opc Opcode, payload []byte) error {
	buf := make([]byte, c.writeBuffSize)

	frameSize := frameHeaderSize(len(payload), c.isClient)
	if frameSize <= c.writeBuffSize {
		if _, err := c.writeFrame(opc, buf, payload, c.isClient, true); err != nil {
			return err
		}

		_, err := c.conn.Write(buf)
		return err
	}

	firstFrame := true
	offset := 0
	for offset < len(payload) {
		frameSize = c.writeBuffSize - frameMaxHeaderSize
		remaining := len(payload) - offset
		if remaining < frameSize {
			frameSize = remaining
		}

		fin := (offset + frameSize) == len(payload)
		n, err := c.writeFrame(opc, buf, payload[offset:offset+frameSize], c.isClient, fin)
		if err != nil {
			return err
		}

		if _, err := c.conn.Write(buf[:n]); err != nil {
			return err
		}

		offset += frameSize
		if firstFrame {
			opc = OpcodeContinueFrame
			firstFrame = false
		}
	}

	return nil
}

func (c *webSocketConn) writeHeaderFrame(opc Opcode, dest, src []byte, masked, fin bool) int {
	dest[0] = byte(opc)
	if fin {
		dest[0] |= finBitMask
	}

	n := len(src)
	pos := 1
	if n <= 125 {
		dest[pos] = byte(n)
		pos++
	} else if n > 125 && n <= 0xFFFF {
		dest[pos] = 126
		pos++
		binary.BigEndian.PutUint16(dest[pos:], uint16(n))
		pos += 2
	} else {
		dest[pos] = 127
		pos++
		binary.BigEndian.PutUint64(dest[pos:], uint64(n))
		pos += 8
	}

	if masked {
		dest[1] |= finBitMask
	}

	return pos
}

func (c *webSocketConn) writeFrame(opcode Opcode, dest, src []byte, masked, fin bool) (int, error) {
	pos := c.writeHeaderFrame(opcode, dest, src, masked, fin)
	n := pos

	if !masked {
		return copy(dest[pos:], src), nil
	}

	mask := getMaskKey()
	n += copy(dest[pos:], mask)
	pos += frameMaskSize
	n += copy(dest[pos:], src)

	for i := pos; len(dest) > i; i++ {
		dest[i] ^= mask[i%4]
	}

	return n, nil
}

type Message struct {
	Opcode Opcode
	Data   []byte
	Err    error
}

// https://datatracker.ietf.org/doc/html/rfc6455#section-5.2
func (c *webSocketConn) ReadMessage() Message {
	var opc Opcode
	message := make([]byte, c.readBuffSize)

	pos := 0
	reader := c.reader
	fin, firstFrame := false, true

	for !fin {
		header, err := readN(reader, 2)
		if err != nil {
			return Message{
				Err: wrapError(err),
			}
		}

		fin = header[0]&finBitMask != 0
		opcode := Opcode(header[0] & opcodeBitMask)
		if firstFrame {
			opc = opcode
			firstFrame = false
		} else if !opcode.IsContinue() && !opcode.IsClose() {
			return Message{
				Err: fmt.Errorf("websocket: unexpected opcode [%d <-> %s] in fragmented message", opcode, opcode),
			}
		} else if opcode.IsClose() {
			c.isClosed = true
		}

		payloadLen := int(header[1] & payloadLenBitMask)

		if payloadLen == 126 {
			extPayloadLen, _err := readN(reader, 2)
			if _err != nil {
				return Message{Err: wrapError(_err)}
			}
			payloadLen = int(binary.BigEndian.Uint16(extPayloadLen))
		} else if payloadLen == 127 {
			extPayloadLen, _err := readN(reader, 8)
			if _err != nil {
				return Message{Err: wrapError(_err)}
			}

			n := binary.BigEndian.Uint64(extPayloadLen)
			if n > uint64(maxInt) {
				return Message{Err: fmt.Errorf("websocket: payload too large (%d bytes)", n)}
			}

			payloadLen = int(n)
		} else if payloadLen <= 125 {
			return Message{Err: fmt.Errorf("websocket: unexpected payloadLen %d, allowed=[125,126,127]", payloadLen)}
		}

		if pos+payloadLen > maxInt {
			return Message{Err: fmt.Errorf("websocket: payload too large (%d bytes)", pos+payloadLen)}
		}

		var mask []byte
		masked := header[1]&maskBitMask != 0
		if masked {
			mask, err = readN(reader, 4)
		}

		data, err := readN(reader, payloadLen)
		if err != nil {
			return Message{Err: wrapError(err)}
		}

		if masked {
			for i := range data {
				data[i] ^= mask[i%4]
			}
		}

		pos += copy(message[pos:], data)
	}

	return Message{Opcode: opc, Data: message}
}

func (c *webSocketConn) MessageIter() <-chan Message {
	ch := make(chan Message)

	go func() {
		defer close(ch)

		shouldClose := false
		for !shouldClose {
			m := c.ReadMessage()
			ch <- m

			shouldClose = m.Err != nil
		}
	}()
	return ch
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

func getMaskKey() []byte {
	return []byte{0x12, 0x34, 0x56, 0x78}
}
