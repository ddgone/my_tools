package procutil

import (
	"os/exec"
	"syscall"
)

func configureCommand(cmd *exec.Cmd) {
	attrs := cmd.SysProcAttr
	if attrs == nil {
		attrs = &syscall.SysProcAttr{}
	}
	attrs.HideWindow = true
	cmd.SysProcAttr = attrs
}
