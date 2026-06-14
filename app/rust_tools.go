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
	"my_tools/libs/core/toolspec"
	"my_tools/libs/framework"
)

func executeLocalRustTool(ctx context.Context, writer io.Writer, manifest toolspec.ToolManifest) func(args string) error {
	return func(args string) error {
		binaryPath, err := resolveLocalRustBinary(manifest, writer)
		if err != nil {
			return err
		}
		parsedArgs, err := framework.ParseArgs(args)
		if err != nil {
			return err
		}
		parsedArgs = normalizeRustCLIArgs(parsedArgs)
		cmd := exec.CommandContext(ctx, binaryPath, parsedArgs...)
		cmd.Stdout = writer
		cmd.Stderr = writer
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("执行 Rust 工具失败: %w", err)
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

func resolveLocalRustBinary(manifest toolspec.ToolManifest, writer io.Writer) (string, error) {
	if assetsDir, ok := runtimeenv.FindBundledAssetsDir(); ok {
		bundledPath := filepath.Join(assetsDir, "rust", runtime.GOOS+"_"+runtime.GOARCH, rustBinaryName(manifest.ID))
		if localFileExists(bundledPath) {
			return bundledPath, nil
		}
	}

	repoRoot, ok := locateRepoRoot()
	if !ok {
		return "", fmt.Errorf("未找到已打包的 Rust 本地产物，且当前运行环境缺少源码工作区")
	}
	layout, err := runtimeenv.ResolveLayout()
	if err != nil {
		return "", fmt.Errorf("解析运行时目录失败: %w", err)
	}
	if err := layout.Ensure(); err != nil {
		return "", fmt.Errorf("准备运行时目录失败: %w", err)
	}

	result, err := builder.BuildPackage(builder.BuildRequest{
		ToolID:           manifest.ID,
		ToolName:         manifest.Name,
		Kind:             builder.KindRust,
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
