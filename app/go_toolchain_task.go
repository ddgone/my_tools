package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"fire-salamander-desktop/internal/toolchain"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type GoToolchainTask struct {
	Kind             string  `json:"kind"`
	Status           string  `json:"status"`
	Message          string  `json:"message"`
	Detail           string  `json:"detail,omitempty"`
	CurrentItem      string  `json:"currentItem,omitempty"`
	ProgressPercent  float64 `json:"progressPercent"`
	Step             int     `json:"step"`
	TotalSteps       int     `json:"totalSteps"`
	Version          string  `json:"version,omitempty"`
	Directory        string  `json:"directory,omitempty"`
	TransferredBytes int64   `json:"transferredBytes,omitempty"`
	TotalBytes       int64   `json:"totalBytes,omitempty"`
	TransferSpeed    string  `json:"transferSpeed,omitempty"`
	Error            string  `json:"error,omitempty"`
	UpdatedAt        int64   `json:"updatedAt"`
}

func (a *App) getGoToolchainTaskState() *GoToolchainTask {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return cloneGoToolchainTask(a.goTask)
}

func (a *App) cancelGoToolchainTask() error {
	a.mu.Lock()
	cancel := a.goCancel
	a.mu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	return nil
}

func (a *App) startInstallGoToolchainTask(req InstallGoToolchainRequest) (*GoToolchainTask, error) {
	a.mu.Lock()
	if a.goTask != nil && a.goTask.Status == "running" {
		task := cloneGoToolchainTask(a.goTask)
		a.mu.Unlock()
		return task, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	task := &GoToolchainTask{
		Kind:            "install",
		Status:          "running",
		Message:         "准备执行 Go SDK 下载任务",
		ProgressPercent: 0,
		Version:         strings.TrimSpace(req.Version),
		Directory:       strings.TrimSpace(req.Directory),
		UpdatedAt:       time.Now().UnixMilli(),
	}
	a.goTask = task
	a.goCancel = cancel
	copyTask := cloneGoToolchainTask(task)
	a.mu.Unlock()
	a.emitGoToolchainTask(copyTask)

	go func() {
		installResult, runErr := toolchain.InstallOfficialReleaseWithOptions(ctx, req.Version, req.Directory, &toolchain.GoInstallHooks{
			OnProgress: func(progress toolchain.GoInstallProgress) {
				a.updateGoToolchainTask(func(task *GoToolchainTask) {
					task.Kind = "install"
					task.Status = "running"
					task.Message = strings.TrimSpace(progress.Message)
					task.Detail = strings.TrimSpace(progress.Detail)
					task.CurrentItem = strings.TrimSpace(progress.CurrentItem)
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
			a.finishGoToolchainTask("canceled", "Go SDK 下载任务已停止", runErr)
			return
		}
		if runErr != nil {
			a.finishGoToolchainTask("failed", "Go SDK 下载任务失败", runErr)
			return
		}

		cfg, cfgErr := toolchain.LoadConfig()
		if cfgErr != nil {
			a.finishGoToolchainTask("failed", "Go SDK 已安装，但读取环境配置失败", cfgErr)
			return
		}
		cfg.SelectedBinary = installResult.BinaryPath
		cfg.KnownBinaries = append(cfg.KnownBinaries, installResult.BinaryPath)
		cfg.LastInstallDirectory = toolchain.NormalizeInstallBaseDirectory(req.Directory)
		cfg.Disabled = false
		if saveErr := toolchain.SaveConfig(cfg); saveErr != nil {
			a.finishGoToolchainTask("failed", "Go SDK 已安装，但保存环境配置失败", saveErr)
			return
		}
		a.finishGoToolchainTask("completed", "Go SDK 下载任务已完成", nil)
	}()

	return copyTask, nil
}

func (a *App) updateGoToolchainTask(update func(task *GoToolchainTask)) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.goTask == nil {
		return
	}
	update(a.goTask)
	a.goTask.UpdatedAt = time.Now().UnixMilli()
	a.emitGoToolchainTaskLocked()
}

func (a *App) finishGoToolchainTask(status string, message string, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.goTask == nil {
		return
	}
	a.goTask.Status = status
	a.goTask.Message = message
	a.goTask.ProgressPercent = clampGoProgressForStatus(a.goTask.ProgressPercent, status)
	a.goTask.TransferSpeed = ""
	a.goTask.UpdatedAt = time.Now().UnixMilli()
	if err != nil && !errors.Is(err, context.Canceled) {
		classifiedMessage, classifiedDetail := toolchain.DescribeGoInstallError(err)
		if strings.TrimSpace(message) == "" || strings.TrimSpace(message) == "Go SDK 下载任务失败" {
			a.goTask.Message = classifiedMessage
		}
		a.goTask.Detail = ""
		a.goTask.Error = classifiedDetail
		if a.goTask.Error == "" && strings.TrimSpace(err.Error()) != strings.TrimSpace(a.goTask.Message) {
			a.goTask.Error = strings.TrimSpace(err.Error())
		}
	}
	if errors.Is(err, context.Canceled) {
		a.goTask.Detail = ""
		a.goTask.Error = ""
	}
	a.goCancel = nil
	a.emitGoToolchainTaskLocked()
}

func (a *App) emitGoToolchainTask(task *GoToolchainTask) {
	if task == nil || a.ctx == nil {
		return
	}
	wailsruntime.EventsEmit(a.ctx, "go:toolchain:task", task)
}

func (a *App) emitGoToolchainTaskLocked() {
	a.emitGoToolchainTask(cloneGoToolchainTask(a.goTask))
}

func cloneGoToolchainTask(task *GoToolchainTask) *GoToolchainTask {
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
