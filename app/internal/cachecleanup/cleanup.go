package cachecleanup

import (
	"fmt"
	"os"
	"path/filepath"

	"fire-salamander-desktop/internal/runtimeenv"
	"fire-salamander-desktop/internal/shared"
)

type Mode string

const (
	ModeAll      Mode = "all"
	ModeOrphaned Mode = "orphaned"
)

type Info struct {
	TotalBytes    int64 `json:"totalBytes"`
	TotalDirs     int   `json:"totalDirs"`
	OrphanedDirs  int   `json:"orphanedDirs"`
	OrphanedBytes int64 `json:"orphanedBytes"`
}

type Result struct {
	Mode        string `json:"mode"`
	RemovedDirs int    `json:"removedDirs"`
	FreedBytes  int64  `json:"freedBytes"`
	Message     string `json:"message"`
}

type Manager struct {
	state       *shared.SharedState
	layout      runtimeenv.Layout
	layoutErr   error
	layoutReady bool
}

func NewManager(state *shared.SharedState) *Manager {
	return &Manager{state: state}
}

func (m *Manager) resolveLayout() error {
	if m.layoutReady {
		return m.layoutErr
	}
	m.layout, m.layoutErr = runtimeenv.ResolveLayout()
	m.layoutReady = true
	return m.layoutErr
}

// GetInfo 返回当前缓存目录的统计信息。
func (m *Manager) GetInfo() (Info, error) {
	if err := m.resolveLayout(); err != nil {
		return Info{}, fmt.Errorf("解析运行时目录失败: %w", err)
	}
	m.state.Mu.RLock()
	validIDs := make(map[string]struct{}, len(m.state.Manifests))
	for id := range m.state.Manifests {
		validIDs[id] = struct{}{}
	}
	m.state.Mu.RUnlock()

	cacheDir := m.layout.CacheDir()
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return Info{}, nil
		}
		return Info{}, fmt.Errorf("读取缓存目录失败: %w", err)
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
	buildCacheDir := m.layout.BuildCacheDir()
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

	return Info{
		TotalBytes:    totalBytes,
		TotalDirs:     totalDirs,
		OrphanedDirs:  orphanedDirs,
		OrphanedBytes: orphanedBytes,
	}, nil
}

// Clean 清理构建缓存。mode: "all" 或 "orphaned"。
func (m *Manager) Clean(mode string) (Result, error) {
	if err := m.resolveLayout(); err != nil {
		return Result{}, fmt.Errorf("解析运行时目录失败: %w", err)
	}
	cleanupMode := Mode(mode)
	switch cleanupMode {
	case ModeAll:
		return m.cleanAll()
	case ModeOrphaned:
		return m.cleanOrphaned()
	default:
		return Result{}, fmt.Errorf("不支持的清理模式: %s", mode)
	}
}

func (m *Manager) cleanAll() (Result, error) {
	cacheDir := m.layout.CacheDir()
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return Result{Mode: "all", Message: "缓存目录不存在，无需清理"}, nil
		}
		return Result{}, fmt.Errorf("读取缓存目录失败: %w", err)
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
			return Result{}, fmt.Errorf("删除失败 %s: %w", subPath, err)
		}
		totalSize += size
		removedCount++
	}

	return Result{
		Mode:        "all",
		RemovedDirs: removedCount,
		FreedBytes:  totalSize,
		Message:     "缓存已全部清理",
	}, nil
}

func (m *Manager) cleanOrphaned() (Result, error) {
	buildCacheDir := m.layout.BuildCacheDir()
	entries, err := os.ReadDir(buildCacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return Result{Mode: "orphaned", Message: "构建缓存目录不存在，无需清理"}, nil
		}
		return Result{}, fmt.Errorf("读取构建缓存目录失败: %w", err)
	}

	m.state.Mu.RLock()
	validIDs := make(map[string]struct{}, len(m.state.Manifests))
	for id := range m.state.Manifests {
		validIDs[id] = struct{}{}
	}
	m.state.Mu.RUnlock()

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
			return Result{}, fmt.Errorf("删除失败 %s: %w", subPath, err)
		}
		totalSize += size
		removedCount++
	}

	if removedCount == 0 {
		return Result{
			Mode:    "orphaned",
			Message: "没有发现无用的脏缓存",
		}, nil
	}

	return Result{
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
