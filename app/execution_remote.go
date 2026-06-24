package main

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
	ToolID    string `json:"toolId"`
	ConnID    string `json:"connId"`
	Args      string `json:"args"`
	PythonEnv string `json:"pythonEnv"`
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
