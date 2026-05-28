package main

import (
	"context"
	"fmt"
	"reflect"
	"syscall"
	"unsafe"
)

type winRect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type monitorInfo struct {
	CbSize    uint32
	RcMonitor winRect
	RcWork    winRect
	DwFlags   uint32
}

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	procGetWindowRect       = user32.NewProc("GetWindowRect")
	procSetWindowPos        = user32.NewProc("SetWindowPos")
	procEnumDisplayMonitors = user32.NewProc("EnumDisplayMonitors")
	procGetMonitorInfoW     = user32.NewProc("GetMonitorInfoW")
)

func nativeWindowFrame(ctx context.Context) (windowFrame, error) {
	hwnd, err := nativeWindowHandle(ctx)
	if err != nil {
		return windowFrame{}, err
	}

	var rect winRect
	ret, _, callErr := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect)))
	if ret == 0 {
		return windowFrame{}, fmt.Errorf("GetWindowRect 失败: %w", callErr)
	}

	return windowFrame{
		X:      int(rect.Left),
		Y:      int(rect.Top),
		Width:  int(rect.Right - rect.Left),
		Height: int(rect.Bottom - rect.Top),
	}, nil
}

func nativeSetWindowFrame(ctx context.Context, frame windowFrame) error {
	hwnd, err := nativeWindowHandle(ctx)
	if err != nil {
		return err
	}

	const (
		hwndTop       = 0
		swpNoZOrder   = 0x0004
		swpNoActivate = 0x0010
	)

	ret, _, callErr := procSetWindowPos.Call(
		hwnd,
		hwndTop,
		uintptr(int32(frame.X)),
		uintptr(int32(frame.Y)),
		uintptr(int32(frame.Width)),
		uintptr(int32(frame.Height)),
		uintptr(swpNoZOrder|swpNoActivate),
	)
	if ret == 0 {
		return fmt.Errorf("SetWindowPos 失败: %w", callErr)
	}
	return nil
}

func nativeWindowHandle(ctx context.Context) (uintptr, error) {
	frontendValue, err := frontendValueFromContext(ctx)
	if err != nil {
		return 0, err
	}

	mainWindow, err := unsafeStructField(frontendValue, "mainWindow")
	if err != nil {
		return 0, err
	}
	if mainWindow.Kind() != reflect.Ptr || mainWindow.IsNil() {
		return 0, fmt.Errorf("主窗口句柄无效")
	}

	handle := mainWindow.MethodByName("Handle")
	if !handle.IsValid() {
		return 0, fmt.Errorf("未找到主窗口 Handle 方法")
	}

	results := handle.Call(nil)
	if len(results) != 1 {
		return 0, fmt.Errorf("Handle 返回值异常")
	}

	return uintptr(results[0].Uint()), nil
}

func isWindowRectVisible(x, y, width, height int) bool {
	rects, err := monitorRects()
	if err != nil || len(rects) == 0 {
		return true
	}

	window := windowFrame{X: x, Y: y, Width: width, Height: height}
	for _, rect := range rects {
		if hasVisibleIntersection(window, rect) {
			return true
		}
	}

	return false
}

func monitorRects() ([]windowFrame, error) {
	rects := make([]windowFrame, 0, 4)
	cb := syscall.NewCallback(func(hMonitor, _, _, _ uintptr) uintptr {
		var info monitorInfo
		info.CbSize = uint32(unsafe.Sizeof(info))
		ret, _, _ := procGetMonitorInfoW.Call(hMonitor, uintptr(unsafe.Pointer(&info)))
		if ret != 0 {
			rects = append(rects, windowFrame{
				X:      int(info.RcMonitor.Left),
				Y:      int(info.RcMonitor.Top),
				Width:  int(info.RcMonitor.Right - info.RcMonitor.Left),
				Height: int(info.RcMonitor.Bottom - info.RcMonitor.Top),
			})
		}
		return 1
	})

	ret, _, callErr := procEnumDisplayMonitors.Call(0, 0, cb, 0)
	if ret == 0 {
		return nil, fmt.Errorf("EnumDisplayMonitors 失败: %w", callErr)
	}

	return rects, nil
}

func hasVisibleIntersection(window, screen windowFrame) bool {
	left := max(window.X, screen.X)
	top := max(window.Y, screen.Y)
	right := min(window.X+window.Width, screen.X+screen.Width)
	bottom := min(window.Y+window.Height, screen.Y+screen.Height)
	return right-left >= minVisibleWidth && bottom-top >= minVisibleHeight
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
