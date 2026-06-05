package procutil

import (
	"context"
	"os/exec"
)

func Command(name string, arg ...string) *exec.Cmd {
	cmd := exec.Command(name, arg...)
	Prepare(cmd)
	return cmd
}

func CommandContext(ctx context.Context, name string, arg ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, arg...)
	Prepare(cmd)
	return cmd
}

func Prepare(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	configureCommand(cmd)
}
