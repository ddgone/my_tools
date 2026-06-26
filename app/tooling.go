package main

import (
	"context"
	"io"
	"sync"

	"my_tools/libs/catalog/builtin"
	"my_tools/libs/core/toolspec"
	"my_tools/tools/python_tools"
)

// ensureTooling 加载 Python 工具和工具清单到 SharedState 中。
// 多次调用安全（sync.Once 保证只初始化一次）。
func ensureTooling(state *SharedState) error {
	toolInitOnce.Do(func() {
		pyTools := loadPythonTools()
		manifests, err := loadToolManifests()
		if err != nil {
			cachedToolingErr = err
			return
		}
		cachedPyTools = pyTools
		cachedManifests = manifests
	})

	if cachedToolingErr != nil {
		return cachedToolingErr
	}

	state.Mu.Lock()
	defer state.Mu.Unlock()
	state.PyTools = cachedPyTools
	state.Manifests = cachedManifests

	return nil
}

var (
	toolInitOnce     sync.Once
	cachedPyTools    map[string]*PythonToolEntry
	cachedManifests  map[string]toolspec.ToolManifest
	cachedToolingErr error
)

// loadPythonTools 注册内置 Python 工具。
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

// loadToolManifests 从内置 manifest 目录加载所有工具规格。
func loadToolManifests() (map[string]toolspec.ToolManifest, error) {
	loaded, err := builtin.Load()
	if err != nil {
		return nil, err
	}

	manifests := make(map[string]toolspec.ToolManifest, len(loaded))
	for _, manifest := range loaded {
		manifests[manifest.ID] = manifest
	}
	return manifests, nil
}
