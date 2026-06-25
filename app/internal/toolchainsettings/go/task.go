package gosettings

import (
	"context"
	"errors"
	"strings"
	"time"

	"fire-salamander-desktop/internal/shared"
	"fire-salamander-desktop/internal/toolchain"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (m *Manager) getGoToolchainTaskState() *shared.GoToolchainTask {
	m.state.Mu.RLock()
	defer m.state.Mu.RUnlock()
	return cloneGoToolchainTask(m.state.GoTask)
}

func (m *Manager) cancelGoToolchainTask() error {
	m.state.Mu.Lock()
	cancel := m.state.GoCancel
	m.state.Mu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	return nil
}

func (m *Manager) startInstallGoToolchainTask(req InstallGoToolchainRequest) (*shared.GoToolchainTask, error) {
	m.state.Mu.Lock()
	if m.state.GoTask != nil && m.state.GoTask.Status == "running" {
		task := cloneGoToolchainTask(m.state.GoTask)
		m.state.Mu.Unlock()
		return task, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	task := &shared.GoToolchainTask{
		Kind:            "install",
		Status:          "running",
		Message:         "准备执行 Go SDK 下载任务",
		ProgressPercent: 0,
		Version:         strings.TrimSpace(req.Version),
		Directory:       strings.TrimSpace(req.Directory),
		UpdatedAt:       time.Now().UnixMilli(),
	}
	m.state.GoTask = task
	m.state.GoCancel = cancel
	copyTask := cloneGoToolchainTask(task)
	m.state.Mu.Unlock()
	m.emitGoToolchainTask(copyTask)

	go func() {
		installResult, runErr := toolchain.InstallOfficialReleaseWithOptions(ctx, req.Version, req.Directory, &toolchain.GoInstallHooks{
			OnProgress: func(progress toolchain.GoInstallProgress) {
				m.updateGoToolchainTask(func(task *shared.GoToolchainTask) {
					task.Kind = "install"
					task.Status = "running"
					task.Message = strings.TrimSpace(progress.Message)
					task.Detail = strings.TrimSpace(progress.Detail)
					task.CurrentItem = strings.TrimSpace(progress.CurrentItem)
					task.CurrentSource = strings.TrimSpace(progress.CurrentSource)
					task.ProgressPercent = progress.ProgressPercent
					task.Step = progress.Step
					task.TotalSteps = progress.TotalSteps
					task.Version = strings.TrimSpace(progress.Version)
					task.Directory = strings.TrimSpace(progress.Directory)
					task.TransferredBytes = progress.TransferredBytes
					task.TotalBytes = progress.TotalBytes
					task.TransferSpeed = strings.TrimSpace(progress.TransferSpeed)
					task.Error = ""
				})
			},
		})
		if errors.Is(runErr, context.Canceled) {
			m.finishGoToolchainTask("canceled", "Go SDK 下载任务已停止", runErr)
			return
		}
		if runErr != nil {
			m.finishGoToolchainTask("failed", "Go SDK 下载任务失败", runErr)
			return
		}

		cfg, cfgErr := toolchain.LoadConfig()
		if cfgErr != nil {
			m.finishGoToolchainTask("failed", "Go SDK 已安装，但读取环境配置失败", cfgErr)
			return
		}
		cfg.SelectedBinary = installResult.BinaryPath
		cfg.KnownBinaries = append(cfg.KnownBinaries, installResult.BinaryPath)
		cfg.LastInstallDirectory = toolchain.NormalizeInstallBaseDirectory(req.Directory)
		cfg.Disabled = false
		if saveErr := toolchain.SaveConfig(cfg); saveErr != nil {
			m.finishGoToolchainTask("failed", "Go SDK 已安装，但保存环境配置失败", saveErr)
			return
		}
		m.finishGoToolchainTask("completed", "Go SDK 下载任务已完成", nil)
	}()

	return copyTask, nil
}

func (m *Manager) updateGoToolchainTask(update func(task *shared.GoToolchainTask)) {
	m.state.Mu.Lock()
	defer m.state.Mu.Unlock()
	if m.state.GoTask == nil {
		return
	}
	update(m.state.GoTask)
	m.state.GoTask.UpdatedAt = time.Now().UnixMilli()
	m.emitGoToolchainTaskLocked()
}

func (m *Manager) finishGoToolchainTask(status string, message string, err error) {
	m.state.Mu.Lock()
	defer m.state.Mu.Unlock()
	if m.state.GoTask == nil {
		return
	}
	m.state.GoTask.Status = status
	m.state.GoTask.Message = message
	m.state.GoTask.ProgressPercent = clampGoProgressForStatus(m.state.GoTask.ProgressPercent, status)
	m.state.GoTask.TransferSpeed = ""
	m.state.GoTask.UpdatedAt = time.Now().UnixMilli()
	if err != nil && !errors.Is(err, context.Canceled) {
		classifiedMessage, classifiedDetail := toolchain.DescribeGoInstallError(err)
		if strings.TrimSpace(message) == "" || strings.TrimSpace(message) == "Go SDK 下载任务失败" {
			m.state.GoTask.Message = classifiedMessage
		}
		m.state.GoTask.Detail = ""
		m.state.GoTask.Error = classifiedDetail
		if m.state.GoTask.Error == "" && strings.TrimSpace(err.Error()) != strings.TrimSpace(m.state.GoTask.Message) {
			m.state.GoTask.Error = strings.TrimSpace(err.Error())
		}
	}
	if errors.Is(err, context.Canceled) {
		m.state.GoTask.Detail = ""
		m.state.GoTask.Error = ""
	}
	m.state.GoCancel = nil
	m.emitGoToolchainTaskLocked()
}

func (m *Manager) emitGoToolchainTask(task *shared.GoToolchainTask) {
	if task == nil || m.state.Ctx == nil {
		return
	}
	wailsruntime.EventsEmit(m.state.Ctx, "go:toolchain:task", task)
}

func (m *Manager) emitGoToolchainTaskLocked() {
	m.emitGoToolchainTask(cloneGoToolchainTask(m.state.GoTask))
}

func cloneGoToolchainTask(task *shared.GoToolchainTask) *shared.GoToolchainTask {
	if task == nil {
		return nil
	}
	copyTask := *task
	return &copyTask
}

func clampGoProgressForStatus(progress float64, status string) float64 {
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
