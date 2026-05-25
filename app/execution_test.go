package main

import (
	"strings"
	"testing"
	"time"
)

func TestEnsureToolingLoadsLegacyTools(t *testing.T) {
	app := NewApp()
	if err := app.ensureTooling(); err != nil {
		t.Fatalf("ensureTooling failed: %v", err)
	}

	if len(app.legacy) < 4 {
		t.Fatalf("expected at least 4 legacy tools, got %d", len(app.legacy))
	}

	if _, ok := app.manifests["geojson_to_shp"]; !ok {
		t.Fatalf("expected geojson_to_shp manifest to be available")
	}
}

func TestStartLocalExecutionRunsLegacyTool(t *testing.T) {
	app := NewApp()
	if err := app.ensureTooling(); err != nil {
		t.Fatalf("ensureTooling failed: %v", err)
	}

	tempDir := t.TempDir()
	task, err := app.StartLocalExecution(ExecutionRequest{
		ToolID: "geojson_to_shp",
		Args:   `-input "` + tempDir + `"`,
	})
	if err != nil {
		t.Fatalf("StartLocalExecution failed: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		tasks := app.ListTasks()
		for _, current := range tasks {
			if current.ID != task.ID {
				continue
			}
			if current.Status == "running" {
				time.Sleep(50 * time.Millisecond)
				continue
			}
			if current.Status != "error" {
				t.Fatalf("expected error status for empty input dir, got %s", current.Status)
			}
			if !strings.Contains(current.ExitMessage, "未找到任何 GeoJSON 文件") {
				t.Fatalf("unexpected exit message: %s", current.ExitMessage)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("task %s did not finish before deadline", task.ID)
}
