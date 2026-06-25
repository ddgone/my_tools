package main

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
