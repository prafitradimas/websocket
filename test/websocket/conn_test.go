package websocket_test

import (
	"io"
	"net"
	"time"

	"github.com/prafitradimas/websocket/pkg/websocket"
)

type fakeConn struct {
	io.Reader
	io.Writer
}

func (c fakeConn) Close() error                       { return nil }
func (c fakeConn) LocalAddr() net.Addr                { return localAddr }
func (c fakeConn) RemoteAddr() net.Addr               { return remoteAddr }
func (c fakeConn) SetDeadline(t time.Time) error      { return nil }
func (c fakeConn) SetReadDeadline(t time.Time) error  { return nil }
func (c fakeConn) SetWriteDeadline(t time.Time) error { return nil }

type fakeAddr int

var (
	localAddr  = fakeAddr(1)
	remoteAddr = fakeAddr(2)
)

func (a fakeAddr) Network() string {
	return "net"
}

func (a fakeAddr) String() string {
	return "str"
}

func newTestConn(r io.Reader, w io.Writer, isClient bool) websocket.WebSocket {
	return websocket.NewConn(fakeConn{Reader: r, Writer: w}, 1024, 1024, nil, isClient)
}
