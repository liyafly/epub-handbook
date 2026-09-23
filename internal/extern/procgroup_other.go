//go:build !unix

package extern

import "os/exec"

func configureProcessGroup(*exec.Cmd) {}
