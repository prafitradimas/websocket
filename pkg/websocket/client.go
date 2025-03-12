package websocket

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
)

type WebSocketClient struct {
	ReadBufferSize  int
	WriteBufferSize int

	Subprotocols []string
}

func clientKey() string {
	return "dGhlIHNhbXBsZSBub25jZQ=="
}

func defaultClientHeader() http.Header {
	return http.Header{
		"Upgrade":               []string{"websocket"},
		"Connection":            []string{"Upgrade"},
		"Sec-WebSocket-Key":     []string{clientKey()},
		"Sec-WebSocket-Version": []string{"13"},
	}
}

func (client *WebSocketClient) DialWithContext(ctx context.Context, url *url.URL, extraHeader http.Header) (WebSocket, error) {
	request := http.Request{
		Method:     http.MethodGet,
		URL:        url,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     defaultClientHeader(),
	}

	switch url.Scheme {
	case "ws":
		url.Scheme = "http"
	case "wss":
		url.Scheme = "https"
	default:
		return nil, fmt.Errorf("websocket: invalid url scheme, expect 'ws' or 'wss' instead of '%s'", url.Scheme)
	}

	req := request.WithContext(ctx)
	if len(client.Subprotocols) > 0 {
		req.Header["Sec-WebSocket-Protocol"] = client.Subprotocols
	}

	tracer := httptrace.ContextClientTrace(ctx)
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", url.Host)
	if err != nil {
		return nil, wrapError(err)
	}

	if tracer != nil && tracer.GotConn != nil {
		tracer.GotConn(httptrace.GotConnInfo{Conn: conn})
	}

	shouldCloseConn := true
	defer func() {
		if shouldCloseConn {
			conn.Close()
		}
	}()

	if err = req.Write(conn); err != nil {
		return nil, wrapError(err)
	}

	ws := NewConn(conn, client.ReadBufferSize, client.WriteBufferSize, nil, true).(*webSocketConn)
	if err = req.Write(conn); err != nil {
		return nil, wrapError(err)
	}

	if tracer != nil && tracer.GotFirstResponseByte != nil {
		if _, err = ws.reader.Peek(1); err != nil {
			tracer.GotFirstResponseByte()
		}
	}

	res, err := http.ReadResponse(ws.reader, req)
	if err != nil {
		return nil, wrapError(err)
	}

	if res.StatusCode != http.StatusSwitchingProtocols {
		return nil, ErrBadHandshake
	}

	// stops deferred function from closing the connection
	shouldCloseConn = false
	return ws, nil
}
