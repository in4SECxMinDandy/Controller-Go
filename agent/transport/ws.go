// Package transport: WebSocket client cho Agent.
package transport

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Client bọc gorilla websocket, thêm helper gửi/nhận envelope.
// Tất cả phương thức Send* thread-safe (serialize qua writeMu).
type Client struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
}

// Dial thiết lập kết nối WSS, tự đọc HTTPS_PROXY từ env.
// insecure=true để chấp nhận self-signed cert (chỉ cho lab).
func Dial(ctx context.Context, rawURL string, insecure bool) (*Client, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	dialer := *websocket.DefaultDialer
	dialer.HandshakeTimeout = 10 * time.Second
	dialer.Proxy = http.ProxyFromEnvironment // tự dùng HTTPS_PROXY/HTTP_PROXY
	dialer.TLSClientConfig = &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: insecure,
		// TODO: pin SHA256 fingerprint của cert controller ở đây.
	}

	conn, _, err := dialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	return &Client{conn: conn}, nil
}

// SendJSON gửi 1 envelope JSON.
func (c *Client) SendJSON(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.conn.WriteMessage(websocket.TextMessage, b)
}

// SendBinary gửi 1 binary message (dùng cho frame).
func (c *Client) SendBinary(b []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.conn.WriteMessage(websocket.BinaryMessage, b)
}

// Recv đọc 1 message. messageType là websocket.TextMessage hoặc BinaryMessage.
func (c *Client) Recv() (messageType int, data []byte, err error) {
	return c.conn.ReadMessage()
}

func (c *Client) Close() error { return c.conn.Close() }

// SetReadDeadline tiện cho heartbeat timeout.
func (c *Client) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}
