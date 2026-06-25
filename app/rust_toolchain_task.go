package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"fire-salamander-desktop/internal/toolchain"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type RustToolchainTask struct {
	Kind             string  `json:"kind"`
	Status           string  `json:"status"`
	Message          string  `json:"message"`
	Detail           string  `json:"detail,omitempty"`
	CurrentItem      string  `json:"currentItem,omitempty"`
	CurrentSource    string  `json:"currentSource,omitempty"`
	ProgressPercent  float64 `json:"progressPercent"`
	Step             int     `json:"step"`
	TotalSteps       int     `json:"totalSteps"`
	RustVersion      string  `json:"rustVersion,omitempty"`
	ZigVersion       string  `json:"zigVersion,omitempty"`
	Directory        string  `json:"directory,omitempty"`
	TransferredBytes int64   `json:"transferredBytes,omitempty"`
	TotalBytes       int64   `json:"totalBytes,omitempty"`
	TransferSpeed    string  `json:"transferSpeed,omitempty"`
	Error            string  `json:"error,omitempty"`
	UpdatedAt        int64   `json:"updatedAt"`
}

func (a *App) getRustToolchainTaskState() *RustToolchainTask {
	a.state.Mu.RLock()
	defer a.state.Mu.RUnlock()
	return cloneRustToolchainTask(a.state.RustTask)
}

func (a *App) cancelRustToolchainTask() error {
	a.state.Mu.Lock()
	cancel := a.state.RustCancel
	a.state.Mu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	return nil
}

func (a *App) startInstallRustToolchainTask(req InstallRustToolchainRequest) (*RustToolchainTask, error) {
	a.state.Mu.Lock()
	if a.state.RustTask != nil && a.state.RustTask.Status == "running" {
		task := cloneRustToolchainTask(a.state.RustTask)
		a.state.Mu.Unlock()
		return task, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	kind := "install"
	switch {
	case strings.TrimSpace(req.RustVersion) != "" && strings.TrimSpace(req.ZigVersion) == "":
		kind = "install-rust"
	case strings.TrimSpace(req.RustVersion) == "" && strings.TrimSpace(req.ZigVersion) != "":
		kind = "install-zig"
	}
	task := &RustToolchainTask{
		Kind:            kind,
		Status:          "running",
		Message:         "准备执行 Rust 交叉编译环境安装任务",
		ProgressPercent: 0,
		RustVersion:     strings.TrimSpace(req.RustVersion),
		ZigVersion:      strings.TrimSpace(req.ZigVersion),
		Directory:       strings.TrimSpace(req.Directory),
		UpdatedAt:       time.Now().UnixMilli(),
	}
	a.state.RustTask = task
	a.state.RustCancel = cancel
	copyTask := cloneRustToolchainTask(task)
	a.state.Mu.Unlock()
	a.emitRustToolchainTask(copyTask)

	go func() {
		installKind := kind
		installResult, runErr := toolchain.InstallManagedRustEnvironmentWithOptions(
			ctx,
			req.RustVersion,
			req.ZigVersion,
			req.Directory,
			&toolchain.RustInstallHooks{
				OnProgress: func(progress toolchain.RustInstallProgress) {
					a.updateRustToolchainTask(func(task *RustToolchainTask) {
						task.Kind = installKind
						task.Status = "running"
						task.Message = strings.TrimSpace(progress.Message)
						task.Detail = strings.TrimSpace(progress.Detail)
						task.CurrentItem = strings.TrimSpace(progress.CurrentItem)
						task.CurrentSource = strings.TrimSpace(progress.CurrentSource)
						task.ProgressPercent = progress.ProgressPercent
						task.Step = progress.Step
						task.TotalSteps = progress.TotalSteps
						task.RustVersion = strings.TrimSpace(progress.RustVersion)
						task.ZigVersion = strings.TrimSpace(progress.ZigVersion)
						task.Directory = strings.TrimSpace(progress.Directory)
						task.TransferredBytes = progress.TransferredBytes
						task.TotalBytes = progress.TotalBytes
						task.TransferSpeed = strings.TrimSpace(progress.TransferSpeed)
						task.Error = ""
					})
				},
			},
		)
		if errors.Is(runErr, context.Canceled) {
			a.finishRustToolchainTask("canceled", "Rust 交叉编译环境安装任务已停止", runErr)
			return
		}
		if runErr != nil {
			a.finishRustToolchainTask("failed", "Rust 交叉编译环境安装任务失败", runErr)
			return
		}

		cfg, cfgErr := toolchain.LoadRustConfig()
		if cfgErr != nil {
			a.finishRustToolchainTask("failed", "Rust 环境已安装，但读取环境配置失败", cfgErr)
			return
		}
		if strings.TrimSpace(installResult.RustDirectory) != "" {
			cfg.SelectedRustRoot = installResult.RustDirectory
			cfg.KnownRustRoots = append(cfg.KnownRustRoots, installResult.RustDirectory)
			cfg.Mode = "manual"
		}
		if strings.TrimSpace(installResult.ZigBinary) != "" {
			cfg.SelectedZigBinary = installResult.ZigBinary
			cfg.KnownZigBinaries = append(cfg.KnownZigBinaries, installResult.ZigBinary)
			cfg.Mode = "manual"
		}
		cfg.LastInstallDirectory = toolchain.NormalizeRustInstallBaseDirectory(req.Directory)
		cfg.Disabled = false
		if saveErr := toolchain.SaveRustConfig(cfg); saveErr != nil {
			a.finishRustToolchainTask("failed", "Rust 环境已安装，但保存环境配置失败", saveErr)
			return
		}
		a.finishRustToolchainTask("completed", "Rust 交叉编译环境安装任务已完成", nil)
	}()

	return copyTask, nil
}

func (a *App) startInstallRustCapabilityTask(kind string) (*RustToolchainTask, error) {
	a.state.Mu.Lock()
	if a.state.RustTask != nil && a.state.RustTask.Status == "running" {
		task := cloneRustToolchainTask(a.state.RustTask)
		a.state.Mu.Unlock()
		return task, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	task := &RustToolchainTask{
		Kind:            kind,
		Status:          "running",
		Message:         "准备补齐 Rust 环境能力",
		ProgressPercent: 0,
		UpdatedAt:       time.Now().UnixMilli(),
	}
	a.state.RustTask = task
	a.state.RustCancel = cancel
	copyTask := cloneRustToolchainTask(task)
	a.state.Mu.Unlock()
	a.emitRustToolchainTask(copyTask)

	go func() {
		var runErr error
		switch kind {
		case "cargo-zigbuild":
			runErr = toolchain.InstallCargoZigbuildForActiveRust(ctx, &toolchain.RustInstallHooks{
				OnProgress: func(progress toolchain.RustInstallProgress) {
					a.updateRustToolchainTask(func(task *RustToolchainTask) {
						task.Kind = kind
						task.Status = "running"
						task.Message = strings.TrimSpace(progress.Message)
						task.Detail = strings.TrimSpace(progress.Detail)
						task.CurrentItem = strings.TrimSpace(progress.CurrentItem)
						task.CurrentSource = strings.TrimSpace(progress.CurrentSource)
						task.ProgressPercent = progress.ProgressPercent
						task.Step = progress.Step
						task.TotalSteps = progress.TotalSteps
						task.Directory = strings.TrimSpace(progress.Directory)
						task.Error = ""
					})
				},
			})
		case "targets":
			runErr = toolchain.InstallTargetsForActiveRust(ctx, &toolchain.RustInstallHooks{
				OnProgress: func(progress toolchain.RustInstallProgress) {
					a.updateRustToolchainTask(func(task *RustToolchainTask) {
						task.Kind = kind
						task.Status = "running"
						task.Message = strings.TrimSpace(progress.Message)
						task.Detail = strings.TrimSpace(progress.Detail)
						task.CurrentItem = strings.TrimSpace(progress.CurrentItem)
						task.CurrentSource = strings.TrimSpace(progress.CurrentSource)
						task.ProgressPercent = progress.ProgressPercent
						task.Step = progress.Step
						task.TotalSteps = progress.TotalSteps
						task.Directory = strings.TrimSpace(progress.Directory)
						task.Error = ""
					})
				},
			})
		default:
			runErr = fmt.Errorf("不支持的 Rust 补齐任务: %s", kind)
		}
		if errors.Is(runErr, context.Canceled) {
			a.finishRustToolchainTask("canceled", "Rust 补齐任务已停止", runErr)
			return
		}
		if runErr != nil {
			a.finishRustToolchainTask("failed", "Rust 补齐任务失败", runErr)
			return
		}
		a.finishRustToolchainTask("completed", "Rust 补齐任务已完成", nil)
	}()

	return copyTask, nil
}

func (a *App) updateRustToolchainTask(update func(task *RustToolchainTask)) {
	a.state.Mu.Lock()
	defer a.state.Mu.Unlock()
	if a.state.RustTask == nil {
		return
	}
	update(a.state.RustTask)
	a.state.RustTask.UpdatedAt = time.Now().UnixMilli()
	a.emitRustToolchainTaskLocked()
}

func (a *App) finishRustToolchainTask(status string, message string, err error) {
	a.state.Mu.Lock()
	defer a.state.Mu.Unlock()
	if a.state.RustTask == nil {
		return
	}
	a.state.RustTask.Status = status
	a.state.RustTask.Message = message
	a.state.RustTask.ProgressPercent = clampRustProgressForStatus(a.state.RustTask.ProgressPercent, status)
	a.state.RustTask.TransferSpeed = ""
	a.state.RustTask.UpdatedAt = time.Now().UnixMilli()
	if err != nil && !errors.Is(err, context.Canceled) {
		classifiedMessage, classifiedDetail := toolchain.DescribeRustInstallError(err)
		if strings.TrimSpace(message) == "" || strings.TrimSpace(message) == "Rust 交叉编译环境安装任务失败" {
			a.state.RustTask.Message = classifiedMessage
		}
		a.state.RustTask.Detail = ""
		a.state.RustTask.Error = classifiedDetail
		if a.state.RustTask.Error == "" && strings.TrimSpace(err.Error()) != strings.TrimSpace(a.state.RustTask.Message) {
			a.state.RustTask.Error = strings.TrimSpace(err.Error())
		}
	}
	if errors.Is(err, context.Canceled) {
		a.state.RustTask.Detail = ""
		a.state.RustTask.Error = ""
	}
	a.state.RustCancel = nil
	a.emitRustToolchainTaskLocked()
}

func (a *App) emitRustToolchainTask(task *RustToolchainTask) {
	if task == nil || a.state.Ctx == nil {
		return
	}
	wailsruntime.EventsEmit(a.state.Ctx, "rust:toolchain:task", task)
}

func (a *App) emitRustToolchainTaskLocked() {
	a.emitRustToolchainTask(cloneRustToolchainTask(a.state.RustTask))
}

func cloneRustToolchainTask(task *RustToolchainTask) *RustToolchainTask {
	if task == nil {
		return nil
	}
	copyTask := *task
	return &copyTask
}

func clampRustProgressForStatus(progress float64, status string) float64 {
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
