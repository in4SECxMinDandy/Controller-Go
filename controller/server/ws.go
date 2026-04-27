package server

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/iot-lab/shared"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true }, // lab
}

// HandleWS xử lý kết nối WSS từ agent.
func HandleWS(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		conn.SetReadLimit(10 << 20) // 10MB cho frame lớn

		// Đọc Hello đầu tiên (timeout 10s).
		conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		mt, data, err := conn.ReadMessage()
		if err != nil || mt != websocket.TextMessage {
			log.Printf("hello recv err: %v", err)
			return
		}
		var env shared.Envelope
		if err := json.Unmarshal(data, &env); err != nil || env.Type != shared.TypeHello {
			log.Printf("not hello: %v", err)
			return
		}
		// Re-decode payload.
		raw, _ := json.Marshal(env.Payload)
		var hello shared.Hello
		_ = json.Unmarshal(raw, &hello)

		// TODO: kiểm tra hello.Token.
		log.Printf("agent connected: id=%s host=%s os=%s", hello.AgentID, hello.Hostname, hello.OS)

		ag := &Agent{
			ID:       hello.AgentID,
			Hostname: hello.Hostname,
			OS:       hello.OS,
			Version:  hello.Version,
			Since:    time.Now(),
			Send:     make(chan []byte, 32),
			frameCh:  make(chan struct{}, 1),
		}
		hub.Register(ag)
		defer hub.Unregister(ag)

		// Welcome
		welcome := shared.Envelope{
			Type: shared.TypeWelcome,
			Payload: shared.Welcome{
				SessionID:   hello.AgentID + "-" + time.Now().Format("150405"),
				FPSTarget:   30,
				JPEGQuality: 40,
				Scale:       0.66, // downscale ~66% mỗi chiều ⇒ ~44% pixel ⇒ encode nhanh hơn ~2x
			},
		}
		wb, _ := json.Marshal(welcome)
		if err := conn.WriteMessage(websocket.TextMessage, wb); err != nil {
			return
		}

		// Writer goroutine
		done := make(chan struct{})
		go func() {
			defer close(done)
			for msg := range ag.Send {
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					return
				}
			}
		}()

		// Read loop
		conn.SetReadDeadline(time.Time{})
		var frameCount int
		var lastFrameLog = time.Now()
		for {
			conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			mt, data, err := conn.ReadMessage()
			if err != nil {
				log.Printf("agent %s disconnected: %v (frames received: %d)", ag.ID, err, frameCount)
				return
			}
			switch mt {
			case websocket.TextMessage:
				// heartbeat hoặc message khác.
				_ = data
			case websocket.BinaryMessage:
				seq, jpegBytes, ok := shared.UnpackFrame(data)
				if !ok {
					log.Printf("agent %s: bad frame header", ag.ID)
					continue
				}
				cp := make([]byte, len(jpegBytes))
				copy(cp, jpegBytes)
				ag.SetFrame(seq, cp)
				frameCount++
				if time.Since(lastFrameLog) > 5*time.Second {
					log.Printf("agent %s: rx frame seq=%d size=%dKB total=%d", ag.ID, seq, len(jpegBytes)/1024, frameCount)
					lastFrameLog = time.Now()
				}
			}
		}
	}
}

// HandleListAgents trả JSON danh sách agent online — debug endpoint.
func HandleListAgents(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(hub.List())
	}
}
