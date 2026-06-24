package python_tools

import (
	"context"
	"embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"my_tools/libs/core/procutil"
)

//go:embed scripts/*.py
var pythonScripts embed.FS

// ReadEmbeddedScript 读取内嵌的 Python 脚本内容。
func ReadEmbeddedScript(scriptName string) ([]byte, error) {
	scriptName = strings.TrimSpace(filepath.Base(scriptName))
	if scriptName == "" {
		return nil, fmt.Errorf("脚本名不能为空")
	}
	return pythonScripts.ReadFile("scripts/" + scriptName)
}

// RunPythonScript 执行内嵌的 Python 脚本。会将脚本释放到临时目录，
// 用指定解释器子进程执行，stdout+stderr 合并写入 out。
func RunPythonScript(ctx context.Context, env string, args string, out io.Writer, scriptName string) error {
	// Handle special pip installation command
	if strings.HasPrefix(args, "!pip ") {
		pkgName := strings.TrimPrefix(args, "!pip ")
		cmdArgs := []string{"-m", "pip", "install"}
		cmdArgs = append(cmdArgs, strings.Fields(pkgName)...)

		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = procutil.Command(env, cmdArgs...)
		} else {
			cmd = procutil.CommandContext(ctx, env, cmdArgs...)
		}
		preparePythonUTF8Env(cmd)
		cmd.Stdout = out
		cmd.Stderr = out

		if err := cmd.Start(); err != nil {
			return err
		}

		done := make(chan struct{})
		defer close(done)

		if runtime.GOOS == "windows" {
			go func() {
				select {
				case <-ctx.Done():
					if cmd.Process != nil {
						_ = procutil.Command("taskkill", "/T", "/F", "/PID", fmt.Sprint(cmd.Process.Pid)).Run()
					}
				case <-done:
				}
			}()
		}

		return cmd.Wait()
	}

	// Read script from embed FS
	content, err := pythonScripts.ReadFile("scripts/" + scriptName)
	if err != nil {
		return fmt.Errorf("读取内嵌脚本失败: %v", err)
	}

	// Create a temporary file
	tempDir := os.TempDir()
	tempPath := filepath.Join(tempDir, scriptName)
	err = os.WriteFile(tempPath, content, 0644)
	if err != nil {
		return fmt.Errorf("释放临时脚本失败: %v", err)
	}
	defer os.Remove(tempPath)

	// Parse arguments
	parsedArgs, err := procutil.ParseArgs(args)
	if err != nil {
		return err
	}

	// Prepare command
	cmdArgs := append([]string{"-u", tempPath}, parsedArgs...)
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = procutil.Command(env, cmdArgs...)
	} else {
		cmd = procutil.CommandContext(ctx, env, cmdArgs...)
	}
	preparePythonUTF8Env(cmd)
	cmd.Stdout = out
	cmd.Stderr = out

	if err := cmd.Start(); err != nil {
		return err
	}

	done := make(chan struct{})
	defer close(done)

	if runtime.GOOS == "windows" {
		go func() {
			select {
			case <-ctx.Done():
				if cmd.Process != nil {
					_ = procutil.Command("taskkill", "/T", "/F", "/PID", fmt.Sprint(cmd.Process.Pid)).Run()
				}
			case <-done:
			}
		}()
	}

	return cmd.Wait()
}

func preparePythonUTF8Env(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.Env = append(os.Environ(),
		"PYTHONIOENCODING=UTF-8",
		"PYTHONUTF8=1",
	)
}
