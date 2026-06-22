package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"fire-salamander-desktop/internal/runtimeenv"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

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

func (a *App) ClearArtifactBatchTasks() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, task := range a.artifactTasks {
		if task != nil && task.Status == "running" {
			return fmt.Errorf("存在进行中的产物任务，暂时无法清空")
		}
	}

	a.artifactTasks = map[string]*ArtifactBatchTask{}
	a.persistArtifactBatchTasksLocked()
	return nil
}

func (a *App) emitArtifactTaskUpdate(task *ArtifactBatchTask) {
	if task == nil || a.ctx == nil {
		return
	}
	wailsruntime.EventsEmit(a.ctx, "artifact:task:update", task)
}

func (a *App) trimArtifactBatchTasksLocked() {
	if len(a.artifactTasks) <= maxArtifactBatchTaskHistory {
		return
	}
	tasks := make([]*ArtifactBatchTask, 0, len(a.artifactTasks))
	for _, task := range a.artifactTasks {
		tasks = append(tasks, task)
	}
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].StartedAt > tasks[j].StartedAt
	})
	for _, task := range tasks[maxArtifactBatchTaskHistory:] {
		delete(a.artifactTasks, task.ID)
	}
}

func artifactBatchTasksFilePath(layout runtimeenv.Layout) string {
	return filepath.Join(layout.ConfigDir(), artifactBatchTasksFileName)
}

func loadArtifactBatchTasksFile(filePath string) ([]*ArtifactBatchTask, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("读取产物任务持久化文件失败: %w", err)
	}

	var tasks []*ArtifactBatchTask
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, fmt.Errorf("解析产物任务持久化文件失败: %w", err)
	}
	return normalizePersistedArtifactTasks(tasks), nil
}

func saveArtifactBatchTasksFile(filePath string, tasks []*ArtifactBatchTask) error {
	normalized := normalizePersistedArtifactTasks(tasks)
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("创建产物任务持久化目录失败: %w", err)
	}
	data, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化产物任务持久化文件失败: %w", err)
	}
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("写入产物任务持久化文件失败: %w", err)
	}
	return nil
}

func normalizePersistedArtifactTasks(tasks []*ArtifactBatchTask) []*ArtifactBatchTask {
	normalized := make([]*ArtifactBatchTask, 0, len(tasks))
	for _, task := range tasks {
		if task == nil || strings.TrimSpace(task.ID) == "" {
			continue
		}
		copyTask := cloneArtifactTask(task)
		recountArtifactTask(copyTask)
		normalized = append(normalized, copyTask)
	}
	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i].StartedAt > normalized[j].StartedAt
	})
	if len(normalized) > maxArtifactBatchTaskHistory {
		normalized = normalized[:maxArtifactBatchTaskHistory]
	}
	return normalized
}

func (a *App) loadArtifactBatchTasks() error {
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

	a.mu.Lock()
	defer a.mu.Unlock()
	a.artifactTasks = make(map[string]*ArtifactBatchTask, len(tasks))
	for _, task := range tasks {
		a.artifactTasks[task.ID] = task
	}
	return nil
}

func (a *App) persistArtifactBatchTasksLocked() {
	layout, err := runtimeenv.ResolveLayout()
	if err != nil {
		fmt.Fprintf(os.Stderr, "持久化产物任务失败: %v\n", err)
		return
	}
	tasks := make([]*ArtifactBatchTask, 0, len(a.artifactTasks))
	for _, task := range a.artifactTasks {
		tasks = append(tasks, task)
	}
	if err := saveArtifactBatchTasksFile(artifactBatchTasksFilePath(layout), tasks); err != nil {
		fmt.Fprintf(os.Stderr, "持久化产物任务失败: %v\n", err)
	}
}
