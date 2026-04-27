//go:build windows

// Package input: gọi Windows SendInput để inject chuột/bàn phím.
//
// Toạ độ chuột dùng MOUSEEVENTF_ABSOLUTE | MOUSEEVENTF_VIRTUALDESK với giá trị
// 0..65535 cho toàn bộ virtual desktop. Caller truyền normalized 0..1 và package
// này nhân lên 65535.
package input

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32          = windows.NewLazySystemDLL("user32.dll")
	procSendInput   = user32.NewProc("SendInput")
	procMapVirtKey  = user32.NewProc("MapVirtualKeyW")
)

const (
	inputMouseT    = 0
	inputKeyboardT = 1

	mouseEventfMove        = 0x0001
	mouseEventfLeftDown    = 0x0002
	mouseEventfLeftUp      = 0x0004
	mouseEventfRightDown   = 0x0008
	mouseEventfRightUp     = 0x0010
	mouseEventfMiddleDown  = 0x0020
	mouseEventfMiddleUp    = 0x0040
	mouseEventfWheel       = 0x0800
	mouseEventfHWheel      = 0x01000
	mouseEventfAbsolute    = 0x8000
	mouseEventfVirtualDesk = 0x4000

	keyEventfKeyUp    = 0x0002
	keyEventfScancode = 0x0008
)

// INPUT trên amd64 = 40 bytes: 4 (type) + 4 (pad) + 32 (union).
type mouseInput struct {
	Dx          int32
	Dy          int32
	MouseData   uint32
	DwFlags     uint32
	Time        uint32
	DwExtraInfo uintptr
}

type keybdInput struct {
	WVk         uint16
	WScan       uint16
	DwFlags     uint32
	Time        uint32
	DwExtraInfo uintptr
	_           [8]byte // pad cho khớp size mouseInput (32 bytes)
}

type inputUnionMouse struct {
	Type uint32
	_    uint32
	Mi   mouseInput
}

type inputUnionKbd struct {
	Type uint32
	_    uint32
	Ki   keybdInput
}

// MouseAction encode 1 sự kiện chuột thành SendInput call.
//   action: "move" | "down" | "up" | "wheel"
//   button: "left" | "right" | "middle" (chỉ có nghĩa với down/up)
//   normX, normY: 0..1 trên virtual desktop (chỉ có nghĩa với move)
//   wheelDelta: âm/dương; ±120 = 1 notch (chỉ có nghĩa với wheel)
func Mouse(action, button string, normX, normY float64, wheelDelta int32) error {
	in := inputUnionMouse{Type: inputMouseT}
	switch action {
	case "move":
		in.Mi.Dx = int32(clamp01(normX) * 65535)
		in.Mi.Dy = int32(clamp01(normY) * 65535)
		in.Mi.DwFlags = mouseEventfMove | mouseEventfAbsolute | mouseEventfVirtualDesk
	case "down":
		in.Mi.DwFlags = buttonFlag(button, true)
	case "up":
		in.Mi.DwFlags = buttonFlag(button, false)
	case "wheel":
		in.Mi.DwFlags = mouseEventfWheel
		in.Mi.MouseData = uint32(wheelDelta)
	default:
		return nil
	}
	return sendOne(unsafe.Pointer(&in))
}

// Key encode 1 sự kiện bàn phím.
//   vk: Windows Virtual-Key Code.
//   action: "down" | "up"
func Key(vk uint16, action string) error {
	scan, _, _ := procMapVirtKey.Call(uintptr(vk), 0 /* MAPVK_VK_TO_VSC */)
	in := inputUnionKbd{Type: inputKeyboardT}
	in.Ki.WVk = vk
	in.Ki.WScan = uint16(scan)
	if action == "up" {
		in.Ki.DwFlags = keyEventfKeyUp
	}
	return sendOne(unsafe.Pointer(&in))
}

func sendOne(p unsafe.Pointer) error {
	const sizeofInput = 40 // amd64
	r, _, e := procSendInput.Call(1, uintptr(p), sizeofInput)
	if r != 1 {
		if e == nil {
			return syscall.EINVAL
		}
		return e
	}
	return nil
}

func buttonFlag(button string, down bool) uint32 {
	switch button {
	case "left":
		if down {
			return mouseEventfLeftDown
		}
		return mouseEventfLeftUp
	case "right":
		if down {
			return mouseEventfRightDown
		}
		return mouseEventfRightUp
	case "middle":
		if down {
			return mouseEventfMiddleDown
		}
		return mouseEventfMiddleUp
	}
	return 0
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
