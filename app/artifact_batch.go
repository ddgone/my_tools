package main

const (
	artifactBatchModeBuildCache = "build_cache"
	artifactBatchModeExport     = "export"
	maxArtifactBatchTaskHistory = 10
	artifactBatchTasksFileName  = "artifact_tasks.json"
)

type ArtifactBatchSelection struct {
	ToolID     string `json:"toolId"`
	TargetOS   string `json:"targetOS"`
	TargetArch string `json:"targetArch"`
}

type ArtifactBatchRequest struct {
	Mode            string                   `json:"mode"`
	ExportRootDir   string                   `json:"exportRootDir,omitempty"`
	Concurrency     int                      `json:"concurrency"`
	SkipUnchanged   bool                     `json:"skipUnchanged"`
	PreferCache     bool                     `json:"preferCache"`
	ForceRebuild    bool                     `json:"forceRebuild"`
	ContinueOnError bool                     `json:"continueOnError"`
	Items           []ArtifactBatchSelection `json:"items"`
}

type ArtifactBatchItemResult struct {
	Key        string `json:"key"`
	ToolID     string `json:"toolId"`
	ToolName   string `json:"toolName"`
	Kind       string `json:"kind"`
	TargetOS   string `json:"targetOS"`
	TargetArch string `json:"targetArch"`
	Status     string `json:"status"`
	Message    string `json:"message"`
	OutputPath string `json:"outputPath,omitempty"`
	CacheHit   bool   `json:"cacheHit"`
	StartedAt  int64  `json:"startedAt"`
	EndedAt    int64  `json:"endedAt,omitempty"`
}

type ArtifactBatchTask struct {
	ID              string                    `json:"id"`
	Mode            string                    `json:"mode"`
	Status          string                    `json:"status"`
	ExportRootDir   string                    `json:"exportRootDir,omitempty"`
	Concurrency     int                       `json:"concurrency"`
	SkipUnchanged   bool                      `json:"skipUnchanged"`
	PreferCache     bool                      `json:"preferCache"`
	ForceRebuild    bool                      `json:"forceRebuild"`
	ContinueOnError bool                      `json:"continueOnError"`
	TotalCount      int                       `json:"totalCount"`
	SuccessCount    int                       `json:"successCount"`
	ErrorCount      int                       `json:"errorCount"`
	CachedCount     int                       `json:"cachedCount"`
	SkippedCount    int                       `json:"skippedCount"`
	StartedAt       int64                     `json:"startedAt"`
	EndedAt         int64                     `json:"endedAt,omitempty"`
	CurrentItem     string                    `json:"currentItem,omitempty"`
	ExitMessage     string                    `json:"exitMessage,omitempty"`
	Items           []ArtifactBatchItemResult `json:"items"`
}

type ArtifactBatchEstimate struct {
	TotalCount   int `json:"totalCount"`
	CachedCount  int `json:"cachedCount"`
	BuildCount   int `json:"buildCount"`
	InvalidCount int `json:"invalidCount"`
}

func (a *App) StartArtifactBatch(req ArtifactBatchRequest) (*ArtifactBatchTask, error) {
	if err := a.ensureTooling(); err != nil {
		return nil, err
	}
	resolved, task, err := a.prepareArtifactBatch(req)
	if err != nil {
		return nil, err
	}

	a.mu.Lock()
	a.artifactTasks[task.ID] = task
	a.trimArtifactBatchTasksLocked()
	a.persistArtifactBatchTasksLocked()
	snapshot := cloneArtifactTask(task)
	a.mu.Unlock()
	a.emitArtifactTaskUpdate(snapshot)

	go a.runArtifactBatch(task.ID, resolved)

	return snapshot, nil
}
