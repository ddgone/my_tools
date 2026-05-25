package main

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

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
