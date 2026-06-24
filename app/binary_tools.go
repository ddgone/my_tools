package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"fire-salamander-desktop/internal/builder"
	"fire-salamander-desktop/internal/runtimeenv"
	"my_tools/libs/core/procutil"
	"my_tools/libs/core/toolspec"
)

func executeLocalBinaryTool(ctx context.Context, writer io.Writer, manifest toolspec.ToolManifest) func(args string) error {
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
		cmd := exec.CommandContext(ctx, binaryPath, parsedArgs...)
		cmd.Stdout = writer
		cmd.Stderr = writer
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("执行工具失败: %w", err)
		}
		return nil
	}
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

func resolveLocalBinary(manifest toolspec.ToolManifest, writer io.Writer) (string, error) {
	if bundledPath, ok := resolveBundledBinary(manifest); ok {
		return bundledPath, nil
	}

	repoRoot, ok := locateRepoRoot()
	if !ok {
		return "", fmt.Errorf("未找到已打包的 %s 本地产物，且当前运行环境缺少源码工作区", manifest.Kind)
	}
	layout, err := runtimeenv.ResolveLayout()
	if err != nil {
		return "", fmt.Errorf("解析运行时目录失败: %w", err)
	}
	if err := layout.Ensure(); err != nil {
		return "", fmt.Errorf("准备运行时目录失败: %w", err)
	}

	kind, err := manifestKindToBuilderKind(manifest.Kind)
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

func resolveBundledBinary(manifest toolspec.ToolManifest) (string, bool) {
	assetsDir, ok := runtimeenv.FindBundledAssetsDir()
	if !ok {
		return "", false
	}
	kindDir := string(manifest.Kind)
	binaryName := manifest.ID
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	bundledPath := filepath.Join(assetsDir, kindDir, runtime.GOOS+"_"+runtime.GOARCH, binaryName)
	if localFileExists(bundledPath) {
		return bundledPath, true
	}
	return "", false
}

func manifestKindToBuilderKind(kind toolspec.ToolKind) (builder.ToolKind, error) {
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
