package main

import (
	"context"
	"time"

	"github.com/iot-lab/controller/server"
)

// App là object Wails bind ra cho frontend.
type App struct {
	ctx context.Context
	hub *server.Hub
}

func NewApp(hub *server.Hub) *App {
	return &App{hub: hub}
}

// startup được Wails gọi khi window mở.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// AgentInfo: shape JSON dùng cho frontend.
type AgentInfo struct {
	ID       string    `json:"id"`
	Hostname string    `json:"hostname"`
	OS       string    `json:"os"`
	Version  string    `json:"version"`
	Since    time.Time `json:"since"`
}

// ListAgents — bind: trả danh sách agent đang online.
func (a *App) ListAgents() []AgentInfo {
	agents := a.hub.List()
	out := make([]AgentInfo, 0, len(agents))
	for _, ag := range agents {
		out = append(out, AgentInfo{
			ID:       ag.ID,
			Hostname: ag.Hostname,
			OS:       ag.OS,
			Version:  ag.Version,
			Since:    ag.Since,
		})
	}
	return out
}

// LocalServerURL — bind: trả URL local mà frontend dùng để embed stream + WS điều khiển.
func (a *App) LocalServerURL() string {
	if a.hub == nil {
		return ""
	}
	// Frontend sẽ ghép /agents/{id}/stream và /control/{id}.
	return a.hub.LocalURL()
}
