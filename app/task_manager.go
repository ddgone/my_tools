package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"fire-salamander-desktop/internal/runtime"
	"fire-salamander-desktop/internal/runtimeenv"
	"fire-salamander-desktop/internal/ssh"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type TaskResultManager struct {
	state  *SharedState
	dialog *DialogManager
	export *ExportManager
}

func NewTaskResultManager(state *SharedState, dialog *DialogManager, export *ExportManager) *TaskResultManager {
	return &TaskResultManager{state: state, dialog: dialog, export: export}
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
	a.task.emitDownloadTaskUpdate(task)
}

func (m *TaskResultManager) ListDownloadTasks() []*DownloadTask {
	m.state.Mu.RLock()
	defer m.state.Mu.RUnlock()

	tasks := make([]*DownloadTask, 0, len(m.state.DownloadTasks))
	for _, task := range m.state.DownloadTasks {
		copyTask := *task
		tasks = append(tasks, &copyTask)
	}
	return tasks
}

func (m *TaskResultManager) emitDownloadTaskUpdate(task *DownloadTask) {
	if task == nil || m.state.Ctx == nil {
		return
	}
	copyTask := *task
	wailsruntime.EventsEmit(m.state.Ctx, "download:task:update", copyTask)
}

func (m *TaskResultManager) emitTaskLog(taskID string, message string) {
	if strings.TrimSpace(message) == "" || m.state.Ctx == nil {
		return
	}
	wailsruntime.EventsEmit(m.state.Ctx, "task:log", TaskLogEvent{
		TaskID:   taskID,
		Message:  message,
		Recorded: time.Now().UnixMilli(),
	})
}

func (m *TaskResultManager) emitTaskUpdate(task *ExecutionTask) {
	if m.state.Ctx == nil || task == nil {
		return
	}
	copyTask := *task
	wailsruntime.EventsEmit(m.state.Ctx, "task:update", copyTask)
}

func (m *TaskResultManager) StartTaskResultDownload(taskID string) (*DownloadTask, error) {
	execTask, selectedPath, err := m.prepareTaskResultDownload(taskID)
	if err != nil || strings.TrimSpace(selectedPath) == "" {
		return nil, err
	}

	downloadTask := &DownloadTask{
		ID:               fmt.Sprintf("download_%d", time.Now().UnixNano()),
		SourceTaskID:     execTask.ID,
		ToolID:           execTask.ToolID,
		ToolName:         execTask.ToolName,
		Status:           "running",
		RemoteResultPath: execTask.RemoteResultPath,
		RemoteResultKind: execTask.RemoteResultKind,
		LocalPath:        selectedPath,
		Directory:        filepath.Dir(selectedPath),
		Message:          initialDownloadTaskMessage(execTask.RemoteResultKind),
		StartedAt:        time.Now().UnixMilli(),
	}

	m.state.Mu.Lock()
	m.state.DownloadTasks[downloadTask.ID] = downloadTask
	m.state.Mu.Unlock()
	m.emitDownloadTaskUpdate(downloadTask)

	go m.runDownloadTask(downloadTask.ID, execTask, selectedPath)
	return downloadTask, nil
}

func (m *TaskResultManager) DownloadTaskResult(taskID string) (string, error) {
	execTask, selectedPath, err := m.prepareTaskResultDownload(taskID)
	if err != nil || strings.TrimSpace(selectedPath) == "" {
		return "", err
	}
	if err := m.downloadTaskResultToPath(execTask, selectedPath, nil); err != nil {
		return "", err
	}
	return selectedPath, nil
}

func (m *TaskResultManager) prepareTaskResultDownload(taskID string) (*ExecutionTask, string, error) {
	m.state.Mu.RLock()
	task, ok := m.state.Tasks[taskID]
	if !ok {
		m.state.Mu.RUnlock()
		return nil, "", fmt.Errorf("未找到任务: %s", taskID)
	}
	copyTask := *task
	m.state.Mu.RUnlock()

	if strings.TrimSpace(copyTask.RemoteConnID) == "" {
		return nil, "", fmt.Errorf("当前任务不是可下载结果的远程任务")
	}
	if copyTask.RemoteResultStatus != "available" || strings.TrimSpace(copyTask.RemoteResultPath) == "" {
		return nil, "", fmt.Errorf("当前任务没有可下载结果")
	}

	layout, err := runtimeenv.ResolveLayout()
	if err != nil {
		return nil, "", fmt.Errorf("初始化运行时目录失败: %w", err)
	}
	if err := layout.Ensure(); err != nil {
		return nil, "", fmt.Errorf("准备运行时目录失败: %w", err)
	}

	defaultDir, err := m.export.loadLastExportDirectory()
	if err != nil {
		return nil, "", err
	}
	if strings.TrimSpace(defaultDir) == "" {
		defaultDir = layout.ExportsDir()
	}

	selectedPath, err := m.dialog.OpenSaveFileDialog(FileDialogRequest{
		Title:            "下载结果",
		FilterName:       resultDownloadFilterName(copyTask.RemoteResultKind),
		FilterGlob:       resultDownloadFilterGlob(copyTask.RemoteResultKind),
		Directory:        false,
		DefaultDirectory: defaultDir,
		DefaultFilename:  buildResultDownloadFileName(copyTask.ToolName, copyTask.RemoteResultPath, copyTask.RemoteResultKind),
	})
	if err != nil {
		return nil, "", err
	}
	selectedPath = finalizeResultDownloadPath(selectedPath, copyTask.RemoteResultKind)
	if strings.TrimSpace(selectedPath) == "" {
		return &copyTask, "", nil
	}
	if err := os.MkdirAll(filepath.Dir(selectedPath), 0755); err != nil {
		return nil, "", fmt.Errorf("创建下载目录失败: %w", err)
	}
	return &copyTask, selectedPath, nil
}

func (m *TaskResultManager) runDownloadTask(downloadTaskID string, execTask *ExecutionTask, selectedPath string) {
	err := m.downloadTaskResultToPath(execTask, selectedPath, func(downloaded int64, total int64) {
		m.state.Mu.Lock()
		task, ok := m.state.DownloadTasks[downloadTaskID]
		if !ok {
			m.state.Mu.Unlock()
			return
		}
		task.DownloadedBytes = downloaded
		task.TotalBytes = total
		task.ProgressPercent = calculateDownloadProgress(downloaded, total)
		task.Message = buildDownloadTaskMessage(downloaded, total)
		updated := *task
		m.state.Mu.Unlock()
		m.emitDownloadTaskUpdate(&updated)
	})

	m.state.Mu.Lock()
	task, ok := m.state.DownloadTasks[downloadTaskID]
	if !ok {
		m.state.Mu.Unlock()
		return
	}
	task.EndedAt = time.Now().UnixMilli()
	if err != nil {
		task.Status = "error"
		task.Message = err.Error()
	} else {
		task.Status = "success"
		task.ProgressPercent = 100
		task.Message = "下载完成"
	}
	updated := *task
	m.state.Mu.Unlock()

	m.emitDownloadTaskUpdate(&updated)
}

func (m *TaskResultManager) downloadTaskResultToPath(task *ExecutionTask, selectedPath string, onProgress func(downloaded int64, total int64)) error {
	conn, err := m.state.SSHStore.GetCredentials(task.RemoteConnID)
	if err != nil {
		return fmt.Errorf("SSH连接凭据已失效: %w", err)
	}

	verifier := ssh.NewHostKeyVerifier(conn.HostKeyFingerprint)
	executor, err := runtime.DialRemote(conn.Host, conn.Port, conn.User, conn.Password, conn.KeyPath, verifier.Callback())
	if err != nil {
		return fmt.Errorf("SSH连接失败: %w", err)
	}
	defer executor.Close()

	ctx := context.Background()
	totalBytes := int64(0)
	switch task.RemoteResultKind {
	case "directory":
		remoteArchivePath := path.Join("/tmp", buildRemoteArchiveFileName(task.ID, task.ToolID, task.RemoteResultPath))
		if err := executor.DownloadDirectoryTarGzViaTempArchive(ctx, task.RemoteResultPath, selectedPath, remoteArchivePath, func(downloaded int64, total int64) {
			totalBytes = total
			if onProgress != nil {
				onProgress(downloaded, total)
			}
		}); err != nil {
			return fmt.Errorf("下载结果目录失败: %w", err)
		}
	default:
		totalBytes, _ = executor.GetFileSize(ctx, task.RemoteResultPath)
		if onProgress != nil {
			onProgress(0, totalBytes)
		}
		if err := executor.DownloadFileWithProgress(ctx, task.RemoteResultPath, selectedPath, func(downloaded int64) {
			if onProgress != nil {
				onProgress(downloaded, totalBytes)
			}
		}); err != nil {
			return fmt.Errorf("下载结果文件失败: %w", err)
		}
	}

	if err := m.export.saveLastExportDirectory(filepath.Dir(selectedPath)); err != nil {
		return err
	}
	if onProgress != nil {
		onProgress(totalBytes, totalBytes)
	}

	m.state.Mu.Lock()
	if current, ok := m.state.Tasks[task.ID]; ok {
		current.RemoteResultDownloadedPath = selectedPath
		m.emitTaskUpdate(current)
	}
	m.state.Mu.Unlock()
	return nil
}
