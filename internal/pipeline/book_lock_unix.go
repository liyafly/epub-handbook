//go:build !windows

package pipeline

import (
	"errors"
	"os"
	"syscall"
)

// processAlive 用 signal 0 探测进程：EPERM 表示存在但属于别的用户，也算活着。
func processAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = p.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}
