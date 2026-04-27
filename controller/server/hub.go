package server

import (
	"sync"
	"time"
)

// Agent đại diện 1 kết nối agent đang online.
type Agent struct {
	ID       string      `json:"id"`
	Hostname string      `json:"hostname"`
	OS       string      `json:"os"`
	Version  string      `json:"version"`
	Since    time.Time   `json:"since"`
	Send     chan []byte `json:"-"` // outbound queue

	frameMu   sync.RWMutex
	lastFrame []byte
	lastSeq   uint32
	frameCh   chan struct{} // signal cho subscriber MJPEG
}

// SetFrame cập nhật frame mới nhất + đánh thức subscriber.
func (a *Agent) SetFrame(seq uint32, jpeg []byte) {
	a.frameMu.Lock()
	a.lastFrame = jpeg
	a.lastSeq = seq
	a.frameMu.Unlock()
	// Non-blocking signal — nếu chưa có ai consume thì bỏ qua.
	select {
	case a.frameCh <- struct{}{}:
	default:
	}
}

// LastFrame trả về frame mới nhất (snapshot, an toàn cho reader).
func (a *Agent) LastFrame() (uint32, []byte) {
	a.frameMu.RLock()
	defer a.frameMu.RUnlock()
	return a.lastSeq, a.lastFrame
}

// FrameSignal trả channel để subscriber chờ frame mới.
func (a *Agent) FrameSignal() <-chan struct{} { return a.frameCh }

// Hub quản lý danh sách Agent đang kết nối.
type Hub struct {
	mu     sync.RWMutex
	agents map[string]*Agent

	register   chan *Agent
	unregister chan *Agent

	localURL string // được main set sau khi biết addr + scheme
}

// SetLocalURL gọi từ main để frontend Wails biết base URL.
func (h *Hub) SetLocalURL(u string) { h.localURL = u }
func (h *Hub) LocalURL() string     { return h.localURL }

func NewHub() *Hub {
	return &Hub{
		agents:     make(map[string]*Agent),
		register:   make(chan *Agent, 16),
		unregister: make(chan *Agent, 16),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case a := <-h.register:
			h.mu.Lock()
			h.agents[a.ID] = a
			h.mu.Unlock()
		case a := <-h.unregister:
			h.mu.Lock()
			if cur, ok := h.agents[a.ID]; ok && cur == a {
				delete(h.agents, a.ID)
				close(a.Send)
			}
			h.mu.Unlock()
		}
	}
}

func (h *Hub) Register(a *Agent)   { h.register <- a }
func (h *Hub) Unregister(a *Agent) { h.unregister <- a }

// Get trả Agent theo ID (nil nếu không có).
func (h *Hub) Get(id string) *Agent {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.agents[id]
}

func (h *Hub) List() []*Agent {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]*Agent, 0, len(h.agents))
	for _, a := range h.agents {
		out = append(out, a)
	}
	return out
}
