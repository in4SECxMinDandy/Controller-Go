// Agent headless — kết nối WSS đến Controller, gửi màn hình + nhận lệnh.
//
// Build (Windows, không console):
//
//	go build -ldflags="-H windowsgui -s -w" -trimpath -o agent.exe
package main

import (
	"context"
	"log"
	"math/rand"
	"os/signal"
	"syscall"
	"time"

	"github.com/iot-lab/agent/persistence"
)

const Version = "0.1.0-mvp1"

func main() {
	cfg := loadConfig()
	log.Printf("agent %s id=%s controller=%s", Version, cfg.AgentID, cfg.ControllerURL)

	// Đăng ký autostart cùng Windows ngay lần đầu chạy (idempotent, không cần admin).
	if changed, err := persistence.EnsureAutostart(); err != nil {
		log.Printf("autostart: %v", err)
	} else if changed {
		log.Printf("autostart: registered")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Vòng lặp reconnect với exponential backoff.
	backoff := time.Second
	const maxBackoff = 60 * time.Second

	for ctx.Err() == nil {
		err := runSession(ctx, cfg)
		if err != nil {
			log.Printf("session ended: %v", err)
		}
		if ctx.Err() != nil {
			return
		}
		jitter := time.Duration(rand.Int63n(int64(backoff / 2)))
		sleep := backoff + jitter
		log.Printf("reconnect in %s", sleep)
		select {
		case <-time.After(sleep):
		case <-ctx.Done():
			return
		}
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}
