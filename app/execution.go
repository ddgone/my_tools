package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"fire-salamander-desktop/internal/builder"
	"fire-salamander-desktop/internal/runtime"
	"fire-salamander-desktop/internal/ssh"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type ExecutionRequest struct {
	ToolID    string `json:"toolId"`
	Args      string `json:"args"`
	PythonEnv string `json:"pythonEnv"`
}

type ExecutionTask struct {
	ID          string `json:"id"`
	ToolID      string `json:"toolId"`
	ToolName    string `json:"toolName"`
	Status      string `json:"status"`
	Target      string `json:"target"`
	Args        string `json:"args"`
	PythonEnv   string `json:"pythonEnv,omitempty"`
	Usage       string `json:"usage"`
	StartedAt   int64  `json:"startedAt"`
	EndedAt     int64  `json:"endedAt,omitempty"`
	ExitMessage string `json:"exitMessage,omitempty"`
}

type TaskLogEvent struct {
	TaskID   string `json:"taskId"`
	Message  string `json:"message"`
	Recorded int64  `json:"recorded"`
}

type taskEventWriter struct {
	taskID string
	app    *App
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (w *taskEventWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buffer.Write(p)
	for {
		line, err := w.buffer.ReadString('\n')
		if err != nil {
			w.buffer.WriteString(line)
			break
		}
		w.app.emitTaskLog(w.taskID, strings.TrimRight(line, "\r\n"))
	}

	return len(p), nil
}

func (w *taskEventWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.buffer.Len() == 0 {
		return
	}
	w.app.emitTaskLog(w.taskID, strings.TrimRight(w.buffer.String(), "\r\n"))
	w.buffer.Reset()
}

func (a *App) StartLocalExecution(req ExecutionRequest) (*ExecutionTask, error) {
	if err := a.ensureTooling(); err != nil {
		return nil, err
	}

	a.mu.RLock()
	legacyTool, ok := a.legacy[req.ToolID]
	manifest, manifestOK := a.manifests[req.ToolID]
	a.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("未找到工具: %s", req.ToolID)
	}

	usage := legacyTool.usage
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
		task.ToolName = legacyTool.tool.Name()
	}

	runCtx, cancel := context.WithCancel(context.Background())

	a.mu.Lock()
	a.tasks[task.ID] = task
	a.cancels[task.ID] = cancel
	a.mu.Unlock()

	a.emitTaskUpdate(task)

	go func() {
		writer := &taskEventWriter{taskID: task.ID, app: a}
		var execErr error
		if legacyTool.runPython != nil {
			pythonEnv := strings.TrimSpace(req.PythonEnv)
			if pythonEnv == "" {
				pythonEnv = "python"
			}
			execErr = legacyTool.runPython(runCtx, pythonEnv, req.Args, writer)
		} else if legacyTool.run != nil {
			execErr = legacyTool.run(runCtx, req.Args, writer)
		} else {
			execErr = fmt.Errorf("工具 %s 缺少可执行入口", task.ToolName)
		}
		writer.Flush()

		a.mu.Lock()
		defer a.mu.Unlock()

		delete(a.cancels, task.ID)
		task.EndedAt = time.Now().UnixMilli()
		switch {
		case runCtx.Err() != nil:
			task.Status = "canceled"
			task.ExitMessage = "任务已取消"
		case execErr != nil:
			task.Status = "error"
			task.ExitMessage = execErr.Error()
			a.emitTaskLog(task.ID, "[错误] "+execErr.Error())
		default:
			task.Status = "success"
			task.ExitMessage = "任务执行完成"
		}
		a.emitTaskUpdate(task)
	}()

	return task, nil
}

func (a *App) CancelExecution(taskID string) error {
	a.mu.RLock()
	cancel, ok := a.cancels[taskID]
	a.mu.RUnlock()
	if !ok {
		return fmt.Errorf("未找到正在运行的任务: %s", taskID)
	}
	cancel()
	return nil
}

func (a *App) ListTasks() []*ExecutionTask {
	a.mu.RLock()
	defer a.mu.RUnlock()

	tasks := make([]*ExecutionTask, 0, len(a.tasks))
	for _, task := range a.tasks {
		copyTask := *task
		tasks = append(tasks, &copyTask)
	}
	return tasks
}

func (a *App) emitTaskLog(taskID string, message string) {
	if strings.TrimSpace(message) == "" || a.ctx == nil {
		return
	}
	wailsruntime.EventsEmit(a.ctx, "task:log", TaskLogEvent{
		TaskID:   taskID,
		Message:  message,
		Recorded: time.Now().UnixMilli(),
	})
}

func (a *App) emitTaskUpdate(task *ExecutionTask) {
	if a.ctx == nil || task == nil {
		return
	}
	copyTask := *task
	wailsruntime.EventsEmit(a.ctx, "task:update", copyTask)
}

type RemoteExecRequest struct {
	ToolID    string `json:"toolId"`
	ConnID    string `json:"connId"`
	Args      string `json:"args"`
	PythonEnv string `json:"pythonEnv"`
}

func (a *App) StartRemoteExecution(req RemoteExecRequest) (*ExecutionTask, error) {
	if err := a.ensureTooling(); err != nil {
		return nil, err
	}

	a.mu.RLock()
	_, ok := a.legacy[req.ToolID]
	manifest, manifestOK := a.manifests[req.ToolID]
	a.mu.RUnlock()

	if !ok {
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
		ID:        fmt.Sprintf("task_%d", time.Now().UnixNano()),
		ToolID:    req.ToolID,
		ToolName:  manifest.Name,
		Status:    "running",
		Target:    "remote:" + target.Host,
		Args:      req.Args,
		PythonEnv: req.PythonEnv,
		StartedAt: time.Now().UnixMilli(),
	}

	if task.ToolName == "" && manifestOK {
		task.ToolName = manifest.Name
	}

	runCtx, cancel := context.WithCancel(context.Background())

	a.mu.Lock()
	a.tasks[task.ID] = task
	a.cancels[task.ID] = cancel
	a.mu.Unlock()

	a.emitTaskUpdate(task)

	go func() {
		defer cancel()
		writer := &taskEventWriter{taskID: task.ID, app: a}
		defer writer.Flush()

		fmt.Fprintf(writer, "[远程] 正在为目标 %s 准备产物...\n", task.ToolName)

		tmpDir, err := os.MkdirTemp("", "fire-salamander-build-*")
		if err != nil {
			a.emitTaskLog(task.ID, "[错误] 创建临时目录失败: "+err.Error())
			a.mu.Lock()
			task.Status = "error"
			task.ExitMessage = err.Error()
			task.EndedAt = time.Now().UnixMilli()
			a.mu.Unlock()
			a.emitTaskUpdate(task)
			return
		}
		defer os.RemoveAll(tmpDir)

		kind := builder.KindGo
		if string(manifest.Kind) == "python" {
			kind = builder.KindPython
		}

		pkgPath, err := builder.BuildPackage(builder.BuildRequest{
			ToolID:    req.ToolID,
			ToolName:  manifest.Name,
			Kind:      kind,
			OutputDir: tmpDir,
		})
		if err != nil {
			fmt.Fprintf(writer, "[远程] 构建产物失败: %v\n", err)
		} else {
			fmt.Fprintf(writer, "[远程] 产物已就绪: %s\n", pkgPath)
		}

		execErr := executeRemotely(runCtx, writer, remoteExecParams{
			host:      target.Host,
			port:      target.Port,
			user:      target.User,
			password:  realConn.Password,
			toolID:    req.ToolID,
			kind:      string(manifest.Kind),
			args:      req.Args,
			pythonEnv: req.PythonEnv,
		})

		a.mu.Lock()
		defer a.mu.Unlock()

		task.EndedAt = time.Now().UnixMilli()
		switch {
		case runCtx.Err() != nil:
			task.Status = "canceled"
			task.ExitMessage = "任务已取消"
		case execErr != nil:
			task.Status = "error"
			task.ExitMessage = execErr.Error()
			a.emitTaskLog(task.ID, "[错误] "+execErr.Error())
		default:
			task.Status = "success"
			task.ExitMessage = "远程任务执行完成"
		}
		a.emitTaskUpdate(task)
	}()

	return task, nil
}

type remoteExecParams struct {
	host, user, password string
	port                 int
	toolID, kind, args   string
	pythonEnv            string
}

func executeRemotely(ctx context.Context, writer io.Writer, params remoteExecParams) error {
	fmt.Fprintf(writer, "[远程] 正在连接 %s@%s:%d ...\n", params.user, params.host, params.port)

	executor, err := runtime.DialRemote(params.host, params.port, params.user, params.password)
	if err != nil {
		return fmt.Errorf("SSH连接失败: %w", err)
	}
	defer executor.Close()

	fmt.Fprintf(writer, "[远程] 连接成功\n")

	var cmd string
	if params.kind == "python" {
		env := params.pythonEnv
		if env == "" {
			env = "python3"
		}
		argPart := strings.TrimSpace(params.args)
		if argPart != "" {
			cmd = fmt.Sprintf("%s %s", env, argPart)
		} else {
			cmd = fmt.Sprintf("%s -c \"print('远程Python环境就绪')\"", env)
		}
	} else {
		argPart := strings.TrimSpace(params.args)
		if argPart != "" {
			cmd = fmt.Sprintf("echo 'Go远程执行: %s' && %s", params.toolID, argPart)
		} else {
			cmd = fmt.Sprintf("echo 'Go远程执行环境就绪: %s'", params.toolID)
		}
	}

	fmt.Fprintf(writer, "[远程] 执行: %s\n", cmd)
	return executor.Execute(ctx, cmd, writer)
}
