//go:build !windows

package input

func Mouse(action, button string, normX, normY float64, wheelDelta int32) error { return nil }
func Key(vk uint16, action string) error                                          { return nil }
