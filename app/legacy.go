package main

import (
	"sync"

	"my_tools/libs/catalog/builtin"
	"my_tools/libs/core/toolspec"
)

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

var (
	toolInitOnce     sync.Once
	cachedPyTools    map[string]*pythonToolEntry
	cachedManifests  map[string]toolspec.ToolManifest
	cachedToolingErr error
)
