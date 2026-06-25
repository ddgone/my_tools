package main

import (
	"context"
	"io"

	"my_tools/tools/python_tools"
)

func loadPythonTools() map[string]*PythonToolEntry {
	bundled := map[string]string{
		"restore_pcd_by_mgrs":    "restore_pcd_by_mgrs.py",
		"python_env_diagnostics": "python_env_diagnostics.py",
	}

	result := make(map[string]*PythonToolEntry, len(bundled))
	for toolID, scriptName := range bundled {
		sn := scriptName
		result[toolID] = &PythonToolEntry{
			ScriptName: sn,
			Run: func(ctx context.Context, env string, args string, out io.Writer) error {
				return python_tools.RunPythonScript(ctx, env, args, out, sn)
			},
		}
	}
	return result
}
