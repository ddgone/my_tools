package execution

import (
	"context"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"
	"time"

	"fire-salamander-desktop/internal/builder"
	"fire-salamander-desktop/internal/runtime"
	"fire-salamander-desktop/internal/runtimeenv"
	"fire-salamander-desktop/internal/ssh"
	"my_tools/libs/core/procutil"
	"my_tools/libs/core/toolspec"
)

type RemoteExecRequest struct {
	ToolID     string `json:"toolId"`
	ConnID     string `json:"connId"`
	InstanceID string `json:"instanceId"`
	Args       string `json:"args"`
	PythonEnv  string `json:"pythonEnv"`
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

func ExecuteRemotely(ctx context.Context, writer io.Writer, params remoteExecParams) (remoteExecutionOutcome, error) {
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

	repoRoot, ok := LocateRepoRoot()
	if !ok {
		return outcome, fmt.Errorf("当前运行环境缺少源码工作区，暂时无法构建单工具远程产物")
	}

	fmt.Fprintf(writer, "[远程] 正在为目标 %s 准备产物...\n", params.toolName)
	buildResult, err := builder.BuildPackage(builder.BuildRequest{
		ToolID:           params.toolID,
		ToolName:         params.toolName,
		Kind:             BuilderKind(params.kind),
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
	fmt.Fprintf(writer, "\n━━━ 远端执行开始 ━━━\n")
	remoteStart := time.Now()
	runErr := executor.Execute(ctx, runCmd, writer)
	remoteElapsed := time.Since(remoteStart).Round(time.Millisecond)
	fmt.Fprintf(writer, "\n━━━ 远端执行结束 ━━━\n")
	fmt.Fprintf(writer, "耗时: %s\n", remoteElapsed)

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

// BuilderKind converts a string kind to builder.ToolKind.
func BuilderKind(kind string) builder.ToolKind {
	if kind == "python" {
		return builder.KindPython
	}
	if kind == "rust" {
		return builder.KindRust
	}
	return builder.KindGo
}

// LocateRepoRoot returns the source workspace root.
func LocateRepoRoot() (string, bool) {
	return runtimeenv.FindRepoRoot()
}

func buildRemoteRunCommand(remoteEntry string, params remoteExecParams) (string, string, error) {
	parsedArgs, err := procutil.ParseArgs(params.args)
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

// --- remote result probing helpers (moved from task_result_probe.go) ---

type remoteResultHint struct {
	Path string
	Kind string
}

func resolveRemoteResultHint(params []toolspec.ParameterSpec, rawArgs string, remoteWorkDir string) (remoteResultHint, error) {
	outputParam, ok := findLikelyOutputParam(params)
	if !ok {
		return remoteResultHint{}, nil
	}

	parsedArgs, err := procutil.ParseArgs(rawArgs)
	if err != nil {
		return remoteResultHint{}, err
	}
	value, ok := extractParamValue(parsedArgs, outputParam)
	if !ok || strings.TrimSpace(value) == "" {
		value, ok = inferDefaultOutputValue(params, parsedArgs, outputParam, remoteWorkDir)
		if !ok || strings.TrimSpace(value) == "" {
			return remoteResultHint{}, nil
		}
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

func inferDefaultOutputValue(params []toolspec.ParameterSpec, parsedArgs []string, outputParam toolspec.ParameterSpec, remoteWorkDir string) (string, bool) {
	if !supportsDefaultOutputInference(outputParam) {
		return "", false
	}

	inputValue, inputParam, ok := findLikelyInputParamValue(params, parsedArgs, outputParam)
	if !ok {
		return "", false
	}
	resolvedInputPath := resolveRemotePath(inputValue, remoteWorkDir)
	if strings.TrimSpace(resolvedInputPath) == "" {
		return "", false
	}

	switch strings.TrimSpace(inputParam.PathMode) {
	case "file":
		return path.Join(path.Dir(resolvedInputPath), "output"), true
	case "directory":
		return path.Join(resolvedInputPath, "output"), true
	default:
		if looksLikeFilePath(resolvedInputPath) {
			return path.Join(path.Dir(resolvedInputPath), "output"), true
		}
		return path.Join(resolvedInputPath, "output"), true
	}
}

func supportsDefaultOutputInference(outputParam toolspec.ParameterSpec) bool {
	if outputParam.Type != toolspec.FieldTypePath {
		return false
	}
	if strings.TrimSpace(outputParam.PathMode) == "file" {
		return false
	}
	if !isLikelyOutputParam(outputParam) {
		return false
	}

	text := strings.ToLower(strings.Join([]string{
		strings.TrimSpace(outputParam.Placeholder),
		strings.TrimSpace(outputParam.Help),
	}, " "))
	return (strings.Contains(text, "默认") || strings.Contains(text, "留空")) && strings.Contains(text, "output")
}

func findLikelyInputParamValue(params []toolspec.ParameterSpec, parsedArgs []string, outputParam toolspec.ParameterSpec) (string, toolspec.ParameterSpec, bool) {
	for _, param := range params {
		if param.Type != toolspec.FieldTypePath {
			continue
		}
		if isSameParam(param, outputParam) || isLikelyOutputParam(param) {
			continue
		}
		value, ok := extractParamValue(parsedArgs, param)
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}
		return value, param, true
	}
	return "", toolspec.ParameterSpec{}, false
}

func findLikelyOutputParam(params []toolspec.ParameterSpec) (toolspec.ParameterSpec, bool) {
	for _, param := range params {
		if isLikelyOutputParam(param) {
			return param, true
		}
	}
	return toolspec.ParameterSpec{}, false
}

func isLikelyOutputParam(param toolspec.ParameterSpec) bool {
	if param.Type != toolspec.FieldTypePath {
		return false
	}
	argKey := strings.TrimSpace(param.ArgKey)
	key := strings.TrimSpace(param.Key)
	switch {
	case argKey == "output":
		return true
	case key == "output", key == "outputDir":
		return true
	default:
		return false
	}
}

func isSameParam(left toolspec.ParameterSpec, right toolspec.ParameterSpec) bool {
	return strings.TrimSpace(left.Key) == strings.TrimSpace(right.Key) &&
		strings.TrimSpace(left.ArgKey) == strings.TrimSpace(right.ArgKey)
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

func looksLikeFilePath(value string) bool {
	base := path.Base(strings.TrimSpace(value))
	ext := path.Ext(base)
	return base != "" && ext != "" && ext != "."
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
