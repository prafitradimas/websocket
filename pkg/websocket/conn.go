package websocket

import (
	"bufio"
	"net"
	"sync"
)

type WebSocket interface {
	WriteMessage(opc Opcode, data []byte) error

	ReadMessage() Message

	LocalAddr() net.Addr

	RemoteAddr() net.Addr

	IsClosed() bool
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

func (c *webSocketConn) WriteMessage(opc Opcode, payload []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	writer := NewFrameWriter(opc, c.writer, c.isClient)
	_, err := writer.Write(payload)

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
	c.readMu.Lock()
	defer c.readMu.Unlock()

	msg := Message{}

	reader, err := NewFrameReader(c.reader)
	if err != nil {
		msg.Err = err
		return msg
	}

	msg.Data = make([]byte, 0, reader.Len())
	reader.Read(msg.Data)
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
