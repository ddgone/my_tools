package main

import (
	"testing"

	gosettings "fire-salamander-desktop/internal/toolchainsettings/go"
	pythonsettings "fire-salamander-desktop/internal/toolchainsettings/python"
	rustsettings "fire-salamander-desktop/internal/toolchainsettings/rust"
)

func TestGoSettingsManagerWiresUp(t *testing.T) {
	state := NewSharedState()
	mgr := gosettings.NewManager(state, nil)
	if mgr == nil {
		t.Fatal("expected manager to be non-nil")
	}
}

func TestPythonSettingsManagerWiresUp(t *testing.T) {
	state := NewSharedState()
	mgr := pythonsettings.NewManager(state, nil)
	if mgr == nil {
		t.Fatal("expected manager to be non-nil")
	}
}

func TestRustSettingsManagerWiresUp(t *testing.T) {
	state := NewSharedState()
	mgr := rustsettings.NewManager(state, nil)
	if mgr == nil {
		t.Fatal("expected manager to be non-nil")
	}
}
