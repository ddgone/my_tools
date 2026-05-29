package main

import "testing"

func TestFrameFromWindowPlacementUsesNormalRectForMinimisedWindow(t *testing.T) {
	placement := windowPlacement{
		ShowCmd: 2,
		RcNormalPosition: winRect{
			Left:   120,
			Top:    80,
			Right:  1400,
			Bottom: 880,
		},
	}

	frame, minimised := frameFromWindowPlacement(placement)
	if !minimised {
		t.Fatal("expected minimised placement to be detected")
	}
	if frame.X != 120 || frame.Y != 80 || frame.Width != 1280 || frame.Height != 800 {
		t.Fatalf("unexpected frame: %+v", frame)
	}
}

func TestIsPlacementMinimisedRecognisesWindowsMinimiseShowCommands(t *testing.T) {
	minimisedShowCmds := []uint32{2, 6, 7, 11}
	for _, showCmd := range minimisedShowCmds {
		if !isPlacementMinimised(showCmd) {
			t.Fatalf("expected showCmd %d to be treated as minimised", showCmd)
		}
	}

	normalShowCmds := []uint32{1, 3, 5, 9, 10}
	for _, showCmd := range normalShowCmds {
		if isPlacementMinimised(showCmd) {
			t.Fatalf("expected showCmd %d to be treated as non-minimised", showCmd)
		}
	}
}
