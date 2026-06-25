package main

import (
	"context"
	"sync"

	"fire-salamander-desktop/internal/ssh"
	"my_tools/libs/core/toolspec"
)

// SharedState 集中保管 App 层所有共享的、需要加锁访问的可变状态。
// Phase 1 阶段字段保持公开，和原来 App 上的字段名一致，
// 使得迁移只需将 a.xxx 改为 a.state.Xxx 即可。
// Phase 2 挪到 internal/ 时再收紧为方法接口。
type SharedState struct {
	Mu            sync.RWMutex
	Ctx           context.Context
	PyTools       map[string]*pythonToolEntry
	Manifests     map[string]toolspec.ToolManifest
	Tasks         map[string]*ExecutionTask
	DownloadTasks map[string]*DownloadTask
	ArtifactTasks map[string]*ArtifactBatchTask
	Cancels       map[string]context.CancelFunc
	PythonTask    *PythonToolchainTask
	PythonCancel  context.CancelFunc
	GoTask        *GoToolchainTask
	GoCancel      context.CancelFunc
	RustTask      *RustToolchainTask
	RustCancel    context.CancelFunc
	SSHStore      *ssh.Store
}

func NewSharedState() *SharedState {
	return &SharedState{
		PyTools:       map[string]*pythonToolEntry{},
		Manifests:     map[string]toolspec.ToolManifest{},
		Tasks:         map[string]*ExecutionTask{},
		DownloadTasks: map[string]*DownloadTask{},
		ArtifactTasks: map[string]*ArtifactBatchTask{},
		Cancels:       map[string]context.CancelFunc{},
		SSHStore:      ssh.NewStore(),
	}
}
