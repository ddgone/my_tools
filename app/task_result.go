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
	"my_tools/libs/core/toolspec"
	"my_tools/libs/framework"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type remoteResultHint struct {
	Path string
	Kind string
}

type DownloadTask struct {
	ID               string  `json:"id"`
	SourceTaskID     string  `json:"sourceTaskId"`
	ToolID           string  `json:"toolId"`
	ToolName         string  `json:"toolName"`
	Status           string  `json:"status"`
	RemoteResultPath string  `json:"remoteResultPath"`
	RemoteResultKind string  `json:"remoteResultKind"`
	LocalPath        string  `json:"localPath,omitempty"`
	Directory        string  `json:"directory,omitempty"`
	Message          string  `json:"message,omitempty"`
	DownloadedBytes  int64   `json:"downloadedBytes"`
	TotalBytes       int64   `json:"totalBytes"`
	ProgressPercent  float64 `json:"progressPercent"`
	StartedAt        int64   `json:"startedAt"`
	EndedAt          int64   `json:"endedAt,omitempty"`
}

func resolveRemoteResultHint(params []toolspec.ParameterSpec, rawArgs string, remoteWorkDir string) (remoteResultHint, error) {
	outputParam, ok := findLikelyOutputParam(params)
	if !ok {
		return remoteResultHint{}, nil
	}

	parsedArgs, err := framework.ParseArgs(rawArgs)
	if err != nil {
		return remoteResultHint{}, err
	}
	value, ok := extractParamValue(parsedArgs, outputParam)
	if !ok || strings.TrimSpace(value) == "" {
		return remoteResultHint{}, nil
	}

	kind := "directory"
	if strings.TrimSpace(outputParam.PathMode) == "file" {
		kind = "file"
	}
	return remoteResultHint{
		Path: resolveRemotePath(value, remoteWorkDir),
		Kind: kind,
	}, nil
}

func findLikelyOutputParam(params []toolspec.ParameterSpec) (toolspec.ParameterSpec, bool) {
	for _, param := range params {
		if param.Type != toolspec.FieldTypePath {
			continue
		}
		argKey := strings.TrimSpace(param.ArgKey)
		key := strings.TrimSpace(param.Key)
		switch {
		case argKey == "output":
			return param, true
		case key == "output", key == "outputDir":
			return param, true
		}
	}
	return toolspec.ParameterSpec{}, false
}

func extractParamValue(parsedArgs []string, param toolspec.ParameterSpec) (string, bool) {
	candidates := make([]string, 0, 4)
	if key := strings.TrimSpace(param.ArgKey); key != "" {
		candidates = append(candidates, "-"+key, "--"+key)
	}
	if key := strings.TrimSpace(param.Key); key != "" && key != param.ArgKey {
		candidates = append(candidates, "-"+key, "--"+key)
	}

	for i := 0; i < len(parsedArgs); i++ {
		token := parsedArgs[i]
		if !containsString(candidates, token) {
			continue
		}
		if i+1 >= len(parsedArgs) {
			return "", false
		}
		next := parsedArgs[i+1]
		if strings.HasPrefix(next, "-") {
			return "", false
		}
		return next, true
	}
	return "", false
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func resolveRemotePath(value string, remoteWorkDir string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if path.IsAbs(trimmed) {
		return path.Clean(trimmed)
	}
	return path.Clean(path.Join(strings.TrimSpace(remoteWorkDir), trimmed))
}

func probeRemoteResult(ctx context.Context, executor *runtime.RemoteExecutor, remotePath string) (remoteResultProbe, error) {
	kind, exists, err := executor.DetectPathKind(ctx, remotePath)
	if err != nil {
		return remoteResultProbe{}, err
	}
	if !exists {
		return remoteResultProbe{
			Status:  "missing",
			Path:    remotePath,
			Message: "未发现可下载结果",
		}, nil
	}

	message := "已探测到可下载结果"
	if kind == "directory" {
		message = "已探测到可下载的输出目录"
	} else if kind == "file" {
		message = "已探测到可下载的输出文件"
	}
	return remoteResultProbe{
		Status:  "available",
		Path:    remotePath,
		Kind:    kind,
		Message: message,
	}, nil
}

func pathWithinRemoteBase(base string, target string) bool {
	cleanBase := path.Clean(strings.TrimSpace(base))
	cleanTarget := path.Clean(strings.TrimSpace(target))
	if cleanBase == "" || cleanTarget == "" {
		return false
	}
	return cleanTarget == cleanBase || strings.HasPrefix(cleanTarget, cleanBase+"/")
}

func (a *App) ListDownloadTasks() []*DownloadTask {
	a.mu.RLock()
	defer a.mu.RUnlock()

	tasks := make([]*DownloadTask, 0, len(a.downloadTasks))
	for _, task := range a.downloadTasks {
		copyTask := *task
		tasks = append(tasks, &copyTask)
	}
	return tasks
}

func (a *App) emitDownloadTaskUpdate(task *DownloadTask) {
	if task == nil || a.ctx == nil {
		return
	}
	copyTask := *task
	wailsruntime.EventsEmit(a.ctx, "download:task:update", copyTask)
}

func (a *App) StartTaskResultDownload(taskID string) (*DownloadTask, error) {
	execTask, selectedPath, err := a.prepareTaskResultDownload(taskID)
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

	a.mu.Lock()
	a.downloadTasks[downloadTask.ID] = downloadTask
	a.mu.Unlock()
	a.emitDownloadTaskUpdate(downloadTask)

	go a.runDownloadTask(downloadTask.ID, execTask, selectedPath)
	return downloadTask, nil
}

func (a *App) DownloadTaskResult(taskID string) (string, error) {
	execTask, selectedPath, err := a.prepareTaskResultDownload(taskID)
	if err != nil || strings.TrimSpace(selectedPath) == "" {
		return "", err
	}
	if err := a.downloadTaskResultToPath(execTask, selectedPath, nil); err != nil {
		return "", err
	}
	return selectedPath, nil
}

func (a *App) prepareTaskResultDownload(taskID string) (*ExecutionTask, string, error) {
	a.mu.RLock()
	task, ok := a.tasks[taskID]
	if !ok {
		a.mu.RUnlock()
		return nil, "", fmt.Errorf("未找到任务: %s", taskID)
	}
	copyTask := *task
	a.mu.RUnlock()

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

	defaultDir, err := a.loadLastExportDirectory()
	if err != nil {
		return nil, "", err
	}
	if strings.TrimSpace(defaultDir) == "" {
		defaultDir = layout.ExportsDir()
	}

	selectedPath, err := a.OpenSaveFileDialog(FileDialogRequest{
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

func (a *App) runDownloadTask(downloadTaskID string, execTask *ExecutionTask, selectedPath string) {
	err := a.downloadTaskResultToPath(execTask, selectedPath, func(downloaded int64, total int64) {
		a.mu.Lock()
		task, ok := a.downloadTasks[downloadTaskID]
		if !ok {
			a.mu.Unlock()
			return
		}
		task.DownloadedBytes = downloaded
		task.TotalBytes = total
		task.ProgressPercent = calculateDownloadProgress(downloaded, total)
		task.Message = buildDownloadTaskMessage(downloaded, total)
		updated := *task
		a.mu.Unlock()
		a.emitDownloadTaskUpdate(&updated)
	})

	a.mu.Lock()
	task, ok := a.downloadTasks[downloadTaskID]
	if !ok {
		a.mu.Unlock()
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
	a.mu.Unlock()

	a.emitDownloadTaskUpdate(&updated)
}

func calculateDownloadProgress(downloaded int64, total int64) float64 {
	if total <= 0 {
		if downloaded > 0 {
			return 8
		}
		return 0
	}
	percent := (float64(downloaded) / float64(total)) * 100
	if percent < 0 {
		return 0
	}
	if percent > 100 {
		return 100
	}
	return percent
}

func buildDownloadTaskMessage(downloaded int64, total int64) string {
	if total <= 0 {
		return fmt.Sprintf("%s / 正在估算大小", formatByteCount(downloaded))
	}
	return fmt.Sprintf("%s / %s", formatByteCount(downloaded), formatByteCount(total))
}

func initialDownloadTaskMessage(remoteResultKind string) string {
	if remoteResultKind == "directory" {
		return "正在准备下载归档"
	}
	return "正在下载输出结果"
}

func formatByteCount(value int64) string {
	if value < 1024 {
		return fmt.Sprintf("%d B", value)
	}
	const unit = 1024
	div, exp := int64(unit), 0
	for n := value / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(value)/float64(div), "KMGTPE"[exp])
}

func (a *App) downloadTaskResultToPath(task *ExecutionTask, selectedPath string, onProgress func(downloaded int64, total int64)) error {
	conn, err := a.sshStore.GetCredentials(task.RemoteConnID)
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

	if err := a.saveLastExportDirectory(filepath.Dir(selectedPath)); err != nil {
		return err
	}
	if onProgress != nil {
		onProgress(totalBytes, totalBytes)
	}

	a.mu.Lock()
	if current, ok := a.tasks[task.ID]; ok {
		current.RemoteResultDownloadedPath = selectedPath
		a.emitTaskUpdate(current)
	}
	a.mu.Unlock()
	return nil
}

func buildRemoteArchiveFileName(taskID string, toolID string, remoteResultPath string) string {
	base := sanitizeRemoteName(strings.TrimSpace(toolID))
	if base == "" {
		base = sanitizeRemoteName(path.Base(strings.TrimSpace(remoteResultPath)))
	}
	if base == "" {
		base = "result"
	}
	suffix := sanitizeRemoteName(strings.TrimSpace(taskID))
	if suffix == "" {
		suffix = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("fire-salamander-%s-%s.tar.gz", base, suffix)
}

func sanitizeRemoteName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	sanitized := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-', r == '_', r == '.':
			return r
		default:
			return '-'
		}
	}, value)
	return strings.Trim(sanitized, "-.")
}

func buildResultDownloadFileName(toolName string, remoteResultPath string, remoteResultKind string) string {
	base := path.Base(strings.TrimSpace(remoteResultPath))
	base = strings.TrimSpace(base)
	if base == "" || base == "." || base == "/" {
		base = sanitizeExportBaseName(toolName)
	}
	if base == "" {
		base = "result"
	}
	if remoteResultKind == "directory" && !strings.HasSuffix(strings.ToLower(base), ".tar.gz") {
		base += ".tar.gz"
	}
	return base
}

func finalizeResultDownloadPath(selectedPath string, remoteResultKind string) string {
	trimmed := strings.TrimSpace(selectedPath)
	if trimmed == "" {
		return ""
	}
	if remoteResultKind == "directory" && !strings.HasSuffix(strings.ToLower(trimmed), ".tar.gz") {
		return trimmed + ".tar.gz"
	}
	return trimmed
}

func resultDownloadFilterName(remoteResultKind string) string {
	if remoteResultKind == "directory" {
		return "Tar Gzip 归档"
	}
	return "所有文件"
}

func resultDownloadFilterGlob(remoteResultKind string) string {
	if remoteResultKind == "directory" {
		return "*.tar.gz"
	}
	return "*.*"
}
