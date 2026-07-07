package shared

import (
	"context"
	"io"
)

// ------- execution -------

type ExecutionRequest struct {
	ToolID     string `json:"toolId"`
	InstanceID string `json:"instanceId"`
	Args       string `json:"args"`
	PythonEnv  string `json:"pythonEnv"`
}

type ExecutionTask struct {
	ID                         string `json:"id"`
	ToolID                     string `json:"toolId"`
	InstanceID                 string `json:"instanceId"`
	ToolName                   string `json:"toolName"`
	Status                     string `json:"status"`
	Target                     string `json:"target"`
	RemoteConnID               string `json:"remoteConnId,omitempty"`
	Args                       string `json:"args"`
	PythonEnv                  string `json:"pythonEnv,omitempty"`
	Usage                      string `json:"usage"`
	StartedAt                  int64  `json:"startedAt"`
	EndedAt                    int64  `json:"endedAt,omitempty"`
	ExitMessage                string `json:"exitMessage,omitempty"`
	RemoteResultStatus         string `json:"remoteResultStatus,omitempty"`
	RemoteResultPath           string `json:"remoteResultPath,omitempty"`
	RemoteResultKind           string `json:"remoteResultKind,omitempty"`
	RemoteResultMessage        string `json:"remoteResultMessage,omitempty"`
	RemoteResultDownloadedPath string `json:"remoteResultDownloadedPath,omitempty"`
	PythonEnvResolved          string `json:"-"` // internal, not serialized
}

// ------- download -------

type DownloadTask struct {
	ID               string  `json:"id"`
	SourceTaskID     string  `json:"sourceTaskId"`
	ToolID           string  `json:"toolId"`
	ToolName         string  `json:"toolName"`
	Status           string  `json:"status"`
	RemoteResultPath string  `json:"remoteResultPath"`
	RemoteResultKind string  `json:"remoteResultKind"`
	LocalPath        string  `json:"localPath,omitempty"`
	Directory        string  `json:"directory,omitempty"`
	Message          string  `json:"message,omitempty"`
	DownloadedBytes  int64   `json:"downloadedBytes"`
	TotalBytes       int64   `json:"totalBytes"`
	ProgressPercent  float64 `json:"progressPercent"`
	StartedAt        int64   `json:"startedAt"`
	EndedAt          int64   `json:"endedAt,omitempty"`
}

// ------- artifact batch -------

const (
	ArtifactBatchModeBuildCache = "build_cache"
	ArtifactBatchModeExport     = "export"
	MaxArtifactBatchTaskHistory = 10
	ArtifactBatchTasksFileName  = "artifact_tasks.json"
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

// ------- toolchain tasks -------

type GoToolchainTask struct {
	Kind             string  `json:"kind"`
	Status           string  `json:"status"`
	Message          string  `json:"message"`
	Detail           string  `json:"detail,omitempty"`
	CurrentItem      string  `json:"currentItem,omitempty"`
	CurrentSource    string  `json:"currentSource,omitempty"`
	ProgressPercent  float64 `json:"progressPercent"`
	Step             int     `json:"step"`
	TotalSteps       int     `json:"totalSteps"`
	Version          string  `json:"version,omitempty"`
	Directory        string  `json:"directory,omitempty"`
	TransferredBytes int64   `json:"transferredBytes,omitempty"`
	TotalBytes       int64   `json:"totalBytes,omitempty"`
	TransferSpeed    string  `json:"transferSpeed,omitempty"`
	Error            string  `json:"error,omitempty"`
	UpdatedAt        int64   `json:"updatedAt"`
}

type PythonToolchainTask struct {
	Kind                 string  `json:"kind"`
	Status               string  `json:"status"`
	Message              string  `json:"message"`
	Detail               string  `json:"detail,omitempty"`
	CurrentItem          string  `json:"currentItem,omitempty"`
	ProgressPercent      float64 `json:"progressPercent"`
	Step                 int     `json:"step"`
	TotalSteps           int     `json:"totalSteps"`
	BaseBinary           string  `json:"baseBinary,omitempty"`
	EnvironmentDirectory string  `json:"environmentDirectory,omitempty"`
	Error                string  `json:"error,omitempty"`
	UpdatedAt            int64   `json:"updatedAt"`
}

type RustToolchainTask struct {
	Kind             string  `json:"kind"`
	Status           string  `json:"status"`
	Message          string  `json:"message"`
	Detail           string  `json:"detail,omitempty"`
	CurrentItem      string  `json:"currentItem,omitempty"`
	CurrentSource    string  `json:"currentSource,omitempty"`
	ProgressPercent  float64 `json:"progressPercent"`
	Step             int     `json:"step"`
	TotalSteps       int     `json:"totalSteps"`
	RustVersion      string  `json:"rustVersion,omitempty"`
	ZigVersion       string  `json:"zigVersion,omitempty"`
	Directory        string  `json:"directory,omitempty"`
	TransferredBytes int64   `json:"transferredBytes,omitempty"`
	TotalBytes       int64   `json:"totalBytes,omitempty"`
	TransferSpeed    string  `json:"transferSpeed,omitempty"`
	Error            string  `json:"error,omitempty"`
	UpdatedAt        int64   `json:"updatedAt"`
}

// ------- python tool entry -------

// PythonRunFunc is the signature for Python script execution closures.
type PythonRunFunc func(ctx context.Context, env string, args string, out io.Writer) error

// TaskEventEmitter is implemented by the task result manager to emit
// execution-related events to the frontend.
type TaskEventEmitter interface {
	EmitTaskUpdate(task *ExecutionTask)
	EmitTaskLog(taskID string, message string)
	EmitDownloadTaskUpdate(task *DownloadTask)
}

type PythonToolEntry struct {
	ScriptName string
	Run        PythonRunFunc
}
