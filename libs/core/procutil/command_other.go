//go:build !windows

package procutil

import "os/exec"

func configureCommand(cmd *exec.Cmd) {}
