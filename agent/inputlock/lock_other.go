//go:build !windows

package inputlock

type Locker struct{}

func Start() *Locker      { return &Locker{} }
func (l *Locker) Stop()   {}
