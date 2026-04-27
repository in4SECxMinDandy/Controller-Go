package server

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/iot-lab/shared"
)

// HandleControlWS: WS từ trang viewer trong browser → forward event xuống agent.
//
// URL: /control/{agent_id}
// Browser gửi JSON Envelope (TypeMouse/TypeKey) → controller marshal lại và
// nhét vào Send channel của agent đó.
func HandleControlWS(hub *Hub) http.HandlerFunc {
	up := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     func(*http.Request) bool { return true },
	}
	return func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/control/")
		if id == "" || strings.Contains(id, "/") {
			http.Error(w, "bad agent id", http.StatusBadRequest)
			return
		}
		ag := hub.Get(id)
		if ag == nil {
			http.Error(w, "agent not found", http.StatusNotFound)
			return
		}

		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("control upgrade: %v", err)
			return
		}
		defer conn.Close()
		log.Printf("control session opened for agent %s", id)
		defer log.Printf("control session closed for agent %s", id)

		for {
			mt, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if mt != websocket.TextMessage {
				continue
			}
			// Validate envelope qua shared.
			var env shared.Envelope
			if err := json.Unmarshal(data, &env); err != nil {
				log.Printf("control: bad json: %v", err)
				continue
			}
			if env.Type != shared.TypeMouse && env.Type != shared.TypeKey {
				continue
			}
			// Forward sang agent.
			select {
			case ag.Send <- data:
			default:
				// queue đầy ⇒ drop, không kẹt browser.
			}
		}
	}
}
