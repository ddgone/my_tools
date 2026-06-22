package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"fire-salamander-desktop/internal/builder"
	"fire-salamander-desktop/internal/runtimeenv"
)

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
		UseCacheAsOutput: true,
	}
	outputPath := ""
	if req.Mode == artifactBatchModeExport {
		outputPath = filepath.Join(
			req.ExportRootDir,
			manifest.ID,
			fmt.Sprintf("%s_%s", item.TargetOS, item.TargetArch),
			exportDefaultFileName(manifest.Name, manifest.ID, string(manifest.Kind), exportModeBinary, item.TargetOS, item.TargetArch),
		)
	}

	var buildResult builder.BuildResult
	if !req.ForceRebuild && (req.SkipUnchanged || req.PreferCache) {
		probe, probeErr := builder.ProbeBuildCache(buildReq)
		if probeErr != nil {
			item.Message = probeErr.Error()
			return item, probeErr
		}
		item.CacheHit = probe.CacheHit
		item.OutputPath = probe.CachePath

		if probe.CacheHit && req.SkipUnchanged {
			switch req.Mode {
			case artifactBatchModeBuildCache:
				item.Status = "skipped"
				item.Message = "构建输入未变化，已跳过"
				return item, nil
			default:
				same, compareErr := sameArtifactFile(outputPath, probe.CachePath)
				if compareErr == nil && same {
					item.Status = "skipped"
					item.Message = "导出目录已是最新产物，已跳过"
					item.OutputPath = outputPath
					return item, nil
				}
			}
		}

		if probe.CacheHit && req.PreferCache {
			buildResult = probe
		}
	}

	if strings.TrimSpace(buildResult.CachePath) == "" {
		buildReq.ForceRebuild = req.ForceRebuild || !req.PreferCache
		var err error
		buildResult, err = builder.BuildPackage(buildReq)
		if err != nil {
			item.Message = err.Error()
			item.CacheHit = buildResult.CacheHit
			return item, err
		}
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
	a.persistArtifactBatchTasksLocked()
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
	a.persistArtifactBatchTasksLocked()
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
	a.persistArtifactBatchTasksLocked()
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
	a.persistArtifactBatchTasksLocked()
	a.emitArtifactTaskUpdate(cloneArtifactTask(task))
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
