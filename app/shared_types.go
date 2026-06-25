package main

import (
	"fire-salamander-desktop/internal/artifact"
	"fire-salamander-desktop/internal/execution"
	"fire-salamander-desktop/internal/exportpkg"
	"fire-salamander-desktop/internal/shared"
	"fire-salamander-desktop/internal/taskresult"
)

// Type aliases — all shared types are defined in internal/shared/.
// Using type aliases keeps existing code unchanged until each manager
// gets its own package.

type (
	SharedState             = shared.SharedState
	ExecutionRequest        = shared.ExecutionRequest
	ExecutionTask           = shared.ExecutionTask
	DownloadTask            = shared.DownloadTask
	ArtifactBatchTask       = shared.ArtifactBatchTask
	ArtifactBatchRequest    = shared.ArtifactBatchRequest
	ArtifactBatchEstimate   = shared.ArtifactBatchEstimate
	ArtifactBatchSelection  = shared.ArtifactBatchSelection
	ArtifactBatchItemResult = shared.ArtifactBatchItemResult
	GoToolchainTask         = shared.GoToolchainTask
	PythonToolchainTask     = shared.PythonToolchainTask
	RustToolchainTask       = shared.RustToolchainTask
	PythonToolEntry         = shared.PythonToolEntry
	PythonRunFunc           = shared.PythonRunFunc

	// Moved to internal/ packages.
	ExecutionManager     = execution.Manager
	RemoteExecRequest    = execution.RemoteExecRequest
	TaskLogEvent         = execution.TaskLogEvent
	ArtifactBatchManager = artifact.Manager
	ExportManager        = exportpkg.Manager
	TaskResultManager    = taskresult.Manager
	ExportToolRequest    = exportpkg.ExportToolRequest
	ExportToolResult     = exportpkg.ExportToolResult
	ExportProgressEvent  = exportpkg.ExportProgressEvent
)

const (
	artifactBatchModeBuildCache = shared.ArtifactBatchModeBuildCache
	artifactBatchModeExport     = shared.ArtifactBatchModeExport
	maxArtifactBatchTaskHistory = shared.MaxArtifactBatchTaskHistory
	artifactBatchTasksFileName  = shared.ArtifactBatchTasksFileName
)

func NewSharedState() *SharedState { return shared.NewSharedState() }
