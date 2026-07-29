//go:build !windows

package tools

import "syscall"

func newSysProcAttr() *syscall.SysProcAttr {
	return nil
}
