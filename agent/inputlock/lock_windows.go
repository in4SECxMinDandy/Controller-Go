//go:build windows

// Package inputlock chặn input local (chuột + bàn phím) bằng low-level hook.
//
// Khi Start() chạy:
//   - Cài WH_KEYBOARD_LL + WH_MOUSE_LL trên 1 OS thread riêng (có message loop).
//   - Mọi event KHÔNG có flag injected sẽ bị nuốt → user thật không thao tác được.
//   - Event do SendInput từ chính tiến trình tạo ra (có flag injected) vẫn đi qua bình thường.
//
// An toàn: Ctrl+Alt+Del là Secure Attention Sequence — Windows bypass mọi user-mode hook.
// Không cần quyền admin.
package inputlock

import (
	"runtime"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                  = windows.NewLazySystemDLL("user32.dll")
	kernel32                = windows.NewLazySystemDLL("kernel32.dll")
	procSetWindowsHookExW   = user32.NewProc("SetWindowsHookExW")
	procUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	procCallNextHookEx      = user32.NewProc("CallNextHookEx")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procPostThreadMessageW  = user32.NewProc("PostThreadMessageW")
	procGetCurrentThreadId  = kernel32.NewProc("GetCurrentThreadId")
)

const (
	wh_KEYBOARD_LL = 13
	wh_MOUSE_LL    = 14
	wm_QUIT        = 0x0012
	hc_ACTION      = 0
	llkhf_INJECTED = 0x10 // KBDLLHOOKSTRUCT.flags
	llmhf_INJECTED = 0x01 // MSLLHOOKSTRUCT.flags
)

type kbdLLHookStruct struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

type point struct{ X, Y int32 }

type msLLHookStruct struct {
	Pt          point
	MouseData   uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

// armed bật/tắt việc chặn (cho phép tạm dừng mà không gỡ hook).
var armed atomic.Bool

// Locker đại diện 1 lần Start. Gọi Stop() để gỡ.
type Locker struct {
	threadID uint32
	done     chan struct{}
}

// Start cài hook trên goroutine riêng. Block tới khi hook ready.
// Trả về Locker để Stop() sau này.
func Start() *Locker {
	l := &Locker{done: make(chan struct{})}
	ready := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		tid, _, _ := procGetCurrentThreadId.Call()
		l.threadID = uint32(tid)

		kbCB := syscall.NewCallback(lowLevelKeyboardProc)
		mouseCB := syscall.NewCallback(lowLevelMouseProc)

		kbHook, _, _ := procSetWindowsHookExW.Call(uintptr(wh_KEYBOARD_LL), kbCB, 0, 0)
		mouseHook, _, _ := procSetWindowsHookExW.Call(uintptr(wh_MOUSE_LL), mouseCB, 0, 0)

		armed.Store(true)
		close(ready)

		// Message loop: hook callback chỉ chạy khi thread bơm message.
		var msg [7]uintptr // sizeof(MSG) ≈ 6-7 uintptr trên amd64
		for {
			r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if r == 0 || int32(r) == -1 {
				break // WM_QUIT hoặc lỗi
			}
		}

		armed.Store(false)
		procUnhookWindowsHookEx.Call(kbHook)
		procUnhookWindowsHookEx.Call(mouseHook)
		close(l.done)
	}()
	<-ready
	return l
}

// Stop gỡ hook và chờ goroutine kết thúc.
func (l *Locker) Stop() {
	if l == nil {
		return
	}
	armed.Store(false)
	procPostThreadMessageW.Call(uintptr(l.threadID), uintptr(wm_QUIT), 0, 0)
	<-l.done
}

func lowLevelKeyboardProc(nCode int32, wParam, lParam uintptr) uintptr {
	if nCode == hc_ACTION && armed.Load() {
		k := (*kbdLLHookStruct)(unsafe.Pointer(lParam))
		if k.Flags&llkhf_INJECTED == 0 {
			return 1 // block input thật; cho phép injected (SendInput) đi qua
		}
	}
	r, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
	return r
}

func lowLevelMouseProc(nCode int32, wParam, lParam uintptr) uintptr {
	if nCode == hc_ACTION && armed.Load() {
		m := (*msLLHookStruct)(unsafe.Pointer(lParam))
		if m.Flags&llmhf_INJECTED == 0 {
			return 1
		}
	}
	r, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
	return r
}
