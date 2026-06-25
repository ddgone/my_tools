package main

import (
	"fmt"
	"os"
	"path/filepath"

	"fire-salamander-desktop/internal/runtimeenv"
)

type CacheCleanupMode string

const (
	CacheCleanupAll      CacheCleanupMode = "all"
	CacheCleanupOrphaned CacheCleanupMode = "orphaned"
)

type CacheInfo struct {
	TotalBytes    int64 `json:"totalBytes"`
	TotalDirs     int   `json:"totalDirs"`
	OrphanedDirs  int   `json:"orphanedDirs"`
	OrphanedBytes int64 `json:"orphanedBytes"`
}

type CacheCleanupResult struct {
	Mode        string `json:"mode"`
	RemovedDirs int    `json:"removedDirs"`
	FreedBytes  int64  `json:"freedBytes"`
	Message     string `json:"message"`
}

// GetCacheInfo 返回当前缓存目录的统计信息。
func (a *App) GetCacheInfo() (CacheInfo, error) {
	if err := a.ensureTooling(); err != nil {
		return CacheInfo{}, err
	}

	layout, err := runtimeenv.ResolveLayout()
	if err != nil {
		return CacheInfo{}, fmt.Errorf("解析运行时目录失败: %w", err)
	}

	a.mu.RLock()
	validIDs := make(map[string]struct{}, len(a.manifests))
	for id := range a.manifests {
		validIDs[id] = struct{}{}
	}
	a.mu.RUnlock()

	cacheDir := layout.CacheDir()
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return CacheInfo{}, nil
		}
		return CacheInfo{}, fmt.Errorf("读取缓存目录失败: %w", err)
	}

	var totalBytes int64
	var totalDirs int
	for _, entry := range entries {
		size, err := dirSize(filepath.Join(cacheDir, entry.Name()))
		if err != nil {
			continue
		}
		totalBytes += size
		totalDirs++
	}

	var orphanedDirs int
	var orphanedBytes int64
	buildCacheDir := layout.BuildCacheDir()
	buildEntries, err := os.ReadDir(buildCacheDir)
	if err == nil {
		for _, entry := range buildEntries {
			if !entry.IsDir() {
				continue
			}
			if _, exists := validIDs[entry.Name()]; !exists {
				size, err := dirSize(filepath.Join(buildCacheDir, entry.Name()))
				if err != nil {
					continue
				}
				orphanedDirs++
				orphanedBytes += size
			}
		}
	}

	return CacheInfo{
		TotalBytes:    totalBytes,
		TotalDirs:     totalDirs,
		OrphanedDirs:  orphanedDirs,
		OrphanedBytes: orphanedBytes,
	}, nil
}

// CleanBuildCache 清理构建缓存。
// mode: "all" 清理整个 cache 目录, "orphaned" 只清理 manifests 中不存在的工具缓存。
func (a *App) CleanBuildCache(mode string) (CacheCleanupResult, error) {
	if err := a.ensureTooling(); err != nil {
		return CacheCleanupResult{}, err
	}

	cleanupMode := CacheCleanupMode(mode)
	switch cleanupMode {
	case CacheCleanupAll:
		return a.cleanAllCache()
	case CacheCleanupOrphaned:
		return a.cleanOrphanedCache()
	default:
		return CacheCleanupResult{}, fmt.Errorf("不支持的清理模式: %s", mode)
	}
}

func (a *App) cleanAllCache() (CacheCleanupResult, error) {
	layout, err := runtimeenv.ResolveLayout()
	if err != nil {
		return CacheCleanupResult{}, fmt.Errorf("解析运行时目录失败: %w", err)
	}

	cacheDir := layout.CacheDir()
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return CacheCleanupResult{Mode: "all", Message: "缓存目录不存在，无需清理"}, nil
		}
		return CacheCleanupResult{}, fmt.Errorf("读取缓存目录失败: %w", err)
	}

	var totalSize int64
	var removedCount int
	for _, entry := range entries {
		subPath := filepath.Join(cacheDir, entry.Name())
		size, err := dirSize(subPath)
		if err != nil {
			continue
		}
		if err := os.RemoveAll(subPath); err != nil {
			return CacheCleanupResult{}, fmt.Errorf("删除失败 %s: %w", subPath, err)
		}
		totalSize += size
		removedCount++
	}

	return CacheCleanupResult{
		Mode:        "all",
		RemovedDirs: removedCount,
		FreedBytes:  totalSize,
		Message:     "缓存已全部清理",
	}, nil
}

func (a *App) cleanOrphanedCache() (CacheCleanupResult, error) {
	layout, err := runtimeenv.ResolveLayout()
	if err != nil {
		return CacheCleanupResult{}, fmt.Errorf("解析运行时目录失败: %w", err)
	}

	buildCacheDir := layout.BuildCacheDir()
	entries, err := os.ReadDir(buildCacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return CacheCleanupResult{Mode: "orphaned", Message: "构建缓存目录不存在，无需清理"}, nil
		}
		return CacheCleanupResult{}, fmt.Errorf("读取构建缓存目录失败: %w", err)
	}

	a.mu.RLock()
	validIDs := make(map[string]struct{}, len(a.manifests))
	for id := range a.manifests {
		validIDs[id] = struct{}{}
	}
	a.mu.RUnlock()

	var totalSize int64
	var removedCount int
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		toolID := entry.Name()
		if _, exists := validIDs[toolID]; exists {
			continue
		}
		subPath := filepath.Join(buildCacheDir, toolID)
		size, err := dirSize(subPath)
		if err != nil {
			continue
		}
		if err := os.RemoveAll(subPath); err != nil {
			return CacheCleanupResult{}, fmt.Errorf("删除失败 %s: %w", subPath, err)
		}
		totalSize += size
		removedCount++
	}

	if removedCount == 0 {
		return CacheCleanupResult{
			Mode:    "orphaned",
			Message: "没有发现无用的脏缓存",
		}, nil
	}

	return CacheCleanupResult{
		Mode:        "orphaned",
		RemovedDirs: removedCount,
		FreedBytes:  totalSize,
		Message:     "已清理无用缓存",
	}, nil
}

func dirSize(path string) (int64, error) {
	var size int64
	err := filepath.WalkDir(path, func(_ string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			info, statErr := entry.Info()
			if statErr != nil {
				return statErr
			}
			size += info.Size()
		}
		return nil
	})
	return size, err
}
