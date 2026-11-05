package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// deadPID 远大于常见 pid_max，不会对应运行中的进程。
const deadPID = 0x7ffffff0

func writeLock(t *testing.T, dir string, owner *lockOwner) string {
	t.Helper()
	lock := filepath.Join(dir, ".epub", buildLockName)
	if err := os.MkdirAll(lock, 0o755); err != nil {
		t.Fatal(err)
	}
	if owner != nil {
		data, _ := json.Marshal(owner)
		mustWrite(t, filepath.Join(lock, lockOwnerName), string(data))
	}
	return lock
}

func TestBookBuildClearsLockOfDeadProcess(t *testing.T) {
	h := newBookHarness(t)
	dir := h.newBook("")
	host, _ := os.Hostname()
	lock := writeLock(t, dir, &lockOwner{PID: deadPID, Host: host})
	if code := h.run(dir, "build"); code != 0 {
		t.Fatalf("build with a stale lock = %d: %s", code, h.err)
	}
	if !strings.Contains(h.out.String(), "Removed stale build lock") {
		t.Errorf("build should say it removed the stale lock: %s", h.out)
	}
	if _, err := os.Stat(lock); err == nil {
		t.Error("the lock must be released after the build")
	}
	writeLock(t, dir, &lockOwner{PID: deadPID, Host: host})
	if code := h.run(dir, "clean"); code != 0 {
		t.Errorf("clean with a stale lock = %d: %s", code, h.err)
	}
}

func TestBookBuildKeepsLocksItCannotVerify(t *testing.T) {
	host, _ := os.Hostname()
	cases := map[string]*lockOwner{
		"live process": {PID: os.Getppid(), Host: host},
		"other host":   {PID: deadPID, Host: host + "-elsewhere"},
		"no owner":     nil,
	}
	for name, owner := range cases {
		t.Run(name, func(t *testing.T) {
			h := newBookHarness(t)
			dir := h.newBook("")
			lock := writeLock(t, dir, owner)
			if code := h.run(dir, "build"); code == 0 || !strings.Contains(h.err.String(), "Another build is active") {
				t.Errorf("build must stop: %d %s", code, h.err)
			}
			if code := h.run(dir, "clean"); code == 0 {
				t.Error("clean must stop")
			}
			if _, err := os.Stat(lock); err != nil {
				t.Error("an unverifiable lock must not be removed")
			}
		})
	}
}

func TestBookBuildLockRecordsOwner(t *testing.T) {
	h := newBookHarness(t)
	dir := h.newBook("")
	r := &bookRunner{bio: h.io()}
	layout, err := resolveBookLayout(dir, readBookEnv())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(layout.StateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	release, err := r.acquireBuildLock(layout)
	if err != nil {
		t.Fatal(err)
	}
	owner, ok := readLockOwner(filepath.Join(layout.StateDir, buildLockName))
	if !ok || owner.PID != os.Getpid() {
		t.Errorf("owner = %+v %v", owner, ok)
	}
	if _, err := r.acquireBuildLock(layout); err == nil {
		t.Error("a second acquire by a live owner must fail")
	}
	release()
	if _, err := os.Stat(filepath.Join(layout.StateDir, buildLockName)); err == nil {
		t.Error("release must remove the lock")
	}
}
