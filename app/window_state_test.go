package main

import "testing"

func TestShouldRestoreSavedFrameRejectsMaximisedAndFullscreen(t *testing.T) {
	frame := windowFrame{X: 100, Y: 100, Width: 1280, Height: 800}

	if shouldRestoreSavedFrame(WindowState{Maximised: true}, frame) {
		t.Fatal("expected maximised window state to skip saved frame restore")
	}
	if shouldRestoreSavedFrame(WindowState{Fullscreen: true}, frame) {
		t.Fatal("expected fullscreen window state to skip saved frame restore")
	}
}

func TestShouldRestoreSavedFrameAcceptsVisibleNormalWindow(t *testing.T) {
	frame := windowFrame{X: 0, Y: 0, Width: 800, Height: 600}
	if !shouldRestoreSavedFrame(WindowState{}, frame) {
		t.Fatal("expected visible normal window state to restore saved frame")
	}
}
