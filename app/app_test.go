package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteWindowConfigCreatesMissingFile(t *testing.T) {
	t.Setenv("FIRE_SALAMANDER_RUNTIME_DIR", t.TempDir())

	app := NewApp()
	t.Cleanup(app.state.Close)
	state := WindowState{
		Width:      1440,
		Height:     900,
		X:          120,
		Y:          80,
		Maximised:  false,
		Fullscreen: false,
	}

	if err := app.window.writeWindowConfig(state); err != nil {
		t.Fatalf("writeWindowConfig failed: %v", err)
	}

	configPath := filepath.Join(os.Getenv("FIRE_SALAMANDER_RUNTIME_DIR"), "config", "app.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var cfg struct {
		App struct {
			Language string `json:"language"`
		} `json:"app"`
		Window WindowState `json:"window"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if cfg.App.Language != "zh-CN" {
		t.Fatalf("expected default language zh-CN, got %q", cfg.App.Language)
	}
	if cfg.Window != state {
		t.Fatalf("unexpected window state: %#v", cfg.Window)
	}
}

func TestWriteWindowConfigRepairsInvalidFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("FIRE_SALAMANDER_RUNTIME_DIR", root)

	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	configPath := filepath.Join(configDir, "app.json")
	if err := os.WriteFile(configPath, []byte("{invalid json"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	app := NewApp()
	t.Cleanup(app.state.Close)
	state := WindowState{
		Width:      1280,
		Height:     800,
		X:          10,
		Y:          20,
		Maximised:  true,
		Fullscreen: false,
	}

	if err := app.window.writeWindowConfig(state); err != nil {
		t.Fatalf("writeWindowConfig failed: %v", err)
	}

	got := app.window.loadWindowConfig()
	if got != state {
		t.Fatalf("expected repaired config to contain %#v, got %#v", state, got)
	}
}
