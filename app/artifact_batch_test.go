package main

import (
	"testing"
)

func TestClearArtifactBatchTasksRejectsRunningTask(t *testing.T) {
	app := NewApp()
	app.state.ArtifactTasks["task_running"] = &ArtifactBatchTask{
		ID:        "task_running",
		Status:    "running",
		StartedAt: 1,
	}

	if err := app.ClearArtifactBatchTasks(); err == nil {
		t.Fatal("expected clear to fail when tasks are running")
	}
}
