package main

import (
	"path/filepath"
	"testing"
)

func TestArtifactBatchTasksFileRoundTrip(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "artifact_tasks.json")
	input := []*ArtifactBatchTask{
		{
			ID:        "task_old",
			Mode:      artifactBatchModeExport,
			Status:    "success",
			StartedAt: 100,
			Items: []ArtifactBatchItemResult{
				{Key: "a", Status: "success"},
			},
		},
		{
			ID:        "task_new",
			Mode:      artifactBatchModeBuildCache,
			Status:    "partial",
			StartedAt: 200,
			Items: []ArtifactBatchItemResult{
				{Key: "b", Status: "cached"},
				{Key: "c", Status: "skipped"},
				{Key: "d", Status: "error"},
			},
		},
	}

	if err := saveArtifactBatchTasksFile(filePath, input); err != nil {
		t.Fatalf("saveArtifactBatchTasksFile returned error: %v", err)
	}

	loaded, err := loadArtifactBatchTasksFile(filePath)
	if err != nil {
		t.Fatalf("loadArtifactBatchTasksFile returned error: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("unexpected task count: got %d want 2", len(loaded))
	}
	if loaded[0].ID != "task_new" {
		t.Fatalf("expected newest task first, got %s", loaded[0].ID)
	}
	if loaded[0].CachedCount != 1 || loaded[0].SkippedCount != 1 || loaded[0].ErrorCount != 1 {
		t.Fatalf("unexpected recount result: %+v", loaded[0])
	}
}

func TestClearArtifactBatchTasksRejectsRunningTask(t *testing.T) {
	app := NewApp()
	app.artifactTasks["task_running"] = &ArtifactBatchTask{
		ID:        "task_running",
		Status:    "running",
		StartedAt: 1,
	}

	if err := app.ClearArtifactBatchTasks(); err == nil {
		t.Fatal("expected clear to fail when tasks are running")
	}
}
