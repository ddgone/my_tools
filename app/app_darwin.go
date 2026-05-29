//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework Cocoa -framework WebKit -framework AppKit
#import <Foundation/Foundation.h>
#import <AppKit/AppKit.h>

#import "WailsContext.h"

typedef struct NativeWindowFrame {
	int x;
	int y;
	int width;
	int height;
} NativeWindowFrame;

static NativeWindowFrame GetNativeWindowFrame(void *inctx) {
	WailsContext *ctx = (__bridge WailsContext*) inctx;
	NSRect frame = [ctx.mainWindow frame];
	NativeWindowFrame out;
	out.x = (int)frame.origin.x;
	out.y = (int)frame.origin.y;
	out.width = (int)frame.size.width;
	out.height = (int)frame.size.height;
	return out;
}

static void SetNativeWindowFrame(void *inctx, int x, int y, int width, int height) {
	WailsContext *ctx = (__bridge WailsContext*) inctx;
	NSRect frame = NSMakeRect(x, y, width, height);
	[ctx.mainWindow setFrame:frame display:YES animate:NO];
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
	"reflect"
	"unsafe"
)

func nativeWindowFrame(ctx context.Context) (windowFrame, bool, error) {
	ptr, err := nativeWindowContext(ctx)
	if err != nil {
		return windowFrame{}, false, err
	}

	frame := C.GetNativeWindowFrame(ptr)
	return windowFrame{
		X:      int(frame.x),
		Y:      int(frame.y),
		Width:  int(frame.width),
		Height: int(frame.height),
	}, false, nil
}

func nativeSetWindowFrame(ctx context.Context, frame windowFrame) error {
	ptr, err := nativeWindowContext(ctx)
	if err != nil {
		return err
	}

	C.SetNativeWindowFrame(ptr, C.int(frame.X), C.int(frame.Y), C.int(frame.Width), C.int(frame.Height))
	return nil
}

func nativeWindowContext(ctx context.Context) (unsafe.Pointer, error) {
	frontendValue, err := frontendValueFromContext(ctx)
	if err != nil {
		return nil, err
	}

	mainWindow, err := unsafeStructField(frontendValue, "mainWindow")
	if err != nil {
		return nil, err
	}
	if mainWindow.Kind() != reflect.Ptr || mainWindow.IsNil() {
		return nil, fmt.Errorf("主窗口句柄无效")
	}

	contextField, err := unsafeStructField(mainWindow, "context")
	if err != nil {
		return nil, err
	}

	return unsafe.Pointer(contextField.Pointer()), nil
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
