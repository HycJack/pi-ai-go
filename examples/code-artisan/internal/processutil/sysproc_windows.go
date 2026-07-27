//go:build windows

package processutil

import "syscall"

const createNoWindow = 0x08000000

func WithoutConsoleWindow() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: createNoWindow}
}
