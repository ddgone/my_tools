package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"fire-salamander-desktop/internal/runtimeenv"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type WindowManager struct {
	state *SharedState
}

func NewWindowManager(state *SharedState) *WindowManager {
	return &WindowManager{state: state}
}

// ------- Wails lifecycle callbacks -------

func (m *WindowManager) DomReady(ctx context.Context) {
	m.state.Ctx = ctx
	m.restoreSavedWindowState()
}

func (m *WindowManager) BeforeClose(ctx context.Context) bool {
	m.state.Ctx = ctx
	if err := m.persistCurrentWindowState(); err != nil {
		log.Printf("persistCurrentWindowState failed: %v", err)
	}
	return false
}

// ------- public API (via App delegation) -------

func (m *WindowManager) GetWindowConfig() WindowState {
	return m.loadWindowConfig()
}

func (m *WindowManager) SaveWindowState(state WindowState) error {
	return m.writeWindowConfig(state)
}

func (m *WindowManager) GetCurrentWindowState() (WindowState, error) {
	return m.currentWindowState()
}

func (m *WindowManager) PersistCurrentWindowState() error {
	return m.persistCurrentWindowState()
}

func (m *WindowManager) IsWindowRectVisible(x, y, width, height int) bool {
	return isWindowRectVisible(x, y, width, height)
}

// ------- internal -------

func (m *WindowManager) currentWindowState() (WindowState, error) {
	snapshot, err := m.currentWindowSnapshot()
	if err != nil {
		return WindowState{}, err
	}
	return snapshot.State, nil
}

func (m *WindowManager) currentWindowSnapshot() (windowSnapshot, error) {
	if m.state.Ctx == nil {
		return windowSnapshot{}, fmt.Errorf("应用尚未初始化")
	}

	// 优先使用 Wails API 获取逻辑像素，避免与 Win32 物理像素混用导致 DPI 缩放问题。
	width, height := wailsruntime.WindowGetSize(m.state.Ctx)
	x, y := wailsruntime.WindowGetPosition(m.state.Ctx)
	frame := windowFrame{X: x, Y: y, Width: width, Height: height}
	minimised := false

	if !frame.valid() {
		// Wails API 返回无效值时，回退到 Win32 原生接口。
		nativeFrame, nativeMinimised, err := nativeWindowFrame(m.state.Ctx)
		if err == nil && nativeFrame.valid() {
			frame = nativeFrame
			minimised = nativeMinimised
		}
	}

	fullscreen := wailsruntime.WindowIsFullscreen(m.state.Ctx)
	maximised := false
	if !fullscreen {
		maximised = wailsruntime.WindowIsMaximised(m.state.Ctx)
	}

	return windowSnapshot{
		State: WindowState{
			Width:      frame.Width,
			Height:     frame.Height,
			X:          frame.X,
			Y:          frame.Y,
			Maximised:  maximised,
			Fullscreen: fullscreen,
		},
		Minimised: minimised,
	}, nil
}

func (m *WindowManager) persistCurrentWindowState() error {
	current, err := m.currentWindowSnapshot()
	if err != nil {
		return err
	}
	if current.Minimised {
		return nil
	}

	next := m.loadWindowConfig()
	if !current.State.Maximised && !current.State.Fullscreen && current.State.Width > 0 && current.State.Height > 0 {
		next.Width = current.State.Width
		next.Height = current.State.Height
		next.X = current.State.X
		next.Y = current.State.Y
	}

	next.Maximised = current.State.Maximised
	next.Fullscreen = current.State.Fullscreen
	return m.writeWindowConfig(next)
}

func (m *WindowManager) restoreSavedWindowState() {
	if m.state.Ctx == nil {
		return
	}

	saved := m.loadWindowConfig()
	frame := frameFromState(saved)
	if !frame.valid() {
		frame.Width = defaultWindowWidth
		frame.Height = defaultWindowHeight
	}

	if shouldRestoreSavedFrame(saved, frame) {
		// 优先使用 Wails API 设置逻辑像素位置。
		wailsruntime.WindowSetSize(m.state.Ctx, frame.Width, frame.Height)
		wailsruntime.WindowSetPosition(m.state.Ctx, frame.X, frame.Y)
		// 再尝试 Win32 原生 API 精确调整（失败忽略，Wails 已设置）。
		_ = nativeSetWindowFrame(m.state.Ctx, frame)
	} else {
		wailsruntime.WindowSetSize(m.state.Ctx, frame.Width, frame.Height)
		wailsruntime.WindowCenter(m.state.Ctx)
	}

	if saved.Fullscreen {
		wailsruntime.WindowFullscreen(m.state.Ctx)
		return
	}
	if saved.Maximised {
		wailsruntime.WindowMaximise(m.state.Ctx)
	}
}

func (m *WindowManager) loadWindowConfig() WindowState {
	layout, err := runtimeenv.ResolveLayout()
	if err != nil {
		return WindowState{Width: 0, Height: 0, X: -1, Y: -1}
	}
	data, err := os.ReadFile(filepath.Join(layout.ConfigDir(), "app.json"))
	if err != nil {
		return WindowState{Width: 0, Height: 0, X: -1, Y: -1}
	}
	var cfg struct {
		Window WindowState `json:"window"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return WindowState{Width: 0, Height: 0, X: -1, Y: -1}
	}
	return cfg.Window
}

func (m *WindowManager) writeWindowConfig(state WindowState) error {
	layout, err := runtimeenv.ResolveLayout()
	if err != nil {
		return fmt.Errorf("解析运行时目录失败: %w", err)
	}
	if err := os.MkdirAll(layout.ConfigDir(), 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}
	configPath := filepath.Join(layout.ConfigDir(), "app.json")

	cfg, err := loadConfigDocument(configPath)
	if err != nil {
		return err
	}

	windowData, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("序列化窗口状态失败: %w", err)
	}
	cfg["window"] = windowData

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("格式化配置文件失败: %w", err)
	}

	if err := os.WriteFile(configPath, out, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}
	return nil
}
