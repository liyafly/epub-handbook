// Package extern 是外部进程边界的唯一入口（SPEC §1 第 3 层，INV-4）。
//
// caps 不得 import os/exec；工具缺失必须显式降级（返回 ErrToolMissing），
// 由调用方决定跳过还是失败，不许静默忽略。
package extern

import (
	"bytes"
	"errors"
	"os/exec"
)

// ErrToolMissing 表示外部工具在 PATH 中不存在。
var ErrToolMissing = errors.New("extern: tool missing")

// LookPath 报告外部工具是否可用。absent 时返回 (false, nil)。
func LookPath(name string) (bool, error) {
	path, err := exec.LookPath(name)
	if err != nil || path == "" {
		return false, nil
	}
	return true, nil
}

// Require 在工具缺失时返回 ErrToolMissing。
func Require(name string) error {
	ok, err := LookPath(name)
	if err != nil {
		return err
	}
	if !ok {
		return ErrToolMissing
	}
	return nil
}

// CmdResult 是一次外部进程运行的产出。
type CmdResult struct {
	ExitCode int
	Stdout   []byte
	Stderr   []byte
}

// Run 在 dir 工作目录下执行 argv[0]（带完整参数 argv[1:]），
// 捕获 stdout/stderr。工具不存在时返回 ErrToolMissing。
// 允许外部进程自行落盘（例如字体子集化工具）——这是 INV-3 中
// extern 作为磁盘边界的另一半职责。
func Run(dir string, argv []string) (CmdResult, error) {
	if len(argv) == 0 {
		return CmdResult{}, errors.New("extern: empty argv")
	}
	if err := Require(argv[0]); err != nil {
		return CmdResult{}, err
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	err := cmd.Run()
	res := CmdResult{Stdout: out.Bytes(), Stderr: errBuf.Bytes()}
	if exit, ok := err.(*exec.ExitError); ok {
		res.ExitCode = exit.ExitCode()
	} else if err != nil {
		return res, err
	}
	return res, nil
}
