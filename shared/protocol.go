// Package shared định nghĩa protocol message dùng chung giữa Agent và Controller.
//
// Tất cả message điều khiển ở dạng JSON, gói trong WebSocket text frame.
// Khung hình màn hình ở dạng binary frame để giảm overhead.
package shared

// Các loại message.
const (
	TypeHello     = "hello"
	TypeWelcome   = "welcome"
	TypeHeartbeat = "heartbeat"
	TypeMouse     = "mouse"
	TypeKey       = "key"
	TypeShutdown  = "shutdown"
	// TypeFrame không xuất hiện trong JSON — frame đi qua binary message.
)

// Envelope là cấu trúc chung cho mọi message JSON.
type Envelope struct {
	Type    string `json:"type"`
	ID      string `json:"id,omitempty"`
	Payload any    `json:"payload,omitempty"`
}

// Hello: Agent gửi sau khi kết nối.
type Hello struct {
	AgentID  string `json:"agent_id"`
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Version  string `json:"version"`
	Token    string `json:"token"`
}

// Welcome: Controller phản hồi Hello.
type Welcome struct {
	SessionID   string  `json:"session_id"`
	FPSTarget   int     `json:"fps_target"`
	JPEGQuality int     `json:"jpeg_quality"`
	Scale       float64 `json:"scale"` // 0<scale<=1; 0.5 = downscale 50% mỗi chiều
}

// Heartbeat: giữ kết nối, đo RTT.
type Heartbeat struct {
	TS int64 `json:"ts"` // unix milli
}

// MouseAction values.
const (
	MouseMove  = "move"
	MouseDown  = "down"
	MouseUp    = "up"
	MouseWheel = "wheel"
)

// Mouse: Controller → Agent. Toạ độ normalized 0..1 trên virtual desktop.
type Mouse struct {
	NormX  float64 `json:"nx"`               // 0..1
	NormY  float64 `json:"ny"`               // 0..1
	Button string  `json:"button,omitempty"` // left|right|middle
	Action string  `json:"action"`           // move|down|up|wheel
	Delta  int     `json:"delta,omitempty"`  // wheel: ±120 = 1 notch
}

// KeyAction values.
const (
	KeyDown = "down"
	KeyUp   = "up"
)

// Key: Controller → Agent. VK là Windows Virtual-Key Code.
type Key struct {
	VK     uint16 `json:"vk"`
	Action string `json:"action"` // down|up
}

// FrameHeader: 8 byte ở đầu binary frame chứa khung hình.
//
//	bytes 0..3 : magic "FRM1"
//	bytes 4..7 : little-endian uint32 — sequence number
//
// Phần còn lại: JPEG bytes.
const (
	FrameMagic     = "FRM1"
	FrameHeaderLen = 8
)

// PackFrame ghép header + payload thành 1 binary message.
func PackFrame(seq uint32, jpegBytes []byte) []byte {
	out := make([]byte, FrameHeaderLen+len(jpegBytes))
	copy(out[0:4], FrameMagic)
	out[4] = byte(seq)
	out[5] = byte(seq >> 8)
	out[6] = byte(seq >> 16)
	out[7] = byte(seq >> 24)
	copy(out[FrameHeaderLen:], jpegBytes)
	return out
}

// UnpackFrame trả seq + jpegBytes (slice vào buf gốc, không copy).
// Trả error nếu magic không khớp hoặc buf quá ngắn.
func UnpackFrame(buf []byte) (seq uint32, jpegBytes []byte, ok bool) {
	if len(buf) < FrameHeaderLen {
		return 0, nil, false
	}
	if string(buf[0:4]) != FrameMagic {
		return 0, nil, false
	}
	seq = uint32(buf[4]) | uint32(buf[5])<<8 | uint32(buf[6])<<16 | uint32(buf[7])<<24
	return seq, buf[FrameHeaderLen:], true
}
