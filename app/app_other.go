//go:build !windows && !darwin

package main

import (
	"context"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func nativeWindowFrame(ctx context.Context) (windowFrame, error) {
	width, height := wailsruntime.WindowGetSize(ctx)
	x, y := wailsruntime.WindowGetPosition(ctx)
	return windowFrame{X: x, Y: y, Width: width, Height: height}, nil
}

func nativeSetWindowFrame(ctx context.Context, frame windowFrame) error {
	wailsruntime.WindowSetSize(ctx, frame.Width, frame.Height)
	wailsruntime.WindowSetPosition(ctx, frame.X, frame.Y)
	return nil
}

func isWindowRectVisible(x, y, width, height int) bool {
	return width > 0 && height > 0 && x >= 0 && y >= 0
}
