package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"fire-salamander-desktop/internal/toolchain"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) getGoToolchainTaskState() *GoToolchainTask {
	a.state.Mu.RLock()
	defer a.state.Mu.RUnlock()
	return cloneGoToolchainTask(a.state.GoTask)
}

func (a *App) cancelGoToolchainTask() error {
	a.state.Mu.Lock()
	cancel := a.state.GoCancel
	a.state.Mu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	return nil
}

func (a *App) startInstallGoToolchainTask(req InstallGoToolchainRequest) (*GoToolchainTask, error) {
	a.state.Mu.Lock()
	if a.state.GoTask != nil && a.state.GoTask.Status == "running" {
		task := cloneGoToolchainTask(a.state.GoTask)
		a.state.Mu.Unlock()
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
	a.state.GoTask = task
	a.state.GoCancel = cancel
	copyTask := cloneGoToolchainTask(task)
	a.state.Mu.Unlock()
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
	a.state.Mu.Lock()
	defer a.state.Mu.Unlock()
	if a.state.GoTask == nil {
		return
	}
	update(a.state.GoTask)
	a.state.GoTask.UpdatedAt = time.Now().UnixMilli()
	a.emitGoToolchainTaskLocked()
}

func (a *App) finishGoToolchainTask(status string, message string, err error) {
	a.state.Mu.Lock()
	defer a.state.Mu.Unlock()
	if a.state.GoTask == nil {
		return
	}
	a.state.GoTask.Status = status
	a.state.GoTask.Message = message
	a.state.GoTask.ProgressPercent = clampGoProgressForStatus(a.state.GoTask.ProgressPercent, status)
	a.state.GoTask.TransferSpeed = ""
	a.state.GoTask.UpdatedAt = time.Now().UnixMilli()
	if err != nil && !errors.Is(err, context.Canceled) {
		classifiedMessage, classifiedDetail := toolchain.DescribeGoInstallError(err)
		if strings.TrimSpace(message) == "" || strings.TrimSpace(message) == "Go SDK 下载任务失败" {
			a.state.GoTask.Message = classifiedMessage
		}
		a.state.GoTask.Detail = ""
		a.state.GoTask.Error = classifiedDetail
		if a.state.GoTask.Error == "" && strings.TrimSpace(err.Error()) != strings.TrimSpace(a.state.GoTask.Message) {
			a.state.GoTask.Error = strings.TrimSpace(err.Error())
		}
	}
	if errors.Is(err, context.Canceled) {
		a.state.GoTask.Detail = ""
		a.state.GoTask.Error = ""
	}
	a.state.GoCancel = nil
	a.emitGoToolchainTaskLocked()
}

func (a *App) emitGoToolchainTask(task *GoToolchainTask) {
	if task == nil || a.state.Ctx == nil {
		return
	}
	wailsruntime.EventsEmit(a.state.Ctx, "go:toolchain:task", task)
}

func (a *App) emitGoToolchainTaskLocked() {
	a.emitGoToolchainTask(cloneGoToolchainTask(a.state.GoTask))
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
