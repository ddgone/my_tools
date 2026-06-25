package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"fire-salamander-desktop/internal/ssh"
	"fire-salamander-desktop/internal/toolchain"
	"my_tools/libs/core/toolspec"
)

type ExecutionManager struct {
	state *SharedState
	task  *TaskResultManager
}

func NewExecutionManager(state *SharedState, task *TaskResultManager) *ExecutionManager {
	return &ExecutionManager{state: state, task: task}
}

// App delegates
func (a *App) StartLocalExecution(req ExecutionRequest) (*ExecutionTask, error) {
	return a.execution.StartLocalExecution(req)
}

func (a *App) StartRemoteExecution(req RemoteExecRequest) (*ExecutionTask, error) {
	return a.execution.StartRemoteExecution(req)
}

func (a *App) CancelExecution(taskID string) error {
	return a.execution.CancelExecution(taskID)
}

func (a *App) ListTasks() []*ExecutionTask {
	return a.execution.ListTasks()
}

func (a *App) registerExecutionTask(task *ExecutionTask) (context.Context, context.CancelFunc) {
	return a.execution.registerExecutionTask(task)
}

func (a *App) finishExecutionTask(taskID string, runCtx context.Context, execErr error, successMessage string, update func(task *ExecutionTask)) {
	a.execution.finishExecutionTask(taskID, runCtx, execErr, successMessage, update)
}

func (m *ExecutionManager) StartLocalExecution(req ExecutionRequest) (*ExecutionTask, error) {
	if err := ensureTooling(m.state); err != nil {
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
		toolName = pyTool.scriptName
	}

	task := &ExecutionTask{
		ID:        fmt.Sprintf("task_%d", time.Now().UnixNano()),
		ToolID:    req.ToolID,
		ToolName:  toolName,
		Status:    "running",
		Target:    "local",
		Args:      req.Args,
		PythonEnv: req.PythonEnv,
		Usage:     usage,
		StartedAt: time.Now().UnixMilli(),
	}

	runCtx, _ := m.registerExecutionTask(task)

	go func() {
		writer := &taskEventWriter{taskID: task.ID, task: m.task}
		var execErr error
		switch {
		case manifestOK && (manifest.Kind == toolspec.ToolKindGo || manifest.Kind == toolspec.ToolKindRust):
			execErr = executeLocalBinaryTool(runCtx, writer, manifest)(req.Args)
		case pyOK && pyTool != nil && pyTool.run != nil:
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
				execErr = pyTool.run(runCtx, pythonEnv, req.Args, writer)
			}
		default:
			execErr = fmt.Errorf("工具 %s 缺少可执行入口", task.ToolName)
		}
		writer.Flush()

		m.finishExecutionTask(task.ID, runCtx, execErr, "任务执行完成", nil)
	}()

	return task, nil
}

func (m *ExecutionManager) StartRemoteExecution(req RemoteExecRequest) (*ExecutionTask, error) {
	if err := ensureTooling(m.state); err != nil {
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

	task := &ExecutionTask{
		ID:           fmt.Sprintf("task_%d", time.Now().UnixNano()),
		ToolID:       req.ToolID,
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

	runCtx, cancel := m.registerExecutionTask(task)

	go func() {
		defer cancel()
		writer := &taskEventWriter{taskID: task.ID, task: m.task}
		defer writer.Flush()

		outcome, execErr := executeRemotely(runCtx, writer, remoteExecParams{
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

		m.finishExecutionTask(task.ID, runCtx, execErr, "远程任务执行完成", func(task *ExecutionTask) {
			task.remoteWorkDir = outcome.keepWorkDir
			task.RemoteResultStatus = outcome.result.Status
			task.RemoteResultPath = outcome.result.Path
			task.RemoteResultKind = outcome.result.Kind
			task.RemoteResultMessage = outcome.result.Message
		})
	}()

	return task, nil
}

func (m *ExecutionManager) registerExecutionTask(task *ExecutionTask) (context.Context, context.CancelFunc) {
	runCtx, cancel := context.WithCancel(context.Background())

	m.state.Mu.Lock()
	m.state.Tasks[task.ID] = task
	m.state.Cancels[task.ID] = cancel
	m.state.Mu.Unlock()

	m.task.emitTaskUpdate(task)
	return runCtx, cancel
}

func (m *ExecutionManager) finishExecutionTask(taskID string, runCtx context.Context, execErr error, successMessage string, update func(task *ExecutionTask)) {
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
		m.task.emitTaskLog(taskID, logMessage)
	}
	m.task.emitTaskUpdate(&copyTask)
}

func (m *ExecutionManager) CancelExecution(taskID string) error {
	m.state.Mu.RLock()
	cancel, ok := m.state.Cancels[taskID]
	m.state.Mu.RUnlock()
	if !ok {
		return fmt.Errorf("未找到正在运行的任务: %s", taskID)
	}
	cancel()
	return nil
}

func (m *ExecutionManager) ListTasks() []*ExecutionTask {
	m.state.Mu.RLock()
	defer m.state.Mu.RUnlock()

	tasks := make([]*ExecutionTask, 0, len(m.state.Tasks))
	for _, task := range m.state.Tasks {
		copyTask := *task
		tasks = append(tasks, &copyTask)
	}
	return tasks
}

// taskEventWriter - 定义在 execution_support.go 中
