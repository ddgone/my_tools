package main

import (
	"context"
	"io"

	"my_tools/tools/python_tools"
)

// pythonToolRun 是 Python 脚本的执行函数签名。
type pythonToolRun func(ctx context.Context, env string, args string, out io.Writer) error

// pythonToolEntry 代表一个内置 Python 工具的运行时信息。
type pythonToolEntry struct {
	scriptName string
	run        pythonToolRun
}

// loadPythonTools 返回内置 Python 工具的运行时表。
// 元数据（名称、分类、参数等）由 manifest 完整提供，
// 此处只负责构造执行闭包。
func loadPythonTools() map[string]*pythonToolEntry {
	bundled := map[string]string{
		"restore_pcd_by_mgrs":    "restore_pcd_by_mgrs.py",
		"python_env_diagnostics": "python_env_diagnostics.py",
	}

	result := make(map[string]*pythonToolEntry, len(bundled))
	for toolID, scriptName := range bundled {
		sn := scriptName
		result[toolID] = &pythonToolEntry{
			scriptName: sn,
			run: func(ctx context.Context, env string, args string, out io.Writer) error {
				return python_tools.RunPythonScript(ctx, env, args, out, sn)
			},
		}
	}
	return result
}
