package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/iot-lab/agent/capture"
	"github.com/iot-lab/agent/input"
	"github.com/iot-lab/agent/inputlock"
	"github.com/iot-lab/agent/transport"
	"github.com/iot-lab/shared"
)

// sessionState giữ tham số từ welcome để capture loop đọc được.
type sessionState struct {
	fpsTarget   atomic.Int32
	jpegQuality atomic.Int32
	scaleX1000  atomic.Int32 // lưu scale * 1000 (atomic không hỗ trợ float)
}

func (s *sessionState) Scale() float64 { return float64(s.scaleX1000.Load()) / 1000.0 }

// runSession: 1 phiên kết nối tới Controller. Trả về khi mất kết nối.
func runSession(ctx context.Context, cfg Config) error {
	dialCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cli, err := transport.Dial(dialCtx, cfg.ControllerURL, true /* insecure for lab */)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer cli.Close()
	log.Printf("connected to %s", cfg.ControllerURL)

	// 1) Hello
	hello := shared.Envelope{
		Type: shared.TypeHello,
		Payload: shared.Hello{
			AgentID:  cfg.AgentID,
			Hostname: cfg.AgentID,
			OS:       runtime.GOOS,
			Version:  Version,
			Token:    cfg.Token,
		},
	}
	if err := cli.SendJSON(hello); err != nil {
		return fmt.Errorf("send hello: %w", err)
	}

	// State chia sẻ giữa read loop và capture loop.
	state := &sessionState{}
	state.fpsTarget.Store(12)
	state.jpegQuality.Store(60)
	state.scaleX1000.Store(1000) // 1.0

	// Khoá input local trong suốt phiên (trừ khi INPUT_LOCK=0).
	var locker *inputlock.Locker
	if os.Getenv("INPUT_LOCK") != "0" {
		locker = inputlock.Start()
		log.Printf("input lock: armed (set INPUT_LOCK=0 to disable)")
		defer func() {
			locker.Stop()
			log.Printf("input lock: released")
		}()
	}

	// 2) Heartbeat + capture goroutines
	subCtx, subCancel := context.WithCancel(ctx)
	defer subCancel()
	go heartbeatLoop(subCtx, cli)
	go captureLoop(subCtx, cli, state)

	// 3) Read loop
	for {
		mt, data, err := cli.Recv()
		if err != nil {
			return fmt.Errorf("recv: %w", err)
		}
		switch mt {
		case websocket.TextMessage:
			if err := handleControl(data, state); err != nil {
				log.Printf("handle control: %v", err)
			}
		case websocket.BinaryMessage:
			// Hiện tại Controller không gửi binary xuống. Bỏ qua.
		}
	}
}

// captureLoop: theo FPS mục tiêu, chụp màn hình + gửi binary frame.
func captureLoop(ctx context.Context, cli *transport.Client, state *sessionState) {
	log.Printf("capture loop start")
	var seq uint32
	var lastLog time.Time
	for {
		if ctx.Err() != nil {
			log.Printf("capture loop stop: ctx done")
			return
		}
		fps := int(state.fpsTarget.Load())
		if fps < 1 {
			fps = 1
		}
		interval := time.Second / time.Duration(fps)
		start := time.Now()

		jpegBytes, err := capture.CaptureJPEG(capture.CaptureOpts{
			Quality: int(state.jpegQuality.Load()),
			Scale:   state.Scale(),
		})
		if err != nil {
			log.Printf("capture: %v", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
			continue
		}
		seq++
		if err := cli.SendBinary(shared.PackFrame(seq, jpegBytes)); err != nil {
			log.Printf("send frame: %v", err)
			return
		}

		// Log mỗi 5s để theo dõi.
		if time.Since(lastLog) > 5*time.Second {
			log.Printf("frame seq=%d size=%dKB took=%dms", seq, len(jpegBytes)/1024, time.Since(start).Milliseconds())
			lastLog = time.Now()
		}

		sleep := interval - time.Since(start)
		if sleep > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(sleep):
			}
		}
	}
}

func heartbeatLoop(ctx context.Context, cli *transport.Client) {
	t := time.NewTicker(20 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			env := shared.Envelope{
				Type:    shared.TypeHeartbeat,
				Payload: shared.Heartbeat{TS: time.Now().UnixMilli()},
			}
			if err := cli.SendJSON(env); err != nil {
				log.Printf("heartbeat: %v", err)
				return
			}
		}
	}
}

func handleControl(data []byte, state *sessionState) error {
	var env shared.Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return err
	}
	switch env.Type {
	case shared.TypeWelcome:
		// Re-decode payload để lấy FPS/quality.
		raw, _ := json.Marshal(env.Payload)
		var w shared.Welcome
		_ = json.Unmarshal(raw, &w)
		if w.FPSTarget > 0 {
			state.fpsTarget.Store(int32(w.FPSTarget))
		}
		if w.JPEGQuality > 0 {
			state.jpegQuality.Store(int32(w.JPEGQuality))
		}
		if w.Scale > 0 && w.Scale <= 1.0 {
			state.scaleX1000.Store(int32(w.Scale * 1000))
		}
		log.Printf("welcome received fps=%d quality=%d scale=%.2f", w.FPSTarget, w.JPEGQuality, w.Scale)
	case shared.TypeHeartbeat:
		// optional: đo RTT
	case shared.TypeMouse:
		raw, _ := json.Marshal(env.Payload)
		var m shared.Mouse
		if err := json.Unmarshal(raw, &m); err != nil {
			return err
		}
		if err := input.Mouse(m.Action, m.Button, m.NormX, m.NormY, int32(m.Delta)); err != nil {
			log.Printf("inject mouse: %v", err)
		}
	case shared.TypeKey:
		raw, _ := json.Marshal(env.Payload)
		var k shared.Key
		if err := json.Unmarshal(raw, &k); err != nil {
			return err
		}
		if err := input.Key(k.VK, k.Action); err != nil {
			log.Printf("inject key: %v", err)
		}
	case shared.TypeShutdown:
		log.Printf("shutdown requested")
		return fmt.Errorf("shutdown by controller")
	default:
		log.Printf("unknown message type: %s", env.Type)
	}
	return nil
}
