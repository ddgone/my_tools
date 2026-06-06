package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"fire-salamander-desktop/internal/toolchain"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type PythonToolchainTask struct {
	Kind                 string  `json:"kind"`
	Status               string  `json:"status"`
	Message              string  `json:"message"`
	Detail               string  `json:"detail,omitempty"`
	CurrentItem          string  `json:"currentItem,omitempty"`
	ProgressPercent      float64 `json:"progressPercent"`
	Step                 int     `json:"step"`
	TotalSteps           int     `json:"totalSteps"`
	BaseBinary           string  `json:"baseBinary,omitempty"`
	EnvironmentDirectory string  `json:"environmentDirectory,omitempty"`
	Error                string  `json:"error,omitempty"`
	UpdatedAt            int64   `json:"updatedAt"`
}

func (a *App) getPythonToolchainTaskState() *PythonToolchainTask {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return clonePythonToolchainTask(a.pythonTask)
}

func (a *App) startPreparePythonToolchainTask() (*PythonToolchainTask, error) {
	return a.startPythonToolchainTask(toolchain.PythonOperationPrepare)
}

func (a *App) startInstallPythonDependenciesTask() (*PythonToolchainTask, error) {
	return a.startPythonToolchainTask(toolchain.PythonOperationInstall)
}

func (a *App) cancelPythonToolchainTask() error {
	a.mu.Lock()
	cancel := a.pythonCancel
	a.mu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	return nil
}

func (a *App) startPythonToolchainTask(kind toolchain.PythonOperationKind) (*PythonToolchainTask, error) {
	a.mu.Lock()
	if a.pythonTask != nil && a.pythonTask.Status == "running" {
		task := clonePythonToolchainTask(a.pythonTask)
		a.mu.Unlock()
		return task, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	task := &PythonToolchainTask{
		Kind:            string(kind),
		Status:          "running",
		Message:         "准备执行 Python 环境任务",
		ProgressPercent: 0,
		UpdatedAt:       time.Now().UnixMilli(),
	}
	a.pythonTask = task
	a.pythonCancel = cancel
	copyTask := clonePythonToolchainTask(task)
	a.mu.Unlock()
	a.emitPythonToolchainTask(copyTask)

	go func() {
		var runErr error
		hooks := &toolchain.PythonOperationHooks{
			OnProgress: func(progress toolchain.PythonOperationProgress) {
				a.updatePythonToolchainTask(func(task *PythonToolchainTask) {
					task.Kind = string(progress.Kind)
					task.Status = "running"
					task.Message = strings.TrimSpace(progress.Message)
					task.Detail = strings.TrimSpace(progress.Detail)
					task.CurrentItem = strings.TrimSpace(progress.CurrentItem)
					task.ProgressPercent = progress.ProgressPercent
					task.Step = progress.Step
					task.TotalSteps = progress.TotalSteps
					task.BaseBinary = strings.TrimSpace(progress.BaseBinary)
					task.EnvironmentDirectory = strings.TrimSpace(progress.EnvironmentDirectory)
					task.Error = ""
				})
			},
		}
		switch kind {
		case toolchain.PythonOperationPrepare:
			_, runErr = toolchain.PrepareManagedPythonEnvironmentWithOptions(ctx, hooks)
		case toolchain.PythonOperationInstall:
			_, runErr = toolchain.InstallPythonDependenciesWithOptions(ctx, hooks)
		}
		if errors.Is(runErr, context.Canceled) {
			a.finishPythonToolchainTask("canceled", "Python 环境任务已停止", runErr)
			return
		}
		if runErr != nil {
			a.finishPythonToolchainTask("failed", "Python 环境任务失败", runErr)
			return
		}
		a.finishPythonToolchainTask("completed", "Python 环境任务已完成", nil)
	}()

	return copyTask, nil
}

func (a *App) updatePythonToolchainTask(update func(task *PythonToolchainTask)) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.pythonTask == nil {
		return
	}
	update(a.pythonTask)
	a.pythonTask.UpdatedAt = time.Now().UnixMilli()
	a.emitPythonToolchainTaskLocked()
}

func (a *App) finishPythonToolchainTask(status string, message string, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.pythonTask == nil {
		return
	}
	a.pythonTask.Status = status
	a.pythonTask.Message = message
	a.pythonTask.ProgressPercent = clampPythonProgressForStatus(a.pythonTask.ProgressPercent, status)
	a.pythonTask.UpdatedAt = time.Now().UnixMilli()
	if err != nil && !errors.Is(err, context.Canceled) {
		a.pythonTask.Error = err.Error()
		a.pythonTask.Detail = err.Error()
	}
	if errors.Is(err, context.Canceled) {
		a.pythonTask.Error = ""
	}
	a.pythonCancel = nil
	a.emitPythonToolchainTaskLocked()
}

func (a *App) emitPythonToolchainTask(task *PythonToolchainTask) {
	if task == nil || a.ctx == nil {
		return
	}
	wailsruntime.EventsEmit(a.ctx, "python:toolchain:task", task)
}

func (a *App) emitPythonToolchainTaskLocked() {
	a.emitPythonToolchainTask(clonePythonToolchainTask(a.pythonTask))
}

func clonePythonToolchainTask(task *PythonToolchainTask) *PythonToolchainTask {
	if task == nil {
		return nil
	}
	copyTask := *task
	return &copyTask
}

func clampPythonProgressForStatus(progress float64, status string) float64 {
	switch status {
	case "completed":
		return 100
	case "canceled", "failed":
		if progress < 0 {
			return 0
		}
		return progress
	default:
		return progress
	}
}
