package main

import (
	"fmt"
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

type ArtifactBatchManager struct {
	state *SharedState
}

func NewArtifactBatchManager(state *SharedState) *ArtifactBatchManager {
	return &ArtifactBatchManager{state: state}
}

// App delegates
func (a *App) StartArtifactBatch(req ArtifactBatchRequest) (*ArtifactBatchTask, error) {
	return a.artifact.StartArtifactBatch(req)
}

func (a *App) ListArtifactBatchTasks() []*ArtifactBatchTask {
	return a.artifact.ListArtifactBatchTasks()
}

func (a *App) ClearArtifactBatchTasks() error {
	return a.artifact.ClearArtifactBatchTasks()
}

func (a *App) EstimateArtifactBatchCache(req ArtifactBatchRequest) (*ArtifactBatchEstimate, error) {
	return a.artifact.EstimateArtifactBatchCache(req)
}

func (a *App) loadArtifactBatchTasks() error {
	return a.artifact.loadArtifactBatchTasks()
}

func (a *App) emitArtifactTaskUpdate(task *ArtifactBatchTask) {
	a.artifact.emitArtifactTaskUpdate(task)
}

func (m *ArtifactBatchManager) StartArtifactBatch(req ArtifactBatchRequest) (*ArtifactBatchTask, error) {
	if err := ensureTooling(m.state); err != nil {
		return nil, err
	}
	resolved, task, err := m.prepareArtifactBatch(req)
	if err != nil {
		return nil, err
	}

	m.state.Mu.Lock()
	m.state.ArtifactTasks[task.ID] = task
	m.trimArtifactBatchTasksLocked()
	m.persistArtifactBatchTasksLocked()
	snapshot := cloneArtifactTask(task)
	m.state.Mu.Unlock()
	m.emitArtifactTaskUpdate(snapshot)

	go m.runArtifactBatch(task.ID, resolved)

	return snapshot, nil
}

func (m *ArtifactBatchManager) ListArtifactBatchTasks() []*ArtifactBatchTask {
	m.state.Mu.RLock()
	defer m.state.Mu.RUnlock()

	tasks := make([]*ArtifactBatchTask, 0, len(m.state.ArtifactTasks))
	for _, task := range m.state.ArtifactTasks {
		tasks = append(tasks, cloneArtifactTask(task))
	}
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].StartedAt > tasks[j].StartedAt
	})
	return tasks
}

func (m *ArtifactBatchManager) ClearArtifactBatchTasks() error {
	m.state.Mu.Lock()
	defer m.state.Mu.Unlock()

	for _, task := range m.state.ArtifactTasks {
		if task != nil && task.Status == "running" {
			return fmt.Errorf("存在进行中的产物任务，暂时无法清空")
		}
	}

	m.state.ArtifactTasks = map[string]*ArtifactBatchTask{}
	m.persistArtifactBatchTasksLocked()
	return nil
}

func (m *ArtifactBatchManager) emitArtifactTaskUpdate(task *ArtifactBatchTask) {
	if task == nil || m.state.Ctx == nil {
		return
	}
	wailsruntime.EventsEmit(m.state.Ctx, "artifact:task:update", task)
}

func (m *ArtifactBatchManager) EstimateArtifactBatchCache(req ArtifactBatchRequest) (*ArtifactBatchEstimate, error) {
	if err := ensureTooling(m.state); err != nil {
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

	m.state.Mu.RLock()
	defer m.state.Mu.RUnlock()

	for _, item := range req.Items {
		key := artifactItemKey(strings.TrimSpace(item.ToolID), strings.TrimSpace(item.TargetOS), strings.TrimSpace(item.TargetArch))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		estimate.TotalCount++

		toolID := strings.TrimSpace(item.ToolID)
		manifest, ok := m.state.Manifests[toolID]
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

func (m *ArtifactBatchManager) prepareArtifactBatch(req ArtifactBatchRequest) (artifactBatchResolvedRequest, *ArtifactBatchTask, error) {
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

	m.state.Mu.RLock()
	defer m.state.Mu.RUnlock()

	for _, item := range req.Items {
		toolID := strings.TrimSpace(item.ToolID)
		targetOS := strings.TrimSpace(item.TargetOS)
		targetArch := strings.TrimSpace(item.TargetArch)
		if toolID == "" || targetOS == "" || targetArch == "" {
			return artifactBatchResolvedRequest{}, nil, fmt.Errorf("存在不完整的工具目标选择")
		}
		manifest, ok := m.state.Manifests[toolID]
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

func (m *ArtifactBatchManager) runArtifactBatch(taskID string, req artifactBatchResolvedRequest) {
	layout, err := runtimeenv.ResolveLayout()
	if err != nil {
		m.finishArtifactBatchWithError(taskID, fmt.Errorf("解析运行时目录失败: %w", err))
		return
	}
	if err := layout.Ensure(); err != nil {
		m.finishArtifactBatchWithError(taskID, fmt.Errorf("准备运行时目录失败: %w", err))
		return
	}
	repoRoot, ok := locateRepoRoot()
	if !ok {
		m.finishArtifactBatchWithError(taskID, fmt.Errorf("当前运行环境缺少源码工作区，暂时无法构建单工具产物"))
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
				if err := m.markArtifactItemRunning(taskID, index); err != nil {
					abortRequested.Store(true)
					return
				}
				result, execErr := m.executeArtifactItem(taskID, req, layout, repoRoot, index)
				if execErr != nil {
					_ = m.completeArtifactItem(taskID, index, result, execErr)
					if !req.ContinueOnError {
						abortRequested.Store(true)
					}
					continue
				}
				_ = m.completeArtifactItem(taskID, index, result, nil)
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

	m.finalizeArtifactBatch(taskID, abortRequested.Load() && !req.ContinueOnError)
}

func (m *ArtifactBatchManager) executeArtifactItem(taskID string, req artifactBatchResolvedRequest, layout runtimeenv.Layout, repoRoot string, index int) (ArtifactBatchItemResult, error) {
	m.state.Mu.RLock()
	task := m.state.ArtifactTasks[taskID]
	item := task.Items[index]
	manifest := m.state.Manifests[item.ToolID]
	m.state.Mu.RUnlock()

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

func (m *ArtifactBatchManager) markArtifactItemRunning(taskID string, index int) error {
	m.state.Mu.Lock()
	defer m.state.Mu.Unlock()
	task, ok := m.state.ArtifactTasks[taskID]
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
	m.persistArtifactBatchTasksLocked()
	m.emitArtifactTaskUpdate(cloneArtifactTask(task))
	return nil
}

func (m *ArtifactBatchManager) completeArtifactItem(taskID string, index int, result ArtifactBatchItemResult, execErr error) error {
	m.state.Mu.Lock()
	defer m.state.Mu.Unlock()
	task, ok := m.state.ArtifactTasks[taskID]
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
	m.persistArtifactBatchTasksLocked()
	m.emitArtifactTaskUpdate(cloneArtifactTask(task))
	return nil
}

func (m *ArtifactBatchManager) finalizeArtifactBatch(taskID string, aborted bool) {
	m.state.Mu.Lock()
	defer m.state.Mu.Unlock()
	task, ok := m.state.ArtifactTasks[taskID]
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
	m.persistArtifactBatchTasksLocked()
	m.emitArtifactTaskUpdate(cloneArtifactTask(task))
}

func (m *ArtifactBatchManager) finishArtifactBatchWithError(taskID string, err error) {
	m.state.Mu.Lock()
	defer m.state.Mu.Unlock()
	task, ok := m.state.ArtifactTasks[taskID]
	if !ok {
		return
	}
	task.Status = "failed"
	task.EndedAt = time.Now().UnixMilli()
	task.ExitMessage = err.Error()
	recountArtifactTask(task)
	m.persistArtifactBatchTasksLocked()
	m.emitArtifactTaskUpdate(cloneArtifactTask(task))
}

func (m *ArtifactBatchManager) trimArtifactBatchTasksLocked() {
	if len(m.state.ArtifactTasks) <= maxArtifactBatchTaskHistory {
		return
	}
	tasks := make([]*ArtifactBatchTask, 0, len(m.state.ArtifactTasks))
	for _, task := range m.state.ArtifactTasks {
		tasks = append(tasks, task)
	}
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].StartedAt > tasks[j].StartedAt
	})
	for _, task := range tasks[maxArtifactBatchTaskHistory:] {
		delete(m.state.ArtifactTasks, task.ID)
	}
}

func (m *ArtifactBatchManager) loadArtifactBatchTasks() error {
	layout, err := runtimeenv.ResolveLayout()
	if err != nil {
		return fmt.Errorf("解析运行时目录失败: %w", err)
	}
	if err := layout.Ensure(); err != nil {
		return fmt.Errorf("准备运行时目录失败: %w", err)
	}
	tasks, err := loadArtifactBatchTasksFile(artifactBatchTasksFilePath(layout))
	if err != nil {
		return err
	}

	m.state.Mu.Lock()
	defer m.state.Mu.Unlock()
	m.state.ArtifactTasks = make(map[string]*ArtifactBatchTask, len(tasks))
	for _, task := range tasks {
		m.state.ArtifactTasks[task.ID] = task
	}
	return nil
}

func (m *ArtifactBatchManager) persistArtifactBatchTasksLocked() {
	layout, err := runtimeenv.ResolveLayout()
	if err != nil {
		fmt.Fprintf(os.Stderr, "持久化产物任务失败: %v\n", err)
		return
	}
	tasks := make([]*ArtifactBatchTask, 0, len(m.state.ArtifactTasks))
	for _, task := range m.state.ArtifactTasks {
		tasks = append(tasks, task)
	}
	if err := saveArtifactBatchTasksFile(artifactBatchTasksFilePath(layout), tasks); err != nil {
		fmt.Fprintf(os.Stderr, "持久化产物任务失败: %v\n", err)
	}
}
