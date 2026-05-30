//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework Cocoa -framework WebKit -framework AppKit
#import <Foundation/Foundation.h>
#import <AppKit/AppKit.h>
#import <dispatch/dispatch.h>

typedef struct NativeWindowFrame {
	int x;
	int y;
	int width;
	int height;
} NativeWindowFrame;

static NSWindow *CurrentWindow(void) {
	NSWindow *window = [NSApp mainWindow];
	if (window == nil) {
		window = [NSApp keyWindow];
	}
	if (window == nil && [[NSApp windows] count] > 0) {
		window = [[NSApp windows] objectAtIndex:0];
	}
	return window;
}

static int ReadNativeWindowFrame(NativeWindowFrame *out, int *minimised) {
	__block int ok = 0;
	void (^work)(void) = ^{
		NSWindow *window = CurrentWindow();
		if (window == nil || out == nil) {
			return;
		}
		NSRect frame = [window frame];
		out->x = (int)frame.origin.x;
		out->y = (int)frame.origin.y;
		out->width = (int)frame.size.width;
		out->height = (int)frame.size.height;
		if (minimised != nil) {
			*minimised = [window isMiniaturized] ? 1 : 0;
		}
		ok = 1;
	};

	if ([NSThread isMainThread]) {
		work();
	} else {
		dispatch_sync(dispatch_get_main_queue(), work);
	}
	return ok;
}

static int WriteNativeWindowFrame(int x, int y, int width, int height) {
	__block int ok = 0;
	void (^work)(void) = ^{
		NSWindow *window = CurrentWindow();
		if (window == nil) {
			return;
		}
		NSRect frame = NSMakeRect(x, y, width, height);
		[window setFrame:frame display:YES animate:NO];
		ok = 1;
	};

	if ([NSThread isMainThread]) {
		work();
	} else {
		dispatch_sync(dispatch_get_main_queue(), work);
	}
	return ok;
}

static int NativeScreenCount() {
	return (int)[[NSScreen screens] count];
}

static NativeWindowFrame NativeScreenFrameAt(int index) {
	NSScreen *screen = [[NSScreen screens] objectAtIndex:index];
	NSRect frame = [screen visibleFrame];
	NativeWindowFrame out;
	out.x = (int)frame.origin.x;
	out.y = (int)frame.origin.y;
	out.width = (int)frame.size.width;
	out.height = (int)frame.size.height;
	return out;
}
*/
import "C"

import (
	"context"
	"fmt"
)

func nativeWindowFrame(_ context.Context) (windowFrame, bool, error) {
	var frame C.NativeWindowFrame
	var minimised C.int
	if C.ReadNativeWindowFrame(&frame, &minimised) == 0 {
		return windowFrame{}, false, fmt.Errorf("无法获取 macOS 主窗口")
	}
	return windowFrame{
		X:      int(frame.x),
		Y:      int(frame.y),
		Width:  int(frame.width),
		Height: int(frame.height),
	}, minimised != 0, nil
}

func nativeSetWindowFrame(_ context.Context, frame windowFrame) error {
	if C.WriteNativeWindowFrame(C.int(frame.X), C.int(frame.Y), C.int(frame.Width), C.int(frame.Height)) == 0 {
		return fmt.Errorf("无法设置 macOS 主窗口位置")
	}
	return nil
}

func isWindowRectVisible(x, y, width, height int) bool {
	count := int(C.NativeScreenCount())
	if count == 0 {
		return true
	}

	window := windowFrame{X: x, Y: y, Width: width, Height: height}
	for i := 0; i < count; i++ {
		screen := C.NativeScreenFrameAt(C.int(i))
		rect := windowFrame{
			X:      int(screen.x),
			Y:      int(screen.y),
			Width:  int(screen.width),
			Height: int(screen.height),
		}
		if hasVisibleIntersection(window, rect) {
			return true
		}
	}

	return false
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
