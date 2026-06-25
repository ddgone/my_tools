package pythonsettings

import (
	"context"
	"errors"
	"strings"
	"time"

	"fire-salamander-desktop/internal/shared"
	"fire-salamander-desktop/internal/toolchain"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (m *Manager) getPythonToolchainTaskState() *shared.PythonToolchainTask {
	m.state.Mu.RLock()
	defer m.state.Mu.RUnlock()
	return clonePythonToolchainTask(m.state.PythonTask)
}

func (m *Manager) startPreparePythonToolchainTask() (*shared.PythonToolchainTask, error) {
	return m.startPythonToolchainTask(toolchain.PythonOperationPrepare)
}

func (m *Manager) startInstallPythonDependenciesTask() (*shared.PythonToolchainTask, error) {
	return m.startPythonToolchainTask(toolchain.PythonOperationInstall)
}

func (m *Manager) cancelPythonToolchainTask() error {
	m.state.Mu.Lock()
	cancel := m.state.PythonCancel
	m.state.Mu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	return nil
}

func (m *Manager) startPythonToolchainTask(kind toolchain.PythonOperationKind) (*shared.PythonToolchainTask, error) {
	m.state.Mu.Lock()
	if m.state.PythonTask != nil && m.state.PythonTask.Status == "running" {
		task := clonePythonToolchainTask(m.state.PythonTask)
		m.state.Mu.Unlock()
		return task, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	task := &shared.PythonToolchainTask{
		Kind:            string(kind),
		Status:          "running",
		Message:         "准备执行 Python 环境任务",
		ProgressPercent: 0,
		UpdatedAt:       time.Now().UnixMilli(),
	}
	m.state.PythonTask = task
	m.state.PythonCancel = cancel
	copyTask := clonePythonToolchainTask(task)
	m.state.Mu.Unlock()
	m.emitPythonToolchainTask(copyTask)

	go func() {
		var runErr error
		hooks := &toolchain.PythonOperationHooks{
			OnProgress: func(progress toolchain.PythonOperationProgress) {
				m.updatePythonToolchainTask(func(task *shared.PythonToolchainTask) {
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
			m.finishPythonToolchainTask("canceled", "Python 环境任务已停止", runErr)
			return
		}
		if runErr != nil {
			m.finishPythonToolchainTask("failed", "Python 环境任务失败", runErr)
			return
		}
		m.finishPythonToolchainTask("completed", "Python 环境任务已完成", nil)
	}()

	return copyTask, nil
}

func (m *Manager) updatePythonToolchainTask(update func(task *shared.PythonToolchainTask)) {
	m.state.Mu.Lock()
	defer m.state.Mu.Unlock()
	if m.state.PythonTask == nil {
		return
	}
	update(m.state.PythonTask)
	m.state.PythonTask.UpdatedAt = time.Now().UnixMilli()
	m.emitPythonToolchainTaskLocked()
}

func (m *Manager) finishPythonToolchainTask(status string, message string, err error) {
	m.state.Mu.Lock()
	defer m.state.Mu.Unlock()
	if m.state.PythonTask == nil {
		return
	}
	m.state.PythonTask.Status = status
	m.state.PythonTask.Message = message
	m.state.PythonTask.ProgressPercent = clampPythonProgressForStatus(m.state.PythonTask.ProgressPercent, status)
	m.state.PythonTask.UpdatedAt = time.Now().UnixMilli()
	if err != nil && !errors.Is(err, context.Canceled) {
		m.state.PythonTask.Error = err.Error()
		m.state.PythonTask.Detail = err.Error()
	}
	if errors.Is(err, context.Canceled) {
		m.state.PythonTask.Error = ""
	}
	m.state.PythonCancel = nil
	m.emitPythonToolchainTaskLocked()
}

func (m *Manager) emitPythonToolchainTask(task *shared.PythonToolchainTask) {
	if task == nil || m.state.Ctx == nil {
		return
	}
	wailsruntime.EventsEmit(m.state.Ctx, "python:toolchain:task", task)
}

func (m *Manager) emitPythonToolchainTaskLocked() {
	m.emitPythonToolchainTask(clonePythonToolchainTask(m.state.PythonTask))
}

func clonePythonToolchainTask(task *shared.PythonToolchainTask) *shared.PythonToolchainTask {
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
