package python_tools

import (
	"context"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"my_tools/pkg/framework"
)

//go:embed scripts/*.py
var pythonScripts embed.FS

func init() {
	// Scan embedded scripts and register them as tools
	entries, err := fs.ReadDir(pythonScripts, "scripts")
	if err != nil {
		fmt.Printf("无法读取内嵌的 Python 脚本: %v\n", err)
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".py") {
			baseName := strings.TrimSuffix(entry.Name(), ".py")
			framework.Register(&PythonTool{
				scriptName: entry.Name(),
				id:         "py_" + baseName,
				name:       "Python: " + baseName,
				category:   "Python 脚本",
			})
		}
	}
}

type PythonTool struct {
	scriptName string
	id         string
	name       string
	category   string
}

func (t *PythonTool) ID() string       { return t.id }
func (t *PythonTool) Name() string     { return t.name }
func (t *PythonTool) Category() string { return t.category }

func (t *PythonTool) Execute(ctx framework.AppContext) {
	usage := fmt.Sprintf(`[yellow]内嵌 Python 脚本工具: %s[-]

[cyan]说明:[-]
这是一个通过 Go 工具箱直接调用的 Python 脚本。
脚本执行时，Go 会将其提取到临时目录，并使用系统的 python 命令执行。

[cyan]使用方法:[-]
在下方输入框中直接输入你想要传递给该脚本的参数即可。

[cyan]示例:[-]
arg1 "arg 2 with space" --flag value
`, t.scriptName)

	ctx.ShowPythonTerminal(t.Name(), usage, func(runCtx context.Context, env string, args string, out io.Writer) error {
		// Handle special pip installation command
		if strings.HasPrefix(args, "!pip ") {
			pkgName := strings.TrimPrefix(args, "!pip ")
			cmdArgs := []string{"-m", "pip", "install"}
			cmdArgs = append(cmdArgs, strings.Fields(pkgName)...)

			var cmd *exec.Cmd
			if runtime.GOOS == "windows" {
				cmd = exec.Command(env, cmdArgs...)
			} else {
				cmd = exec.CommandContext(runCtx, env, cmdArgs...)
			}

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
					case <-runCtx.Done():
						if cmd.Process != nil {
							_ = exec.Command("taskkill", "/T", "/F", "/PID", fmt.Sprint(cmd.Process.Pid)).Run()
						}
					case <-done:
					}
				}()
			}

			return cmd.Wait()
		}

		// Read script from embed FS
		content, err := pythonScripts.ReadFile("scripts/" + t.scriptName)
		if err != nil {
			return fmt.Errorf("读取内嵌脚本失败: %v", err)
		}

		// Create a temporary file
		tempDir := os.TempDir()
		tempPath := filepath.Join(tempDir, t.scriptName)
		err = os.WriteFile(tempPath, content, 0644)
		if err != nil {
			return fmt.Errorf("释放临时脚本失败: %v", err)
		}

		// Note: We don't remove the temp file immediately because the execution is synchronous,
		// but we can clean it up after the command finishes.
		defer os.Remove(tempPath)

		// Parse arguments using framework parser
		parsedArgs := framework.ParseArgs(args)

		// Prepare command
		cmdArgs := append([]string{tempPath}, parsedArgs...)
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.Command(env, cmdArgs...)
		} else {
			cmd = exec.CommandContext(runCtx, env, cmdArgs...)
		}

		// Fallback logic is removed because user explicitly specified the env
		// The TUI enforces "python" as the default if empty

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
				case <-runCtx.Done():
					if cmd.Process != nil {
						_ = exec.Command("taskkill", "/T", "/F", "/PID", fmt.Sprint(cmd.Process.Pid)).Run()
					}
				case <-done:
				}
			}()
		}

		return cmd.Wait()
	})
}
