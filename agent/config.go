package main

import (
	"os"
)

// Config được lấy từ biến môi trường (sau này có thể đổi sang file mã hoá).
type Config struct {
	ControllerURL string // ví dụ: wss://127.0.0.1:8443/ws
	Token         string // token chia sẻ trước
	AgentID       string // ID cố định, hoặc auto-gen lần đầu
}

func loadConfig() Config {
	c := Config{
		ControllerURL: getenv("CONTROLLER_URL", "wss://127.0.0.1:8443/ws"),
		Token:         getenv("AGENT_TOKEN", "dev-token"),
		AgentID:       getenv("AGENT_ID", ""),
	}
	if c.AgentID == "" {
		host, _ := os.Hostname()
		c.AgentID = host
	}
	return c
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
