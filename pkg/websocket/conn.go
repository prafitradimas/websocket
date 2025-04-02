package websocket

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

type WebSocket interface {
	WriteMessage(opc Opcode, data []byte) error

	WriteCloseMessage(status CloseStatus, payload []byte) error

	ReadMessage() Message

	LocalAddr() net.Addr

	RemoteAddr() net.Addr

	IsClosed() bool

	Close() error
}

func NewConn(connection net.Conn, reader *bufio.Reader, writer *bufio.Writer, isClient bool) WebSocket {
	return &webSocketConn{
		conn:   connection,
		reader: reader,
		writer: writer,
	}
}

type webSocketConn struct {
	conn net.Conn

	reader *bufio.Reader
	writer *bufio.Writer

	readMu  sync.Mutex
	writeMu sync.Mutex

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

func (c *webSocketConn) Close() error {
	c.isClosed = true
	return c.conn.Close()
}

func (c *webSocketConn) WriteMessage(opc Opcode, payload []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	c.conn.SetWriteDeadline(time.Now().Add(time.Second * 2))

	buf := make([]byte, 512)
	writer := NewFrameWriter(opc, c.conn, buf, c.isClient)
	_, err := writer.Write(payload)

	return err
}

func (c *webSocketConn) WriteCloseMessage(status CloseStatus, payload []byte) error {
	if len(payload) > 125 {
		return errors.New("too large")
	}

	buf := make([]byte, 0, 125)
	binary.BigEndian.AppendUint16(buf, uint16(status))
	buf = append(buf, payload...)

	err := c.WriteMessage(OpcodeCloseFrame, buf)
	return err
}

type Message struct {
	Opcode Opcode
	Data   []byte
	Err    error
}

func (m *Message) String() string {
	return string(m.Data)
}

// https://datatracker.ietf.org/doc/html/rfc6455#section-5.2
func (c *webSocketConn) ReadMessage() Message {
	if c.IsClosed() {
		return Message{
			Err: io.EOF,
		}
	}

	c.readMu.Lock()
	defer c.readMu.Unlock()

	c.conn.SetReadDeadline(time.Now().Add(time.Second * 2))

	reader := NewFrameReader(c.conn)

	msg := Message{}
	msg.Opcode = reader.Opcode()

	if msg.Err == nil {
		msg.Data, msg.Err = io.ReadAll(reader)
	}

	return msg
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
