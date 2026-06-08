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

	"my_tools/libs/core/procutil"
	"my_tools/libs/framework"
)

//go:embed scripts/*.py
var pythonScripts embed.FS

type toolMeta struct {
	id       string
	name     string
	category string
	usage    string
}

var scriptMeta = map[string]toolMeta{
	"restore_pcd_by_mgrs.py": {
		id:       "restore_pcd_by_mgrs",
		name:     "白犀牛偏转后的pcd文件转换回未偏转的las文件",
		category: "Python 脚本",
		usage: `[yellow]白犀牛偏转后的 PCD 文件转换回未偏转的 LAS 文件[-]

[cyan]功能说明:[-]
将经过白犀牛偏转处理的 PCD 点云文件，根据文件名中的 MGRS 百米块信息，
反向还原为原始 UTM 坐标系的 LAS 文件。

PCD 文件名必须包含 MGRS 百米块格式（如 50QKL416457），脚本会自动识别
并提供正确的 UTM 偏移量来恢复坐标。

[cyan]参数说明:[-]
  -input  <目录>    必需，包含 .pcd 文件的目录
  -output <目录>    可选，输出 .las 文件的目录，默认在输入目录下创建 output 子文件夹
  -workers <数量>   可选，并行线程数，默认 1

[cyan]使用示例:[-]
  -input D:\pcd_data -output D:\las_output
  -input .\pcd_data -workers 4
`,
	},
	"python_env_diagnostics.py": {
		id:       "python_env_diagnostics",
		name:     "Python 环境诊断与依赖验证",
		category: "Python 脚本",
		usage: `[yellow]Python 环境诊断与依赖验证[-]

[cyan]功能说明:[-]
验证当前 Python 工具是否真的运行在托管虚拟环境中，并输出一份环境诊断文件。
脚本会导入 numpy 与 requests，执行一段小型数值计算，然后把解释器、依赖版本、
site-packages、当前参数和计算结果写入指定 JSON 文件，便于验收 Python 环境配置是否生效。

[cyan]参数说明:[-]
  -output <文件>   必需，输出诊断 JSON 文件路径
  -count  <数量>   可选，生成计算样本数量，默认 1024
  -label  <文本>   可选，写入报告中的诊断标签

[cyan]使用示例:[-]
  -output /tmp/python-diag.json
  -output /tmp/python-diag.json -count 4096 -label smoke-test
`,
	},
}

func ReadEmbeddedScript(scriptName string) ([]byte, error) {
	scriptName = strings.TrimSpace(filepath.Base(scriptName))
	if scriptName == "" {
		return nil, fmt.Errorf("脚本名不能为空")
	}
	return pythonScripts.ReadFile("scripts/" + scriptName)
}

func init() {
	entries, err := fs.ReadDir(pythonScripts, "scripts")
	if err != nil {
		fmt.Printf("无法读取内嵌的 Python 脚本: %v\n", err)
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".py") {
			baseName := strings.TrimSuffix(entry.Name(), ".py")

			if meta, ok := scriptMeta[entry.Name()]; ok {
				framework.Register(&PythonTool{
					scriptName: entry.Name(),
					id:         meta.id,
					name:       meta.name,
					category:   meta.category,
					usage:      meta.usage,
				})
			} else {
				framework.Register(&PythonTool{
					scriptName: entry.Name(),
					id:         "py_" + baseName,
					name:       "Python: " + baseName,
					category:   "Python 脚本",
				})
			}
		}
	}
}

type PythonTool struct {
	scriptName string
	id         string
	name       string
	category   string
	usage      string
}

func (t *PythonTool) ID() string       { return t.id }
func (t *PythonTool) Name() string     { return t.name }
func (t *PythonTool) Category() string { return t.category }

func (t *PythonTool) Execute(ctx framework.AppContext) {
	usage := t.usage
	if usage == "" {
		usage = fmt.Sprintf(`[yellow]内嵌 Python 脚本工具: %s[-]

[cyan]说明:[-]
这是一个通过 Go 工具箱直接调用的 Python 脚本。
脚本执行时，Go 会将其提取到临时目录，并使用系统的 python 命令执行。

[cyan]使用方法:[-]
在下方输入框中直接输入你想要传递给该脚本的参数即可。

[cyan]示例:[-]
arg1 "arg 2 with space" --flag value
`, t.scriptName)
	}

	ctx.ShowPythonTerminal(t.Name(), usage, func(runCtx context.Context, env string, args string, out io.Writer) error {
		// Handle special pip installation command
		if strings.HasPrefix(args, "!pip ") {
			pkgName := strings.TrimPrefix(args, "!pip ")
			cmdArgs := []string{"-m", "pip", "install"}
			cmdArgs = append(cmdArgs, strings.Fields(pkgName)...)

			var cmd *exec.Cmd
			if runtime.GOOS == "windows" {
				cmd = procutil.Command(env, cmdArgs...)
			} else {
				cmd = procutil.CommandContext(runCtx, env, cmdArgs...)
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
					case <-runCtx.Done():
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
		parsedArgs, err := framework.ParseArgs(args)
		if err != nil {
			return err
		}

		// Prepare command
		cmdArgs := append([]string{"-u", tempPath}, parsedArgs...)
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = procutil.Command(env, cmdArgs...)
		} else {
			cmd = procutil.CommandContext(runCtx, env, cmdArgs...)
		}
		preparePythonUTF8Env(cmd)

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
						_ = procutil.Command("taskkill", "/T", "/F", "/PID", fmt.Sprint(cmd.Process.Pid)).Run()
					}
				case <-done:
				}
			}()
		}

		return cmd.Wait()
	})
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
