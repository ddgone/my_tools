package shared

import (
	"context"
	"sync"

	"fire-salamander-desktop/internal/exechistory"
	"fire-salamander-desktop/internal/ssh"
	"my_tools/libs/core/toolspec"
)

// SharedState 集中保管所有共享可变状态，供各 internal 子包使用。
type SharedState struct {
	Mu            sync.RWMutex
	Ctx           context.Context
	PyTools       map[string]*PythonToolEntry
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
	RecordStore   *exechistory.Store
}

func NewSharedState() *SharedState {
	recordStore, err := exechistory.NewStore()
	if err != nil {
		recordStore = nil
	}
	return &SharedState{
		PyTools:       map[string]*PythonToolEntry{},
		Manifests:     map[string]toolspec.ToolManifest{},
		Tasks:         map[string]*ExecutionTask{},
		DownloadTasks: map[string]*DownloadTask{},
		ArtifactTasks: map[string]*ArtifactBatchTask{},
		Cancels:       map[string]context.CancelFunc{},
		SSHStore:      ssh.NewStore(),
		RecordStore:   recordStore,
	}
}

// Close 释放持有的持久化资源（如执行记录数据库连接）。
func (s *SharedState) Close() {
	if s.RecordStore != nil {
		_ = s.RecordStore.Close()
	}
}
