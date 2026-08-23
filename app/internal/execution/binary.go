package execution

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"fire-salamander-desktop/internal/builder"
	"fire-salamander-desktop/internal/runtimeenv"
	"my_tools/libs/core/procutil"
	"my_tools/libs/core/toolspec"
)

// ExecuteLocalBinaryTool returns a closure that executes a compiled tool locally.
func ExecuteLocalBinaryTool(ctx context.Context, writer io.Writer, manifest toolspec.ToolManifest) func(args string) error {
	return func(args string) error {
		binaryPath, err := resolveLocalBinary(manifest, writer)
		if err != nil {
			return err
		}
		parsedArgs, err := procutil.ParseArgs(args)
		if err != nil {
			return err
		}
		if manifest.Kind == toolspec.ToolKindRust {
			parsedArgs = normalizeRustCLIArgs(parsedArgs)
		}

		startTime := time.Now()
		fmt.Fprintf(writer, "\n━━━ 本地执行开始 ━━━\n")
		fmt.Fprintf(writer, "工具: %s (%s)\n", manifest.Name, manifest.ID)
		fmt.Fprintf(writer, "产物: %s\n", binaryPath)
		fmt.Fprintf(writer, "命令: %s %s\n", filepath.Base(binaryPath), strings.Join(parsedArgs, " "))
		fmt.Fprintf(writer, "━━━━━━━━━━━━━━━━━━━━\n\n")

		cmd := procutil.CommandContext(ctx, binaryPath, parsedArgs...)
		cmd.Stdout = writer
		cmd.Stderr = writer
		execErr := cmd.Run()

		elapsed := time.Since(startTime).Round(time.Millisecond)
		fmt.Fprintf(writer, "\n━━━ 本地执行结束 ━━━\n")
		fmt.Fprintf(writer, "耗时: %s\n", elapsed)
		if execErr != nil {
			fmt.Fprintf(writer, "状态: 失败\n")
			fmt.Fprintf(writer, "━━━━━━━━━━━━━━━━━━━━\n")
			return fmt.Errorf("执行工具失败: %w", execErr)
		}
		fmt.Fprintf(writer, "状态: 成功\n")
		fmt.Fprintf(writer, "━━━━━━━━━━━━━━━━━━━━\n")
		return nil
	}
}

// NormalizeRustCLIArgs ensures Rust CLIs receive --flag style args.
func NormalizeRustCLIArgs(args []string) []string {
	return normalizeRustCLIArgs(args)
}

func normalizeRustCLIArgs(args []string) []string {
	if len(args) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(args))
	for _, arg := range args {
		if isSingleDashLongFlag(arg) {
			normalized = append(normalized, "-"+arg)
			continue
		}
		normalized = append(normalized, arg)
	}
	return normalized
}

func isSingleDashLongFlag(arg string) bool {
	if len(arg) <= 2 || arg[0] != '-' || arg[1] == '-' {
		return false
	}
	for _, r := range arg[1:] {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-':
		default:
			return false
		}
	}
	return true
}

// resolveLocalBinary 获取工具的本地可执行产物。
//
// 优先使用便携部署态下预置的编译产物（program/tools），命中即直接返回，
// 不触发源码构建、不需要仓库与本机工具链；未命中才回退到 builder.BuildPackage
// 现场构建流程（缓存优先，未命中时在源码工作区下现场编译）。
func resolveLocalBinary(manifest toolspec.ToolManifest, writer io.Writer) (string, error) {
	layout, err := runtimeenv.ResolveLayout()
	if err != nil {
		return "", fmt.Errorf("解析运行时目录失败: %w", err)
	}

	if kind, err := ManifestKindToBuilderKind(manifest.Kind); err == nil {
		if prebuilt, ok := resolveProgramPrebuilt(layout, manifest.ID, kind, runtime.GOOS, runtime.GOARCH); ok {
			return prebuilt, nil
		}
	}

	repoRoot, ok := LocateRepoRoot()
	if !ok {
		return "", fmt.Errorf(
			"预置产物中未找到 %s/%s 平台的 %s 产物，且当前环境无源码工作区无法现场构建；请重新分发包含该平台产物的安装包",
			runtime.GOOS, runtime.GOARCH, manifest.ID)
	}
	if err := layout.Ensure(); err != nil {
		return "", fmt.Errorf("准备运行时目录失败: %w", err)
	}

	kind, err := ManifestKindToBuilderKind(manifest.Kind)
	if err != nil {
		return "", err
	}

	result, err := builder.BuildPackage(builder.BuildRequest{
		ToolID:           manifest.ID,
		ToolName:         manifest.Name,
		Kind:             kind,
		OutputDir:        layout.BuildCacheDir(),
		CacheDir:         layout.BuildCacheDir(),
		RepoRoot:         repoRoot,
		SourceEntry:      manifest.Source.Entry,
		TargetOS:         runtime.GOOS,
		TargetArch:       runtime.GOARCH,
		UseCacheAsOutput: true,
		Progress:         writer,
	})
	if err != nil {
		return "", err
	}
	return result.Path, nil
}

// resolveProgramPrebuilt 在便携部署态下尝试解析指定平台的预置产物；命中返回路径与 true。
func resolveProgramPrebuilt(layout runtimeenv.Layout, toolID string, kind builder.ToolKind, targetOS, targetArch string) (string, bool) {
	programToolsDir := layout.ProgramToolsDir()
	if programToolsDir == "" {
		return "", false
	}
	return builder.ResolveProgramToolPath(programToolsDir, builder.BuildRequest{
		ToolID:     toolID,
		Kind:       kind,
		TargetOS:   targetOS,
		TargetArch: targetArch,
	})
}

// ManifestKindToBuilderKind converts toolspec.ToolKind to builder.ToolKind.
func ManifestKindToBuilderKind(kind toolspec.ToolKind) (builder.ToolKind, error) {
	switch kind {
	case toolspec.ToolKindGo:
		return builder.KindGo, nil
	case toolspec.ToolKindRust:
		return builder.KindRust, nil
	default:
		return "", fmt.Errorf("不支持的编译型工具类型: %s", kind)
	}
}

func rustBinaryName(toolID string) string {
	if runtime.GOOS == "windows" {
		return toolID + ".exe"
	}
	return toolID
}

func localFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
