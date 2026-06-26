package main

import "fire-salamander-desktop/internal/cachecleanup"

// Type aliases for Wails binding compatibility.
type (
	CacheCleanupMode   = cachecleanup.Mode
	CacheInfo          = cachecleanup.Info
	CacheCleanupResult = cachecleanup.Result
)

// CacheCleanupManager is an alias for internal use.
type CacheCleanupManager = cachecleanup.Manager

func NewCacheCleanupManager(state *SharedState) *CacheCleanupManager {
	return cachecleanup.NewManager(state)
}

func (a *App) GetCacheInfo() (CacheInfo, error) {
	if err := ensureTooling(a.state); err != nil {
		return CacheInfo{}, err
	}
	return a.cache.GetInfo()
}

func (a *App) CleanBuildCache(mode string) (CacheCleanupResult, error) {
	if err := ensureTooling(a.state); err != nil {
		return CacheCleanupResult{}, err
	}
	return a.cache.Clean(mode)
}
