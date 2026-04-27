//go:build !windows

package persistence

// Stub cho các OS khác — agent chỉ chạy thật trên Windows.
func EnsureAutostart() (bool, error) { return false, nil }
func RemoveAutostart() error         { return nil }
