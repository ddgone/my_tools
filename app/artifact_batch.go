package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"fire-salamander-desktop/internal/builder"
	"fire-salamander-desktop/internal/runtimeenv"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	artifactBatchModeBuildCache = "build_cache"
	artifactBatchModeExport     = "export"
)

type ArtifactBatchSelection struct {
	ToolID     string `json:"toolId"`
	TargetOS   string `json:"targetOS"`
	TargetArch string `json:"targetArch"`
}

type ArtifactBatchRequest struct {
	Mode            string                   `json:"mode"`
	ExportRootDir   string                   `json:"exportRootDir,omitempty"`
	Concurrency     int                      `json:"concurrency"`
	SkipUnchanged   bool                     `json:"skipUnchanged"`
	PreferCache     bool                     `json:"preferCache"`
	ForceRebuild    bool                     `json:"forceRebuild"`
	ContinueOnError bool                     `json:"continueOnError"`
	Items           []ArtifactBatchSelection `json:"items"`
}

type ArtifactBatchItemResult struct {
	Key        string `json:"key"`
	ToolID     string `json:"toolId"`
	ToolName   string `json:"toolName"`
	Kind       string `json:"kind"`
	TargetOS   string `json:"targetOS"`
	TargetArch string `json:"targetArch"`
	Status     string `json:"status"`
	Message    string `json:"message"`
	OutputPath string `json:"outputPath,omitempty"`
	CacheHit   bool   `json:"cacheHit"`
	StartedAt  int64  `json:"startedAt"`
	EndedAt    int64  `json:"endedAt,omitempty"`
}

type ArtifactBatchTask struct {
	ID            string                    `json:"id"`
	Mode          string                    `json:"mode"`
	Status        string                    `json:"status"`
	ExportRootDir string                    `json:"exportRootDir,omitempty"`
	TotalCount    int                       `json:"totalCount"`
	SuccessCount  int                       `json:"successCount"`
	ErrorCount    int                       `json:"errorCount"`
	CachedCount   int                       `json:"cachedCount"`
	SkippedCount  int                       `json:"skippedCount"`
	StartedAt     int64                     `json:"startedAt"`
	EndedAt       int64                     `json:"endedAt,omitempty"`
	CurrentItem   string                    `json:"currentItem,omitempty"`
	ExitMessage   string                    `json:"exitMessage,omitempty"`
	Items         []ArtifactBatchItemResult `json:"items"`
}

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

func (a *App) StartArtifactBatch(req ArtifactBatchRequest) (*ArtifactBatchTask, error) {
	if err := a.ensureTooling(); err != nil {
		return nil, err
	}
	resolved, task, err := a.prepareArtifactBatch(req)
	if err != nil {
		return nil, err
	}

	a.mu.Lock()
	a.artifactTasks[task.ID] = task
	snapshot := cloneArtifactTask(task)
	a.mu.Unlock()
	a.emitArtifactTaskUpdate(snapshot)

	go a.runArtifactBatch(task.ID, resolved)

	return snapshot, nil
}

func (a *App) ListArtifactBatchTasks() []*ArtifactBatchTask {
	a.mu.RLock()
	defer a.mu.RUnlock()

	tasks := make([]*ArtifactBatchTask, 0, len(a.artifactTasks))
	for _, task := range a.artifactTasks {
		tasks = append(tasks, cloneArtifactTask(task))
	}
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].StartedAt > tasks[j].StartedAt
	})
	return tasks
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
		if string(manifest.Kind) != "go" {
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

	_ = layout
	taskID := fmt.Sprintf("artifact_%d", time.Now().UnixNano())
	now := time.Now().UnixMilli()
	task := &ArtifactBatchTask{
		ID:            taskID,
		Mode:          mode,
		Status:        "running",
		ExportRootDir: exportRootDir,
		TotalCount:    len(taskItems),
		StartedAt:     now,
		Items:         taskItems,
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

func (a *App) runArtifactBatch(taskID string, req artifactBatchResolvedRequest) {
	layout, err := runtimeenv.ResolveLayout()
	if err != nil {
		a.finishArtifactBatchWithError(taskID, fmt.Errorf("解析运行时目录失败: %w", err))
		return
	}
	if err := layout.Ensure(); err != nil {
		a.finishArtifactBatchWithError(taskID, fmt.Errorf("准备运行时目录失败: %w", err))
		return
	}
	repoRoot, ok := locateRepoRoot()
	if !ok {
		a.finishArtifactBatchWithError(taskID, fmt.Errorf("当前运行环境缺少源码工作区，暂时无法构建单工具产物"))
		return
	}

	jobs := make(chan int)
	var workerWG sync.WaitGroup
	var abortRequested atomic.Bool

	for workerIndex := 0; workerIndex < req.Concurrency; workerIndex++ {
		workerWG.Add(1)
		go func() {
			defer workerWG.Done()
			for index := range jobs {
				if abortRequested.Load() && !req.ContinueOnError {
					return
				}
				if err := a.markArtifactItemRunning(taskID, index); err != nil {
					abortRequested.Store(true)
					return
				}
				result, execErr := a.executeArtifactItem(taskID, req, layout, repoRoot, index)
				if execErr != nil {
					_ = a.completeArtifactItem(taskID, index, result, execErr)
					if !req.ContinueOnError {
						abortRequested.Store(true)
					}
					continue
				}
				_ = a.completeArtifactItem(taskID, index, result, nil)
			}
		}()
	}

	for index := range req.Items {
		if abortRequested.Load() && !req.ContinueOnError {
			break
		}
		jobs <- index
	}
	close(jobs)
	workerWG.Wait()

	a.finalizeArtifactBatch(taskID, abortRequested.Load() && !req.ContinueOnError)
}

func (a *App) executeArtifactItem(taskID string, req artifactBatchResolvedRequest, layout runtimeenv.Layout, repoRoot string, index int) (ArtifactBatchItemResult, error) {
	a.mu.RLock()
	task := a.artifactTasks[taskID]
	item := task.Items[index]
	manifest := a.manifests[item.ToolID]
	a.mu.RUnlock()

	buildReq := builder.BuildRequest{
		ToolID:           manifest.ID,
		ToolName:         manifest.Name,
		Kind:             builderKind(string(manifest.Kind)),
		OutputDir:        layout.BuildCacheDir(),
		CacheDir:         layout.BuildCacheDir(),
		OutputName:       exportDefaultFileName(manifest.Name, manifest.ID, string(manifest.Kind), exportModeBinary, item.TargetOS, item.TargetArch),
		RepoRoot:         repoRoot,
		SourceEntry:      manifest.Source.Entry,
		TargetOS:         item.TargetOS,
		TargetArch:       item.TargetArch,
		ForceRebuild:     req.ForceRebuild || !req.PreferCache,
		UseCacheAsOutput: true,
	}
	buildResult, err := builder.BuildPackage(buildReq)
	if err != nil {
		item.Message = err.Error()
		item.CacheHit = buildResult.CacheHit
		return item, err
	}

	item.CacheHit = buildResult.CacheHit
	item.OutputPath = buildResult.CachePath

	switch req.Mode {
	case artifactBatchModeBuildCache:
		if buildResult.CacheHit && req.SkipUnchanged && !req.ForceRebuild {
			item.Status = "skipped"
			item.Message = "命中构建缓存，已跳过"
			return item, nil
		}
		if buildResult.CacheHit {
			item.Status = "cached"
			item.Message = "命中构建缓存"
			return item, nil
		}
		item.Status = "success"
		item.Message = "已写入构建缓存"
		return item, nil
	default:
		outputPath := filepath.Join(
			req.ExportRootDir,
			manifest.ID,
			fmt.Sprintf("%s_%s", item.TargetOS, item.TargetArch),
			exportDefaultFileName(manifest.Name, manifest.ID, string(manifest.Kind), exportModeBinary, item.TargetOS, item.TargetArch),
		)
		if req.SkipUnchanged && !req.ForceRebuild {
			same, compareErr := sameArtifactFile(outputPath, buildResult.CachePath)
			if compareErr == nil && same {
				item.Status = "skipped"
				item.Message = "导出目录已是最新产物，已跳过"
				item.OutputPath = outputPath
				return item, nil
			}
		}
		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			item.Message = fmt.Sprintf("创建导出目录失败: %v", err)
			return item, err
		}
		if err := copyExportFile(buildResult.CachePath, outputPath, 0755); err != nil {
			item.Message = fmt.Sprintf("写入导出产物失败: %v", err)
			return item, err
		}
		item.OutputPath = outputPath
		if buildResult.CacheHit {
			item.Status = "cached"
			item.Message = "命中缓存并完成导出"
			return item, nil
		}
		item.Status = "success"
		item.Message = "批量导出完成"
		return item, nil
	}
}

func (a *App) markArtifactItemRunning(taskID string, index int) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	task, ok := a.artifactTasks[taskID]
	if !ok {
		return fmt.Errorf("未找到产物任务: %s", taskID)
	}
	if index < 0 || index >= len(task.Items) {
		return fmt.Errorf("产物任务项索引越界")
	}
	task.Items[index].Status = "running"
	task.Items[index].Message = "执行中"
	task.Items[index].StartedAt = time.Now().UnixMilli()
	task.CurrentItem = task.Items[index].ToolName + " " + task.Items[index].TargetOS + "/" + task.Items[index].TargetArch
	recountArtifactTask(task)
	a.emitArtifactTaskUpdate(cloneArtifactTask(task))
	return nil
}

func (a *App) completeArtifactItem(taskID string, index int, result ArtifactBatchItemResult, execErr error) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	task, ok := a.artifactTasks[taskID]
	if !ok {
		return fmt.Errorf("未找到产物任务: %s", taskID)
	}
	if index < 0 || index >= len(task.Items) {
		return fmt.Errorf("产物任务项索引越界")
	}
	result.EndedAt = time.Now().UnixMilli()
	if result.StartedAt == 0 {
		result.StartedAt = result.EndedAt
	}
	if execErr != nil {
		result.Status = "error"
		if strings.TrimSpace(result.Message) == "" {
			result.Message = execErr.Error()
		}
	}
	task.Items[index] = result
	task.CurrentItem = ""
	recountArtifactTask(task)
	a.emitArtifactTaskUpdate(cloneArtifactTask(task))
	return nil
}

func (a *App) finalizeArtifactBatch(taskID string, aborted bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	task, ok := a.artifactTasks[taskID]
	if !ok {
		return
	}
	if aborted {
		for index := range task.Items {
			if task.Items[index].Status == "pending" {
				task.Items[index].Status = "skipped"
				task.Items[index].Message = "前序任务失败，后续已停止"
				task.Items[index].StartedAt = time.Now().UnixMilli()
				task.Items[index].EndedAt = task.Items[index].StartedAt
			}
		}
	}
	task.EndedAt = time.Now().UnixMilli()
	recountArtifactTask(task)
	switch {
	case task.ErrorCount == 0:
		task.Status = "success"
		task.ExitMessage = "批量任务完成"
	case task.SuccessCount > 0 || task.CachedCount > 0 || task.SkippedCount > 0:
		task.Status = "partial"
		task.ExitMessage = "部分任务执行失败"
	default:
		task.Status = "failed"
		task.ExitMessage = "批量任务失败"
	}
	a.emitArtifactTaskUpdate(cloneArtifactTask(task))
}

func (a *App) finishArtifactBatchWithError(taskID string, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	task, ok := a.artifactTasks[taskID]
	if !ok {
		return
	}
	task.Status = "failed"
	task.EndedAt = time.Now().UnixMilli()
	task.ExitMessage = err.Error()
	recountArtifactTask(task)
	a.emitArtifactTaskUpdate(cloneArtifactTask(task))
}

func (a *App) emitArtifactTaskUpdate(task *ArtifactBatchTask) {
	if task == nil || a.ctx == nil {
		return
	}
	wailsruntime.EventsEmit(a.ctx, "artifact:task:update", task)
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

func recountArtifactTask(task *ArtifactBatchTask) {
	task.TotalCount = len(task.Items)
	task.SuccessCount = 0
	task.ErrorCount = 0
	task.CachedCount = 0
	task.SkippedCount = 0
	for _, item := range task.Items {
		switch item.Status {
		case "success":
			task.SuccessCount++
		case "error":
			task.ErrorCount++
		case "cached":
			task.CachedCount++
		case "skipped":
			task.SkippedCount++
		}
	}
}

func cloneArtifactTask(task *ArtifactBatchTask) *ArtifactBatchTask {
	if task == nil {
		return nil
	}
	copyTask := *task
	copyTask.Items = append([]ArtifactBatchItemResult(nil), task.Items...)
	return &copyTask
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
