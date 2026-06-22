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

func (a *App) registerExecutionTask(task *ExecutionTask) (context.Context, context.CancelFunc) {
	runCtx, cancel := context.WithCancel(context.Background())

	a.mu.Lock()
	a.tasks[task.ID] = task
	a.cancels[task.ID] = cancel
	a.mu.Unlock()

	a.emitTaskUpdate(task)
	return runCtx, cancel
}

func (a *App) finishExecutionTask(taskID string, runCtx context.Context, execErr error, successMessage string, update func(task *ExecutionTask)) {
	a.mu.Lock()
	task, ok := a.tasks[taskID]
	if !ok {
		a.mu.Unlock()
		return
	}

	delete(a.cancels, taskID)
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
	a.mu.Unlock()

	if logMessage != "" {
		a.emitTaskLog(taskID, logMessage)
	}
	a.emitTaskUpdate(&copyTask)
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
