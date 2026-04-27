//go:build windows

// Package persistence: tự đăng ký khởi chạy cùng Windows.
//
// Cơ chế: ghi đường dẫn .exe vào registry key
//   HKEY_CURRENT_USER\Software\Microsoft\Windows\CurrentVersion\Run
// Key này chạy ở quyền user hiện tại, KHÔNG yêu cầu UAC.
// Windows sẽ tự chạy giá trị này mỗi lần user đăng nhập.
package persistence

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
	valueName  = "IoTLabAgent" // tên hiển thị trong registry
)

// EnsureAutostart đảm bảo agent được đăng ký autostart.
// Idempotent: gọi nhiều lần an toàn, chỉ ghi nếu chưa có hoặc đường dẫn khác.
// Trả về (đã thay đổi gì không, error).
func EnsureAutostart() (changed bool, err error) {
	exe, err := os.Executable()
	if err != nil {
		return false, fmt.Errorf("get exe path: %w", err)
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return false, err
	}
	// Bao quote để xử lý đường dẫn có khoảng trắng.
	desired := `"` + exe + `"`

	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return false, fmt.Errorf("open run key: %w", err)
	}
	defer k.Close()

	if cur, _, err := k.GetStringValue(valueName); err == nil {
		if strings.EqualFold(cur, desired) {
			return false, nil // đã đúng rồi
		}
	}
	if err := k.SetStringValue(valueName, desired); err != nil {
		return false, fmt.Errorf("set value: %w", err)
	}
	return true, nil
}

// RemoveAutostart gỡ đăng ký (dùng cho lệnh shutdown/uninstall sau này).
func RemoveAutostart() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if err := k.DeleteValue(valueName); err != nil && err != registry.ErrNotExist {
		return err
	}
	return nil
}
