package execution

import (
	"context"
	"fmt"
	"strings"
	"time"

	"fire-salamander-desktop/internal/shared"
	"fire-salamander-desktop/internal/ssh"
	"fire-salamander-desktop/internal/toolchain"
	"my_tools/libs/core/toolspec"
)

// Manager orchestrates local and remote tool executions.
type Manager struct {
	state         *shared.SharedState
	emitter       shared.TaskEventEmitter
	ensureTooling func(*shared.SharedState) error
}

// NewManager creates a new execution Manager.
func NewManager(state *shared.SharedState, emitter shared.TaskEventEmitter, ensureTooling func(*shared.SharedState) error) *Manager {
	return &Manager{state: state, emitter: emitter, ensureTooling: ensureTooling}
}

// StartLocalExecution runs a tool on the local machine.
func (m *Manager) StartLocalExecution(req shared.ExecutionRequest) (*shared.ExecutionTask, error) {
	if err := m.ensureTooling(m.state); err != nil {
		return nil, err
	}

	m.state.Mu.RLock()
	pyTool, pyOK := m.state.PyTools[req.ToolID]
	manifest, manifestOK := m.state.Manifests[req.ToolID]
	m.state.Mu.RUnlock()

	if !pyOK && !manifestOK {
		return nil, fmt.Errorf("未找到工具: %s", req.ToolID)
	}

	usage := ""
	if manifestOK && manifest.Docs.Usage != "" {
		usage = manifest.Docs.Usage
	}

	toolName := manifest.Name
	if toolName == "" && pyOK && pyTool != nil {
		toolName = pyTool.ScriptName
	}

	task := &shared.ExecutionTask{
		ID:         fmt.Sprintf("task_%d", time.Now().UnixNano()),
		ToolID:     req.ToolID,
		InstanceID: req.InstanceID,
		ToolName:   toolName,
		Status:     "running",
		Target:     "local",
		Args:       req.Args,
		PythonEnv:  req.PythonEnv,
		Usage:      usage,
		StartedAt:  time.Now().UnixMilli(),
	}

	runCtx, _ := m.RegisterExecutionTask(task)

	go func() {
		writer := &taskEventWriter{taskID: task.ID, emitter: m.emitter}
		var execErr error
		switch {
		case manifestOK && (manifest.Kind == toolspec.ToolKindGo || manifest.Kind == toolspec.ToolKindRust):
			execErr = ExecuteLocalBinaryTool(runCtx, writer, manifest)(req.Args)
		case pyOK && pyTool != nil && pyTool.Run != nil:
			pythonEnv := strings.TrimSpace(req.PythonEnv)
			if pythonEnv == "" {
				pythonEnv, execErr = toolchain.ResolvePythonBinaryForTool(req.ToolID)
				if execErr == nil {
					m.state.Mu.Lock()
					task.PythonEnv = pythonEnv
					m.state.Mu.Unlock()
				}
			}
			if execErr == nil {
				execErr = pyTool.Run(runCtx, pythonEnv, req.Args, writer)
			}
		default:
			execErr = fmt.Errorf("工具 %s 缺少可执行入口", task.ToolName)
		}
		writer.Flush()

		m.FinishExecutionTask(task.ID, runCtx, execErr, "任务执行完成", nil)
	}()

	return task, nil
}

// StartRemoteExecution runs a tool on a remote machine via SSH.
func (m *Manager) StartRemoteExecution(req RemoteExecRequest) (*shared.ExecutionTask, error) {
	if err := m.ensureTooling(m.state); err != nil {
		return nil, err
	}

	m.state.Mu.RLock()
	manifest, manifestOK := m.state.Manifests[req.ToolID]
	m.state.Mu.RUnlock()

	if !manifestOK {
		return nil, fmt.Errorf("未找到工具: %s", req.ToolID)
	}

	conns := m.state.SSHStore.List()
	var target *ssh.Connection
	for _, c := range conns {
		if c.ID == req.ConnID {
			target = c
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("未找到SSH连接: %s", req.ConnID)
	}

	realConn, err := m.state.SSHStore.GetCredentials(req.ConnID)
	if err != nil {
		return nil, fmt.Errorf("SSH连接凭据已失效: %w", err)
	}

	task := &shared.ExecutionTask{
		ID:           fmt.Sprintf("task_%d", time.Now().UnixNano()),
		ToolID:       req.ToolID,
		InstanceID:   req.InstanceID,
		ToolName:     manifest.Name,
		Status:       "running",
		Target:       "remote:" + target.Host,
		RemoteConnID: req.ConnID,
		Args:         req.Args,
		PythonEnv:    req.PythonEnv,
		StartedAt:    time.Now().UnixMilli(),
	}

	if task.ToolName == "" && manifestOK {
		task.ToolName = manifest.Name
	}

	runCtx, cancel := m.RegisterExecutionTask(task)

	go func() {
		defer cancel()
		writer := &taskEventWriter{taskID: task.ID, emitter: m.emitter}
		defer writer.Flush()

		outcome, execErr := ExecuteRemotely(runCtx, writer, remoteExecParams{
			host:               target.Host,
			port:               target.Port,
			user:               target.User,
			password:           realConn.Password,
			keyPath:            realConn.KeyPath,
			hostKeyFingerprint: realConn.HostKeyFingerprint,
			toolID:             req.ToolID,
			taskID:             task.ID,
			toolName:           task.ToolName,
			kind:               string(manifest.Kind),
			args:               req.Args,
			pythonEnv:          req.PythonEnv,
			sourceEntry:        manifest.Source.Entry,
			manifestParams:     manifest.Params,
		})

		m.FinishExecutionTask(task.ID, runCtx, execErr, "远程任务执行完成", func(task *shared.ExecutionTask) {
			task.RemoteResultStatus = outcome.result.Status
			task.RemoteResultPath = outcome.result.Path
			task.RemoteResultKind = outcome.result.Kind
			task.RemoteResultMessage = outcome.result.Message
		})
	}()

	return task, nil
}

// RegisterExecutionTask adds a task to the shared state and emits its initial update.
func (m *Manager) RegisterExecutionTask(task *shared.ExecutionTask) (context.Context, context.CancelFunc) {
	runCtx, cancel := context.WithCancel(context.Background())

	m.state.Mu.Lock()
	m.state.Tasks[task.ID] = task
	m.state.Cancels[task.ID] = cancel
	m.state.Mu.Unlock()

	m.emitter.EmitTaskUpdate(task)
	return runCtx, cancel
}

// FinishExecutionTask records a task's completion status and emits the final update.
func (m *Manager) FinishExecutionTask(taskID string, runCtx context.Context, execErr error, successMessage string, update func(task *shared.ExecutionTask)) {
	m.state.Mu.Lock()
	task, ok := m.state.Tasks[taskID]
	if !ok {
		m.state.Mu.Unlock()
		return
	}

	delete(m.state.Cancels, taskID)
	task.EndedAt = time.Now().UnixMilli()

	logMessage := ""
	switch {
	case runCtx.Err() != nil:
		task.Status = "canceled"
		task.ExitMessage = "任务已取消"
	case execErr != nil:
		task.Status = "error"
		task.ExitMessage = execErr.Error()
		logMessage = "[错误] " + execErr.Error()
	default:
		task.Status = "success"
		task.ExitMessage = successMessage
	}

	if update != nil {
		update(task)
	}

	copyTask := *task
	m.state.Mu.Unlock()

	if logMessage != "" {
		m.emitter.EmitTaskLog(taskID, logMessage)
	}
	m.emitter.EmitTaskUpdate(&copyTask)
}

// CancelExecution cancels a running task by its ID.
func (m *Manager) CancelExecution(taskID string) error {
	m.state.Mu.RLock()
	cancel, ok := m.state.Cancels[taskID]
	m.state.Mu.RUnlock()
	if !ok {
		return fmt.Errorf("未找到正在运行的任务: %s", taskID)
	}
	cancel()
	return nil
}

// ListTasks returns a copy of all current execution tasks.
func (m *Manager) ListTasks() []*shared.ExecutionTask {
	m.state.Mu.RLock()
	defer m.state.Mu.RUnlock()

	tasks := make([]*shared.ExecutionTask, 0, len(m.state.Tasks))
	for _, task := range m.state.Tasks {
		copyTask := *task
		tasks = append(tasks, &copyTask)
	}
	return tasks
}
