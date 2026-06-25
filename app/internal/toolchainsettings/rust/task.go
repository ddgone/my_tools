package rustsettings

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"fire-salamander-desktop/internal/shared"
	"fire-salamander-desktop/internal/toolchain"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (m *Manager) getRustToolchainTaskState() *shared.RustToolchainTask {
	m.state.Mu.RLock()
	defer m.state.Mu.RUnlock()
	return cloneRustToolchainTask(m.state.RustTask)
}

func (m *Manager) cancelRustToolchainTask() error {
	m.state.Mu.Lock()
	cancel := m.state.RustCancel
	m.state.Mu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	return nil
}

func (m *Manager) startInstallRustToolchainTask(req InstallRustToolchainRequest) (*shared.RustToolchainTask, error) {
	m.state.Mu.Lock()
	if m.state.RustTask != nil && m.state.RustTask.Status == "running" {
		task := cloneRustToolchainTask(m.state.RustTask)
		m.state.Mu.Unlock()
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
	task := &shared.RustToolchainTask{
		Kind:            kind,
		Status:          "running",
		Message:         "准备执行 Rust 交叉编译环境安装任务",
		ProgressPercent: 0,
		RustVersion:     strings.TrimSpace(req.RustVersion),
		ZigVersion:      strings.TrimSpace(req.ZigVersion),
		Directory:       strings.TrimSpace(req.Directory),
		UpdatedAt:       time.Now().UnixMilli(),
	}
	m.state.RustTask = task
	m.state.RustCancel = cancel
	copyTask := cloneRustToolchainTask(task)
	m.state.Mu.Unlock()
	m.emitRustToolchainTask(copyTask)

	go func() {
		installKind := kind
		installResult, runErr := toolchain.InstallManagedRustEnvironmentWithOptions(
			ctx,
			req.RustVersion,
			req.ZigVersion,
			req.Directory,
			&toolchain.RustInstallHooks{
				OnProgress: func(progress toolchain.RustInstallProgress) {
					m.updateRustToolchainTask(func(task *shared.RustToolchainTask) {
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
			m.finishRustToolchainTask("canceled", "Rust 交叉编译环境安装任务已停止", runErr)
			return
		}
		if runErr != nil {
			m.finishRustToolchainTask("failed", "Rust 交叉编译环境安装任务失败", runErr)
			return
		}

		cfg, cfgErr := toolchain.LoadRustConfig()
		if cfgErr != nil {
			m.finishRustToolchainTask("failed", "Rust 环境已安装，但读取环境配置失败", cfgErr)
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
			m.finishRustToolchainTask("failed", "Rust 环境已安装，但保存环境配置失败", saveErr)
			return
		}
		m.finishRustToolchainTask("completed", "Rust 交叉编译环境安装任务已完成", nil)
	}()

	return copyTask, nil
}

func (m *Manager) startInstallRustCapabilityTask(kind string) (*shared.RustToolchainTask, error) {
	m.state.Mu.Lock()
	if m.state.RustTask != nil && m.state.RustTask.Status == "running" {
		task := cloneRustToolchainTask(m.state.RustTask)
		m.state.Mu.Unlock()
		return task, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	task := &shared.RustToolchainTask{
		Kind:            kind,
		Status:          "running",
		Message:         "准备补齐 Rust 环境能力",
		ProgressPercent: 0,
		UpdatedAt:       time.Now().UnixMilli(),
	}
	m.state.RustTask = task
	m.state.RustCancel = cancel
	copyTask := cloneRustToolchainTask(task)
	m.state.Mu.Unlock()
	m.emitRustToolchainTask(copyTask)

	go func() {
		var runErr error
		switch kind {
		case "cargo-zigbuild":
			runErr = toolchain.InstallCargoZigbuildForActiveRust(ctx, &toolchain.RustInstallHooks{
				OnProgress: func(progress toolchain.RustInstallProgress) {
					m.updateRustToolchainTask(func(task *shared.RustToolchainTask) {
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
					m.updateRustToolchainTask(func(task *shared.RustToolchainTask) {
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
			m.finishRustToolchainTask("canceled", "Rust 补齐任务已停止", runErr)
			return
		}
		if runErr != nil {
			m.finishRustToolchainTask("failed", "Rust 补齐任务失败", runErr)
			return
		}
		m.finishRustToolchainTask("completed", "Rust 补齐任务已完成", nil)
	}()

	return copyTask, nil
}

func (m *Manager) updateRustToolchainTask(update func(task *shared.RustToolchainTask)) {
	m.state.Mu.Lock()
	defer m.state.Mu.Unlock()
	if m.state.RustTask == nil {
		return
	}
	update(m.state.RustTask)
	m.state.RustTask.UpdatedAt = time.Now().UnixMilli()
	m.emitRustToolchainTaskLocked()
}

func (m *Manager) finishRustToolchainTask(status string, message string, err error) {
	m.state.Mu.Lock()
	defer m.state.Mu.Unlock()
	if m.state.RustTask == nil {
		return
	}
	m.state.RustTask.Status = status
	m.state.RustTask.Message = message
	m.state.RustTask.ProgressPercent = clampRustProgressForStatus(m.state.RustTask.ProgressPercent, status)
	m.state.RustTask.TransferSpeed = ""
	m.state.RustTask.UpdatedAt = time.Now().UnixMilli()
	if err != nil && !errors.Is(err, context.Canceled) {
		classifiedMessage, classifiedDetail := toolchain.DescribeRustInstallError(err)
		if strings.TrimSpace(message) == "" || strings.TrimSpace(message) == "Rust 交叉编译环境安装任务失败" {
			m.state.RustTask.Message = classifiedMessage
		}
		m.state.RustTask.Detail = ""
		m.state.RustTask.Error = classifiedDetail
		if m.state.RustTask.Error == "" && strings.TrimSpace(err.Error()) != strings.TrimSpace(m.state.RustTask.Message) {
			m.state.RustTask.Error = strings.TrimSpace(err.Error())
		}
	}
	if errors.Is(err, context.Canceled) {
		m.state.RustTask.Detail = ""
		m.state.RustTask.Error = ""
	}
	m.state.RustCancel = nil
	m.emitRustToolchainTaskLocked()
}

func (m *Manager) emitRustToolchainTask(task *shared.RustToolchainTask) {
	if task == nil || m.state.Ctx == nil {
		return
	}
	wailsruntime.EventsEmit(m.state.Ctx, "rust:toolchain:task", task)
}

func (m *Manager) emitRustToolchainTaskLocked() {
	m.emitRustToolchainTask(cloneRustToolchainTask(m.state.RustTask))
}

func cloneRustToolchainTask(task *shared.RustToolchainTask) *shared.RustToolchainTask {
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
