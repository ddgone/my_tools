package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fire-salamander-desktop/internal/builder"
	"fire-salamander-desktop/internal/runtime"
	"fire-salamander-desktop/internal/runtimeenv"
	"fire-salamander-desktop/internal/ssh"
	"fire-salamander-desktop/internal/toolchain"
	"my_tools/libs/core/toolspec"
	"my_tools/libs/framework"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
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

	runCtx, cancel := context.WithCancel(context.Background())

	a.mu.Lock()
	a.tasks[task.ID] = task
	a.cancels[task.ID] = cancel
	a.mu.Unlock()

	a.emitTaskUpdate(task)

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
			task.ExitMessage = "远程任务执行完成"
		}
		task.remoteWorkDir = outcome.keepWorkDir
		task.RemoteResultStatus = outcome.result.Status
		task.RemoteResultPath = outcome.result.Path
		task.RemoteResultKind = outcome.result.Kind
		task.RemoteResultMessage = outcome.result.Message
		a.emitTaskUpdate(task)
	}()

	return task, nil
}

type remoteExecParams struct {
	host, user, password string
	keyPath              string
	hostKeyFingerprint   string
	port                 int
	toolID, kind, args   string
	taskID, toolName     string
	pythonEnv            string
	sourceEntry          string
	manifestParams       []toolspec.ParameterSpec
}

type remoteResultProbe struct {
	Status  string
	Path    string
	Kind    string
	Message string
}

type remoteExecutionOutcome struct {
	result      remoteResultProbe
	keepWorkDir string
}

func executeRemotely(ctx context.Context, writer io.Writer, params remoteExecParams) (remoteExecutionOutcome, error) {
	var outcome remoteExecutionOutcome
	fmt.Fprintf(writer, "[远程] 正在连接 %s@%s:%d ...\n", params.user, params.host, params.port)

	verifier := ssh.NewHostKeyVerifier(params.hostKeyFingerprint)
	executor, err := runtime.DialRemote(params.host, params.port, params.user, params.password, params.keyPath, verifier.Callback())
	if err != nil {
		return outcome, fmt.Errorf("SSH连接失败: %w", err)
	}
	defer executor.Close()

	fmt.Fprintf(writer, "[远程] 连接成功\n")

	platform, err := executor.DetectPlatform(ctx)
	if err != nil {
		return outcome, err
	}
	fmt.Fprintf(writer, "[远程] 目标平台: %s/%s\n", platform.OS, platform.Arch)

	layout, err := runtimeenv.ResolveLayout()
	if err != nil {
		return outcome, fmt.Errorf("初始化运行时目录失败: %w", err)
	}
	if err := layout.Ensure(); err != nil {
		return outcome, fmt.Errorf("准备运行时目录失败: %w", err)
	}

	repoRoot, ok := locateRepoRoot()
	if !ok {
		return outcome, fmt.Errorf("当前运行环境缺少源码工作区，暂时无法构建单工具远程产物")
	}

	fmt.Fprintf(writer, "[远程] 正在为目标 %s 准备产物...\n", params.toolName)
	buildResult, err := builder.BuildPackage(builder.BuildRequest{
		ToolID:           params.toolID,
		ToolName:         params.toolName,
		Kind:             builderKind(params.kind),
		OutputDir:        layout.BuildCacheDir(),
		CacheDir:         layout.BuildCacheDir(),
		RepoRoot:         repoRoot,
		SourceEntry:      params.sourceEntry,
		TargetOS:         platform.OS,
		TargetArch:       platform.Arch,
		UseCacheAsOutput: true,
		Progress:         writer,
	})
	if err != nil {
		return outcome, err
	}
	pkgPath := buildResult.Path
	fmt.Fprintf(writer, "[远程] 产物已就绪: %s\n", pkgPath)

	remoteDir := buildRemoteWorkDir(params.taskID)
	if err := executor.Execute(ctx, fmt.Sprintf("mkdir -p %s", runtime.ShellQuote(remoteDir)), writer); err != nil {
		return outcome, fmt.Errorf("创建远端临时目录失败: %w", err)
	}
	fmt.Fprintf(writer, "[远程] 远端工作目录: %s\n", remoteDir)
	keepRemoteDir := false
	defer func() {
		if keepRemoteDir {
			fmt.Fprintf(writer, "[远程] 已保留远端工作目录，供后续下载结果使用: %s\n", remoteDir)
			return
		}
		cleanupCmd := fmt.Sprintf("rm -rf %s", runtime.ShellQuote(remoteDir))
		if cleanupErr := executor.Execute(context.Background(), cleanupCmd, writer); cleanupErr != nil {
			fmt.Fprintf(writer, "[远程] 清理远端临时目录失败: %v\n", cleanupErr)
		} else {
			fmt.Fprintf(writer, "[远程] 已清理远端临时目录\n")
		}
	}()

	remoteEntry := path.Join(remoteDir, filepath.Base(pkgPath))
	fmt.Fprintf(writer, "[远程] 正在上传产物到 %s ...\n", remoteEntry)
	if err := executor.Upload(ctx, pkgPath, remoteEntry); err != nil {
		return outcome, err
	}

	runCmd, chmodCmd, err := buildRemoteRunCommand(remoteEntry, params)
	if err != nil {
		return outcome, err
	}
	if chmodCmd != "" {
		if err := executor.Execute(ctx, chmodCmd, writer); err != nil {
			return outcome, fmt.Errorf("设置远端产物权限失败: %w", err)
		}
	}

	resultHint, hintErr := resolveRemoteResultHint(params.manifestParams, params.args, remoteDir)
	if hintErr != nil {
		fmt.Fprintf(writer, "[远程] 结果探测规则解析失败: %v\n", hintErr)
		outcome.result = remoteResultProbe{
			Status:  "error",
			Message: fmt.Sprintf("结果探测规则解析失败：%v", hintErr),
		}
	}

	fmt.Fprintf(writer, "[远程] 执行: %s\n", runCmd)
	runErr := executor.Execute(ctx, runCmd, writer)

	if ctx.Err() == nil && resultHint.Path != "" {
		probe, probeErr := probeRemoteResult(ctx, executor, resultHint.Path)
		if probeErr != nil {
			fmt.Fprintf(writer, "[远程] 结果探测失败: %v\n", probeErr)
			outcome.result = remoteResultProbe{
				Status:  "error",
				Path:    resultHint.Path,
				Kind:    resultHint.Kind,
				Message: fmt.Sprintf("结果探测失败：%v", probeErr),
			}
		} else {
			outcome.result = probe
			switch probe.Status {
			case "available":
				fmt.Fprintf(writer, "[远程] 已探测到可下载结果: %s\n", probe.Path)
				if pathWithinRemoteBase(remoteDir, probe.Path) {
					keepRemoteDir = true
					outcome.keepWorkDir = remoteDir
				}
			case "missing":
				fmt.Fprintf(writer, "[远程] 未发现结果路径: %s\n", probe.Path)
			}
		}
	}

	return outcome, runErr
}

func builderKind(kind string) builder.ToolKind {
	if kind == "python" {
		return builder.KindPython
	}
	if kind == "rust" {
		return builder.KindRust
	}
	return builder.KindGo
}

func locateRepoRoot() (string, bool) {
	return runtimeenv.FindRepoRoot()
}

func buildRemoteRunCommand(remoteEntry string, params remoteExecParams) (string, string, error) {
	parsedArgs, err := framework.ParseArgs(params.args)
	if err != nil {
		return "", "", err
	}
	if params.kind == "rust" {
		parsedArgs = normalizeRustCLIArgs(parsedArgs)
	}
	quotedArgs := joinRemoteShellArgs(parsedArgs)
	if params.kind == "python" {
		env := strings.TrimSpace(params.pythonEnv)
		if env == "" {
			env = "python3"
		}
		cmd := fmt.Sprintf("cd %s && %s %s", runtime.ShellQuote(path.Dir(remoteEntry)), runtime.ShellQuote(env), runtime.ShellQuote("./"+path.Base(remoteEntry)))
		if quotedArgs != "" {
			cmd += " " + quotedArgs
		}
		return cmd, "", nil
	}

	cmd := fmt.Sprintf("cd %s && %s", runtime.ShellQuote(path.Dir(remoteEntry)), runtime.ShellQuote("./"+path.Base(remoteEntry)))
	if quotedArgs != "" {
		cmd += " " + quotedArgs
	}
	return cmd, fmt.Sprintf("chmod +x %s", runtime.ShellQuote(remoteEntry)), nil
}

func joinRemoteShellArgs(args []string) string {
	if len(args) == 0 {
		return ""
	}
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		parts = append(parts, runtime.ShellQuote(arg))
	}
	return strings.Join(parts, " ")
}

func buildRemoteWorkDir(taskID string) string {
	safeTaskID := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, strings.TrimSpace(taskID))
	if safeTaskID == "" {
		safeTaskID = fmt.Sprintf("task-%d", time.Now().UnixNano())
	}
	return path.Join("/tmp", "fire-salamander-"+safeTaskID)
}
