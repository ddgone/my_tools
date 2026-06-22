package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fire-salamander-desktop/internal/builder"
	"fire-salamander-desktop/internal/runtimeenv"
)

type artifactBatchResolvedRequest struct {
	Mode            string
	ExportRootDir   string
	Concurrency     int
	SkipUnchanged   bool
	PreferCache     bool
	ForceRebuild    bool
	ContinueOnError bool
	Items           []ArtifactBatchSelection
}

func (a *App) EstimateArtifactBatchCache(req ArtifactBatchRequest) (*ArtifactBatchEstimate, error) {
	if err := a.ensureTooling(); err != nil {
		return nil, err
	}
	mode := normalizeArtifactBatchMode(req.Mode)
	repoRoot, ok := locateRepoRoot()
	if !ok || strings.TrimSpace(repoRoot) == "" {
		return nil, fmt.Errorf("当前运行环境缺少源码工作区，暂时无法估算构建缓存")
	}
	layout, err := runtimeenv.ResolveLayout()
	if err != nil {
		return nil, fmt.Errorf("解析运行时目录失败: %w", err)
	}
	if err := layout.Ensure(); err != nil {
		return nil, fmt.Errorf("准备运行时目录失败: %w", err)
	}

	estimate := &ArtifactBatchEstimate{}
	seen := make(map[string]struct{}, len(req.Items))

	a.mu.RLock()
	defer a.mu.RUnlock()

	for _, item := range req.Items {
		key := artifactItemKey(strings.TrimSpace(item.ToolID), strings.TrimSpace(item.TargetOS), strings.TrimSpace(item.TargetArch))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		estimate.TotalCount++

		toolID := strings.TrimSpace(item.ToolID)
		manifest, ok := a.manifests[toolID]
		if !ok || strings.TrimSpace(item.TargetOS) == "" || strings.TrimSpace(item.TargetArch) == "" {
			estimate.InvalidCount++
			continue
		}
		if req.ForceRebuild {
			estimate.BuildCount++
			continue
		}

		probe, probeErr := builder.ProbeBuildCache(builder.BuildRequest{
			ToolID:      manifest.ID,
			ToolName:    manifest.Name,
			Kind:        builderKind(string(manifest.Kind)),
			OutputDir:   layout.BuildCacheDir(),
			CacheDir:    layout.BuildCacheDir(),
			OutputName:  exportDefaultFileName(manifest.Name, manifest.ID, string(manifest.Kind), exportModeBinary, item.TargetOS, item.TargetArch),
			RepoRoot:    repoRoot,
			SourceEntry: manifest.Source.Entry,
			TargetOS:    item.TargetOS,
			TargetArch:  item.TargetArch,
		})
		if probeErr != nil {
			estimate.InvalidCount++
			continue
		}

		if !probe.CacheHit {
			estimate.BuildCount++
			continue
		}

		if req.SkipUnchanged {
			if mode == artifactBatchModeBuildCache {
				estimate.CachedCount++
				continue
			}
			outputPath := filepath.Join(
				strings.TrimSpace(req.ExportRootDir),
				manifest.ID,
				fmt.Sprintf("%s_%s", item.TargetOS, item.TargetArch),
				exportDefaultFileName(manifest.Name, manifest.ID, string(manifest.Kind), exportModeBinary, item.TargetOS, item.TargetArch),
			)
			same, compareErr := sameArtifactFile(outputPath, probe.CachePath)
			if compareErr == nil && same {
				estimate.CachedCount++
				continue
			}
		}

		if req.PreferCache {
			estimate.CachedCount++
		} else {
			estimate.BuildCount++
		}
	}

	return estimate, nil
}

func (a *App) prepareArtifactBatch(req ArtifactBatchRequest) (artifactBatchResolvedRequest, *ArtifactBatchTask, error) {
	mode := normalizeArtifactBatchMode(req.Mode)
	if len(req.Items) == 0 {
		return artifactBatchResolvedRequest{}, nil, fmt.Errorf("请至少选择一个工具目标")
	}

	layout, err := runtimeenv.ResolveLayout()
	if err != nil {
		return artifactBatchResolvedRequest{}, nil, fmt.Errorf("解析运行时目录失败: %w", err)
	}
	if err := layout.Ensure(); err != nil {
		return artifactBatchResolvedRequest{}, nil, fmt.Errorf("准备运行时目录失败: %w", err)
	}
	repoRoot, ok := locateRepoRoot()
	if !ok {
		return artifactBatchResolvedRequest{}, nil, fmt.Errorf("当前运行环境缺少源码工作区，暂时无法构建单工具产物")
	}
	if repoRoot == "" {
		return artifactBatchResolvedRequest{}, nil, fmt.Errorf("未找到源码工作区")
	}

	exportRootDir := strings.TrimSpace(req.ExportRootDir)
	if mode == artifactBatchModeExport && exportRootDir == "" {
		return artifactBatchResolvedRequest{}, nil, fmt.Errorf("请选择批量导出目录")
	}
	if mode == artifactBatchModeExport {
		if err := os.MkdirAll(exportRootDir, 0755); err != nil {
			return artifactBatchResolvedRequest{}, nil, fmt.Errorf("创建导出目录失败: %w", err)
		}
	}

	concurrency := req.Concurrency
	if concurrency <= 0 {
		concurrency = 4
	}
	if concurrency > 8 {
		concurrency = 8
	}

	items := make([]ArtifactBatchSelection, 0, len(req.Items))
	taskItems := make([]ArtifactBatchItemResult, 0, len(req.Items))
	seen := make(map[string]struct{}, len(req.Items))

	a.mu.RLock()
	defer a.mu.RUnlock()

	for _, item := range req.Items {
		toolID := strings.TrimSpace(item.ToolID)
		targetOS := strings.TrimSpace(item.TargetOS)
		targetArch := strings.TrimSpace(item.TargetArch)
		if toolID == "" || targetOS == "" || targetArch == "" {
			return artifactBatchResolvedRequest{}, nil, fmt.Errorf("存在不完整的工具目标选择")
		}
		manifest, ok := a.manifests[toolID]
		if !ok {
			return artifactBatchResolvedRequest{}, nil, fmt.Errorf("未找到工具: %s", toolID)
		}
		if string(manifest.Kind) != "go" && string(manifest.Kind) != "rust" {
			return artifactBatchResolvedRequest{}, nil, fmt.Errorf("工具 %s 当前仅支持脚本导出，暂不支持批量二进制产物", manifest.Name)
		}
		key := artifactItemKey(toolID, targetOS, targetArch)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		items = append(items, ArtifactBatchSelection{
			ToolID:     toolID,
			TargetOS:   targetOS,
			TargetArch: targetArch,
		})
		taskItems = append(taskItems, ArtifactBatchItemResult{
			Key:        key,
			ToolID:     toolID,
			ToolName:   manifest.Name,
			Kind:       string(manifest.Kind),
			TargetOS:   targetOS,
			TargetArch: targetArch,
			Status:     "pending",
			Message:    "等待执行",
		})
	}

	if len(items) == 0 {
		return artifactBatchResolvedRequest{}, nil, fmt.Errorf("没有可执行的批量产物项")
	}

	taskID := fmt.Sprintf("artifact_%d", time.Now().UnixNano())
	now := time.Now().UnixMilli()
	task := &ArtifactBatchTask{
		ID:              taskID,
		Mode:            mode,
		Status:          "running",
		ExportRootDir:   exportRootDir,
		Concurrency:     concurrency,
		SkipUnchanged:   req.SkipUnchanged,
		PreferCache:     req.PreferCache,
		ForceRebuild:    req.ForceRebuild,
		ContinueOnError: req.ContinueOnError,
		TotalCount:      len(taskItems),
		StartedAt:       now,
		Items:           taskItems,
	}

	return artifactBatchResolvedRequest{
		Mode:            mode,
		ExportRootDir:   exportRootDir,
		Concurrency:     concurrency,
		SkipUnchanged:   req.SkipUnchanged,
		PreferCache:     req.PreferCache,
		ForceRebuild:    req.ForceRebuild,
		ContinueOnError: req.ContinueOnError,
		Items:           items,
	}, task, nil
}

func normalizeArtifactBatchMode(mode string) string {
	if strings.TrimSpace(mode) == artifactBatchModeBuildCache {
		return artifactBatchModeBuildCache
	}
	return artifactBatchModeExport
}

func artifactItemKey(toolID string, targetOS string, targetArch string) string {
	return fmt.Sprintf("%s:%s/%s", toolID, targetOS, targetArch)
}

func sameArtifactFile(targetPath string, cachePath string) (bool, error) {
	targetPath = strings.TrimSpace(targetPath)
	cachePath = strings.TrimSpace(cachePath)
	if targetPath == "" || cachePath == "" {
		return false, nil
	}
	if _, err := os.Stat(targetPath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	targetDigest, err := digestFile(targetPath)
	if err != nil {
		return false, err
	}
	cacheDigest, err := digestFile(cachePath)
	if err != nil {
		return false, err
	}
	return targetDigest == cacheDigest, nil
}

func digestFile(path string) ([32]byte, error) {
	var zero [32]byte
	file, err := os.Open(path)
	if err != nil {
		return zero, err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return zero, err
	}
	var sum [32]byte
	copy(sum[:], digest.Sum(nil))
	return sum, nil
}
