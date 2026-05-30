package main

import (
	"context"
	"fmt"
	"log"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	defaultWindowWidth  = 1280
	defaultWindowHeight = 800
	minVisibleWidth     = 80
	minVisibleHeight    = 60
)

type windowFrame struct {
	X      int
	Y      int
	Width  int
	Height int
}

type windowSnapshot struct {
	State     WindowState
	Minimised bool
}

func (f windowFrame) valid() bool {
	return f.Width > 0 && f.Height > 0
}

func frameFromState(state WindowState) windowFrame {
	return windowFrame{
		X:      state.X,
		Y:      state.Y,
		Width:  state.Width,
		Height: state.Height,
	}
}

func (a *App) domReady(ctx context.Context) {
	a.ctx = ctx
	a.restoreSavedWindowState()
}

func (a *App) beforeClose(ctx context.Context) bool {
	a.ctx = ctx
	if err := a.persistCurrentWindowState(); err != nil {
		log.Printf("persistCurrentWindowState failed: %v", err)
	}
	return false
}

func (a *App) currentWindowState() (WindowState, error) {
	snapshot, err := a.currentWindowSnapshot()
	if err != nil {
		return WindowState{}, err
	}
	return snapshot.State, nil
}

func (a *App) currentWindowSnapshot() (windowSnapshot, error) {
	if a.ctx == nil {
		return windowSnapshot{}, fmt.Errorf("应用尚未初始化")
	}

	frame, minimised, err := nativeWindowFrame(a.ctx)
	if err != nil {
		width, height := wailsruntime.WindowGetSize(a.ctx)
		x, y := wailsruntime.WindowGetPosition(a.ctx)
		frame = windowFrame{X: x, Y: y, Width: width, Height: height}
	}

	fullscreen := wailsruntime.WindowIsFullscreen(a.ctx)
	maximised := false
	if !fullscreen {
		maximised = wailsruntime.WindowIsMaximised(a.ctx)
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

func (a *App) persistCurrentWindowState() error {
	current, err := a.currentWindowSnapshot()
	if err != nil {
		return err
	}
	if current.Minimised {
		return nil
	}

	next := a.loadWindowConfig()
	if !current.State.Maximised && !current.State.Fullscreen && current.State.Width > 0 && current.State.Height > 0 {
		next.Width = current.State.Width
		next.Height = current.State.Height
		next.X = current.State.X
		next.Y = current.State.Y
	}

	next.Maximised = current.State.Maximised
	next.Fullscreen = current.State.Fullscreen
	return a.writeWindowConfig(next)
}

func (a *App) restoreSavedWindowState() {
	if a.ctx == nil {
		return
	}

	saved := a.loadWindowConfig()
	frame := frameFromState(saved)
	if !frame.valid() {
		frame.Width = defaultWindowWidth
		frame.Height = defaultWindowHeight
	}

	if frame.valid() && isWindowRectVisible(frame.X, frame.Y, frame.Width, frame.Height) {
		if err := nativeSetWindowFrame(a.ctx, frame); err != nil {
			wailsruntime.WindowSetSize(a.ctx, frame.Width, frame.Height)
			wailsruntime.WindowSetPosition(a.ctx, frame.X, frame.Y)
		}
	} else {
		wailsruntime.WindowSetSize(a.ctx, frame.Width, frame.Height)
		wailsruntime.WindowCenter(a.ctx)
	}

	if saved.Fullscreen {
		wailsruntime.WindowFullscreen(a.ctx)
		return
	}
	if saved.Maximised {
		wailsruntime.WindowMaximise(a.ctx)
	}
}
