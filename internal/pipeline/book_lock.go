package pipeline

// 书级构建锁：<状态目录>/build.lock 目录 + owner.json（PID、主机名）。
//
// 进程被强行结束（kill -9、断电、旧版本被 SIGPIPE 杀掉）时 defer 来不及删锁。
// 下次运行时，若锁的主人是本机上已不存在的进程，就当作残留锁清掉再继续。
// 没有 owner.json 的锁（旧 sh 构建器留下的，或另一个进程刚建目录还没写完）、
// 别的主机建的锁、主人仍在运行的锁，一律照旧阻断，交给人判断。

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/liyafly/epub-handbook/internal/zipfs"
)

const (
	buildLockName = "build.lock"
	lockOwnerName = "owner.json"
)

type lockOwner struct {
	PID  int    `json:"pid"`
	Host string `json:"host"`
}

// acquireBuildLock 创建构建锁；遇到残留锁先清掉再试一次。返回释放函数。
func (r *bookRunner) acquireBuildLock(layout bookLayout) (release func(), err error) {
	lockDir := filepath.Join(layout.StateDir, buildLockName)
	for attempt := 0; ; attempt++ {
		err := zipfs.MkdirExclusive(lockDir)
		if err == nil {
			break
		}
		if !errors.Is(err, zipfs.ErrOutputExists) {
			return nil, err
		}
		if attempt > 0 || !r.clearStaleLock(lockDir) {
			return nil, fmt.Errorf("Another build is active (or a lock it cannot verify exists): %s\n"+
				"If no build is running, remove that directory and retry.", lockDir)
		}
	}
	host, _ := os.Hostname()
	data, err := json.Marshal(lockOwner{PID: os.Getpid(), Host: host})
	if err == nil {
		err = zipfs.WriteFileAtomic(filepath.Join(lockDir, lockOwnerName), append(data, '\n'), 0o644)
	}
	if err != nil {
		_ = zipfs.RemoveAll(lockDir)
		return nil, err
	}
	return func() { _ = zipfs.RemoveAll(lockDir) }, nil
}

// clearStaleLock 在锁属于本机已结束的进程时删除它，返回是否已删除。
// 两个进程同时清同一把残留锁的竞态极小（崩溃后恰好并发构建同一本书），不另加保护。
func (r *bookRunner) clearStaleLock(lockDir string) bool {
	owner, ok := readLockOwner(lockDir)
	if !ok {
		return false
	}
	if host, _ := os.Hostname(); owner.Host == "" || owner.Host != host {
		return false
	}
	if owner.PID == os.Getpid() || processAlive(owner.PID) {
		return false
	}
	if err := zipfs.RemoveAll(lockDir); err != nil {
		return false
	}
	fmt.Fprintf(r.bio.out(), "Removed stale build lock left by process %d (no longer running)\n", owner.PID)
	return true
}

func readLockOwner(lockDir string) (lockOwner, bool) {
	data, err := readSmallFile(filepath.Join(lockDir, lockOwnerName))
	if err != nil {
		return lockOwner{}, false
	}
	var owner lockOwner
	if json.Unmarshal(data, &owner) != nil || owner.PID <= 0 {
		return lockOwner{}, false
	}
	return owner, true
}

// buildLocked 报告是否有一把有效的构建锁；残留锁会被清掉。
func (r *bookRunner) buildLocked(layout bookLayout) bool {
	lockDir := filepath.Join(layout.StateDir, buildLockName)
	if _, err := os.Lstat(lockDir); err != nil {
		return false
	}
	return !r.clearStaleLock(lockDir)
}
