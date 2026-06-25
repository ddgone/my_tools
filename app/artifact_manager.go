package main

import (
	"fire-salamander-desktop/internal/artifact"
)

func NewArtifactBatchManager(state *SharedState) *artifact.Manager {
	return artifact.NewManager(state, ensureTooling)
}

// App delegates

func (a *App) StartArtifactBatch(req ArtifactBatchRequest) (*ArtifactBatchTask, error) {
	return a.artifact.StartArtifactBatch(req)
}

func (a *App) ListArtifactBatchTasks() []*ArtifactBatchTask {
	return a.artifact.ListArtifactBatchTasks()
}

func (a *App) ClearArtifactBatchTasks() error {
	return a.artifact.ClearArtifactBatchTasks()
}

func (a *App) EstimateArtifactBatchCache(req ArtifactBatchRequest) (*ArtifactBatchEstimate, error) {
	return a.artifact.EstimateArtifactBatchCache(req)
}

func (a *App) loadArtifactBatchTasks() error {
	return a.artifact.LoadArtifactBatchTasks()
}

func (a *App) emitArtifactTaskUpdate(task *ArtifactBatchTask) {
	a.artifact.EmitArtifactTaskUpdate(task)
}
