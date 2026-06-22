package main

import (
	"fmt"
	"strings"
	"time"

	"fire-salamander-desktop/internal/ssh"
	"fire-salamander-desktop/internal/toolchain"
	"my_tools/libs/core/toolspec"
)

type ExecutionRequest struct {
	ToolID    string `json:"toolId"`
	Args      string `json:"args"`
	PythonEnv string `json:"pythonEnv"`
}

type ExecutionTask struct {
	ID                         string `json:"id"`
	ToolID                     string `json:"toolId"`
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

	remoteWorkDir string
}

func (a *App) StartLocalExecution(req ExecutionRequest) (*ExecutionTask, error) {
	if err := a.ensureTooling(); err != nil {
		return nil, err
	}

	a.mu.RLock()
	legacyTool, ok := a.legacy[req.ToolID]
	manifest, manifestOK := a.manifests[req.ToolID]
	a.mu.RUnlock()

	if !ok && !manifestOK {
		return nil, fmt.Errorf("未找到工具: %s", req.ToolID)
	}

	usage := ""
	if ok && legacyTool != nil {
		usage = legacyTool.usage
	}
	if manifestOK && manifest.Docs.Usage != "" {
		usage = manifest.Docs.Usage
	}

	task := &ExecutionTask{
		ID:        fmt.Sprintf("task_%d", time.Now().UnixNano()),
		ToolID:    req.ToolID,
		ToolName:  manifest.Name,
		Status:    "running",
		Target:    "local",
		Args:      req.Args,
		PythonEnv: req.PythonEnv,
		Usage:     usage,
		StartedAt: time.Now().UnixMilli(),
	}

	if task.ToolName == "" {
		switch {
		case ok && legacyTool != nil && legacyTool.tool != nil:
			task.ToolName = legacyTool.tool.Name()
		case manifestOK:
			task.ToolName = manifest.Name
		}
	}

	runCtx, _ := a.registerExecutionTask(task)

	go func() {
		writer := &taskEventWriter{taskID: task.ID, app: a}
		var execErr error
		switch {
		case manifestOK && manifest.Kind == toolspec.ToolKindRust:
			execErr = executeLocalRustTool(runCtx, writer, manifest)(req.Args)
		case legacyTool != nil && legacyTool.runPython != nil:
			pythonEnv := strings.TrimSpace(req.PythonEnv)
			if pythonEnv == "" {
				pythonEnv, execErr = toolchain.ResolvePythonBinaryForTool(req.ToolID)
				if execErr == nil {
					a.mu.Lock()
					task.PythonEnv = pythonEnv
					a.mu.Unlock()
				}
			}
			if execErr == nil {
				execErr = legacyTool.runPython(runCtx, pythonEnv, req.Args, writer)
			}
		case legacyTool != nil && legacyTool.run != nil:
			execErr = legacyTool.run(runCtx, req.Args, writer)
		default:
			execErr = fmt.Errorf("工具 %s 缺少可执行入口", task.ToolName)
		}
		writer.Flush()

		a.finishExecutionTask(task.ID, runCtx, execErr, "任务执行完成", nil)
	}()

	return task, nil
}

func (a *App) StartRemoteExecution(req RemoteExecRequest) (*ExecutionTask, error) {
	if err := a.ensureTooling(); err != nil {
		return nil, err
	}

	a.mu.RLock()
	_, ok := a.legacy[req.ToolID]
	manifest, manifestOK := a.manifests[req.ToolID]
	a.mu.RUnlock()

	if !ok && !manifestOK {
		return nil, fmt.Errorf("未找到工具: %s", req.ToolID)
	}

	conns := a.sshStore.List()
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

	realConn, err := a.sshStore.GetCredentials(req.ConnID)
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

	runCtx, cancel := a.registerExecutionTask(task)

	go func() {
		defer cancel()
		writer := &taskEventWriter{taskID: task.ID, app: a}
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

		a.finishExecutionTask(task.ID, runCtx, execErr, "远程任务执行完成", func(task *ExecutionTask) {
			task.remoteWorkDir = outcome.keepWorkDir
			task.RemoteResultStatus = outcome.result.Status
			task.RemoteResultPath = outcome.result.Path
			task.RemoteResultKind = outcome.result.Kind
			task.RemoteResultMessage = outcome.result.Message
		})
	}()

	return task, nil
}
