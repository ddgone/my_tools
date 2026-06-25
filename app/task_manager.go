package main

import "fire-salamander-desktop/internal/taskresult"

func NewTaskResultManager(state *SharedState, dialog *DialogManager, exportMgr *ExportManager) *TaskResultManager {
	return taskresult.NewManager(state, dialog, exportMgr)
}

func (a *App) ListDownloadTasks() []*DownloadTask {
	return a.task.ListDownloadTasks()
}

func (a *App) StartTaskResultDownload(taskID string) (*DownloadTask, error) {
	return a.task.StartTaskResultDownload(taskID)
}

func (a *App) DownloadTaskResult(taskID string) (string, error) {
	return a.task.DownloadTaskResult(taskID)
}

func (a *App) emitDownloadTaskUpdate(task *DownloadTask) {
	a.task.EmitDownloadTaskUpdate(task)
}
