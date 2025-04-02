package websocket

import (
	"crypto/sha1"
	"encoding/base64"
	"net/http"
	"slices"
	"strings"
)

type WebSocketServer struct {
	Subprotocols []string
}

func (this *WebSocketServer) Upgrade(res http.ResponseWriter, req *http.Request) (WebSocket, error) {
	if req.Method != "GET" {
		res.WriteHeader(http.StatusMethodNotAllowed)
		return nil, ErrMethodNotAllowed
	}

	// The handshake from the client looks as follows:
	//      GET /chat HTTP/1.1
	//      Host: server.example.com
	//      Upgrade: websocket
	//      Connection: Upgrade
	//      Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==
	//      Origin: http://example.com
	//      Sec-WebSocket-Protocol: chat, superchat
	//      Sec-WebSocket-Version: 13
	if req.Header.Get("Connection") != "Upgrade" || req.Header.Get("Upgrade") != "websocket" {
		res.WriteHeader(http.StatusUpgradeRequired)
		return nil, ErrBadUpgrade
	}

	if req.Header.Get("Sec-WebSocket-Version") != "13" || req.Header.Get("Sec-WebSocket-Key") == "" {
		res.WriteHeader(http.StatusBadRequest)
		return nil, ErrBadHandshake
	}

	clientKey := req.Header.Get("Sec-WebSocket-Key")

	// The handshake from the server looks as follows:
	//      HTTP/1.1 101 Switching Protocols
	//      Upgrade: websocket
	//      Connection: Upgrade
	//      Sec-WebSocket-Accept: s3pPLMBiTxaQ9kYGzzhZRbK+xOo=
	//      Sec-WebSocket-Protocol: chat
	res.WriteHeader(http.StatusSwitchingProtocols)
	res.Header().Set("Upgrade", "websocket")
	res.Header().Set("Connection", "Upgrade")
	res.Header().Set("Sec-WebSocket-Accept", serverKey(clientKey))

	if subprotocols, ok := req.Header["Sec-Websocket-Protocol"]; ok {
		for i := range this.Subprotocols {
			subprotocol := strings.TrimSpace(this.Subprotocols[i])
			if slices.Contains(subprotocols, subprotocol) {
				res.Header().Add("Sec-WebSocket-Protocol", subprotocol)
			}
		}
	}

	conn, readwriter, err := http.NewResponseController(res).Hijack()
	if err != nil {
		_ = conn.Close()
		return nil, wrapError(err)
	}

	return NewConn(conn, readwriter.Reader, readwriter.Writer, false), nil
}

func serverKey(clientKey string) string {
	hash := sha1.New()
	hash.Write([]byte(clientKey))
	hash.Write([]byte("258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(hash.Sum(nil))
}
