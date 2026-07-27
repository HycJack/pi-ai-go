//go:build !windows

package processutil

import "syscall"

func WithoutConsoleWindow() *syscall.SysProcAttr {
	return nil
}
