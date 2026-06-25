package main

import "context"

// ExecutionManager delegates live in the execution package.
// App wrappers forward to the Manager instance stored on App.
// The Manager takes a shared.TaskEventEmitter, which TaskResultManager implements.

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
	return a.execution.RegisterExecutionTask(task)
}

func (a *App) finishExecutionTask(taskID string, runCtx context.Context, execErr error, successMessage string, update func(task *ExecutionTask)) {
	a.execution.FinishExecutionTask(taskID, runCtx, execErr, successMessage, update)
}
