package main

import (
	"fire-salamander-desktop/internal/artifact"
	"fire-salamander-desktop/internal/execution"
	"fire-salamander-desktop/internal/exportpkg"
	runtimebridge "fire-salamander-desktop/internal/runtime"
	"fire-salamander-desktop/internal/shared"
	"fire-salamander-desktop/internal/taskresult"

	gosettings "fire-salamander-desktop/internal/toolchainsettings/go"
	pythonsettings "fire-salamander-desktop/internal/toolchainsettings/python"
	rustsettings "fire-salamander-desktop/internal/toolchainsettings/rust"
)

// Type aliases — all shared types are defined in internal/ packages.
// Using type aliases keeps Wails-bound method signatures stable.

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
	ExecutionManager       = execution.Manager
	RemoteExecRequest      = execution.RemoteExecRequest
	TaskLogEvent           = execution.TaskLogEvent
	ArtifactBatchManager   = artifact.Manager
	ExportManager          = exportpkg.Manager
	TaskResultManager      = taskresult.Manager
	RemotePathEntry        = runtimebridge.RemotePathEntry
	RemotePathBrowseResult = runtimebridge.RemotePathBrowseResult
	ExportToolRequest      = exportpkg.ExportToolRequest
	ExportToolResult       = exportpkg.ExportToolResult
	ExportProgressEvent    = exportpkg.ExportProgressEvent

	// Toolchain settings managers (moved to internal/toolchainsettings/).
	GoSettingsManager     = gosettings.Manager
	PythonSettingsManager = pythonsettings.Manager
	RustSettingsManager   = rustsettings.Manager

	// Toolchain settings frontend types (kept accessible for Wails binding).
	GoToolchainConfig         = gosettings.GoToolchainConfig
	GoToolchainCandidate      = gosettings.GoToolchainCandidate
	GoToolchainState          = gosettings.GoToolchainState
	GoRuntimeDetails          = gosettings.GoRuntimeDetails
	GoOfficialRelease         = gosettings.GoOfficialRelease
	GoToolchainTaskState      = gosettings.GoToolchainTaskState
	InstallGoToolchainRequest = gosettings.InstallGoToolchainRequest

	PythonToolchainConfig    = pythonsettings.PythonToolchainConfig
	PythonToolchainCandidate = pythonsettings.PythonToolchainCandidate
	PythonDependencyStatus   = pythonsettings.PythonDependencyStatus
	PythonToolchainState     = pythonsettings.PythonToolchainState
	PythonToolchainTaskState = pythonsettings.PythonToolchainTaskState

	RustToolchainConfig         = rustsettings.RustToolchainConfig
	RustToolchainCandidate      = rustsettings.RustToolchainCandidate
	RustToolchainTargetStatus   = rustsettings.RustToolchainTargetStatus
	RustToolchainState          = rustsettings.RustToolchainState
	RustToolchainEnvironment    = rustsettings.RustToolchainEnvironment
	RustOfficialRelease         = rustsettings.RustOfficialRelease
	ZigOfficialRelease          = rustsettings.ZigOfficialRelease
	RustToolchainTaskState      = rustsettings.RustToolchainTaskState
	InstallRustToolchainRequest = rustsettings.InstallRustToolchainRequest
)

const (
	artifactBatchModeBuildCache = shared.ArtifactBatchModeBuildCache
	artifactBatchModeExport     = shared.ArtifactBatchModeExport
	maxArtifactBatchTaskHistory = shared.MaxArtifactBatchTaskHistory
	artifactBatchTasksFileName  = shared.ArtifactBatchTasksFileName
)

func NewSharedState() *SharedState { return shared.NewSharedState() }
