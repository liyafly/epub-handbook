//go:build windows

package pipeline

import "os"

// processAlive：Windows 上 FindProcess 会打开进程句柄，进程不存在时返回错误。
func processAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	_ = p.Release()
	return true
}
