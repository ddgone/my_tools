package exportpkg

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"fire-salamander-desktop/internal/builder"
	"fire-salamander-desktop/internal/execution"
)

type ExportToolRequest struct {
	ToolID     string `json:"toolId"`
	Mode       string `json:"mode,omitempty"`
	TargetOS   string `json:"targetOS,omitempty"`
	TargetArch string `json:"targetArch,omitempty"`
}

type ExportToolResult struct {
	ToolID     string `json:"toolId"`
	ToolName   string `json:"toolName"`
	Strategy   string `json:"strategy"`
	Mode       string `json:"mode"`
	FilePath   string `json:"filePath"`
	Directory  string `json:"directory"`
	TargetOS   string `json:"targetOS,omitempty"`
	TargetArch string `json:"targetArch,omitempty"`
}

type ExportProgressEvent struct {
	ToolID   string `json:"toolId"`
	Message  string `json:"message"`
	Recorded int64  `json:"recorded"`
}

type exportConfig struct {
	LastDirectory string `json:"lastDirectory"`
}

const (
	exportModeBinary = "binary"
	exportModeSource = "source"
)

type exportArtifactRequest struct {
	toolID, toolName string
	kind, mode       string
	outputPath       string
	repoRoot         string
	sourceEntry      string
	targetOS         string
	targetArch       string
	cacheDir         string
	progress         io.Writer
}

func exportArtifact(req exportArtifactRequest) (string, error) {
	if req.mode == exportModeSource {
		return exportSourceArtifact(req)
	}

	buildReq := builder.BuildRequest{
		ToolID:      req.toolID,
		ToolName:    req.toolName,
		Kind:        execution.BuilderKind(req.kind),
		OutputDir:   filepath.Dir(req.outputPath),
		CacheDir:    req.cacheDir,
		OutputName:  filepath.Base(req.outputPath),
		RepoRoot:    req.repoRoot,
		SourceEntry: req.sourceEntry,
		TargetOS:    req.targetOS,
		TargetArch:  req.targetArch,
		Progress:    req.progress,
	}
	result, err := builder.BuildPackage(buildReq)
	if err != nil {
		return "", err
	}
	return result.Path, nil
}

func exportSourceArtifact(req exportArtifactRequest) (string, error) {
	reportExportProgress(req.progress, "复制源码产物")
	sourcePath, err := resolveSourceEntryPath(req.repoRoot, req.sourceEntry)
	if err != nil {
		return "", err
	}
	if err := copyExportFile(sourcePath, req.outputPath, 0644); err != nil {
		return "", fmt.Errorf("导出源码失败: %w", err)
	}
	return req.outputPath, nil
}

func reportExportProgress(writer io.Writer, message string) {
	if writer == nil {
		return
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}
	_, _ = io.WriteString(writer, message+"\n")
}

func resolveSourceEntryPath(repoRoot string, sourceEntry string) (string, error) {
	entry := strings.TrimSpace(sourceEntry)
	if entry == "" {
		return "", fmt.Errorf("缺少源码入口")
	}
	if filepath.IsAbs(entry) {
		return entry, nil
	}
	if strings.TrimSpace(repoRoot) == "" {
		return "", fmt.Errorf("缺少仓库根目录，无法解析源码入口")
	}
	return filepath.Join(repoRoot, filepath.FromSlash(entry)), nil
}

func copyExportFile(srcPath, dstPath string, mode os.FileMode) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	return os.Chmod(dstPath, mode)
}

func NormalizeExportMode(kind string, mode string) string {
	normalized := strings.TrimSpace(mode)
	if kind == "python" {
		return exportModeSource
	}
	if kind == "rust" {
		return exportModeBinary
	}
	if normalized == exportModeSource {
		return exportModeSource
	}
	return exportModeBinary
}

func NormalizeExportTarget(kind string, mode string, targetOS string, targetArch string) (string, string) {
	if (kind != "go" && kind != "rust") || mode != exportModeBinary {
		return "", ""
	}
	normalizedOS := strings.TrimSpace(targetOS)
	if normalizedOS == "" {
		normalizedOS = runtime.GOOS
	}
	normalizedArch := strings.TrimSpace(targetArch)
	if normalizedArch == "" {
		normalizedArch = runtime.GOARCH
	}
	return normalizedOS, normalizedArch
}

func exportFilterName(kind string, mode string) string {
	if kind == "python" || mode == exportModeSource {
		if kind == "python" {
			return "Python 脚本"
		}
		if kind == "rust" {
			return "Rust 源码"
		}
		return "Go 源码"
	}
	if kind == "go" {
		return "可执行文件"
	}
	if kind == "rust" {
		return "Rust 可执行文件"
	}
	if kind == "python" {
		return "Python 脚本"
	}
	return "可执行文件"
}

func exportFilterGlob(kind string, mode string, targetOS string) string {
	if kind == "python" {
		return "*.py"
	}
	if mode == exportModeSource {
		if kind == "rust" {
			return "*.rs"
		}
		return "*.go"
	}
	if targetOS == "windows" {
		return "*.exe"
	}
	return "*"
}

func ExportDefaultFileName(toolName string, toolID string, kind string, mode string, targetOS string, targetArch string) string {
	base := SanitizeExportBaseName(toolID)
	if base == "" {
		base = SanitizeExportBaseName(toolName)
	}
	if base == "" {
		base = "tool"
	}
	if kind == "python" {
		return base + ".py"
	}
	if mode == exportModeSource {
		if kind == "rust" {
			return base + ".rs"
		}
		return base + ".go"
	}
	if targetOS != "" {
		base += "_" + SanitizeExportBaseName(targetOS)
	}
	if (kind == "go" || kind == "rust") && mode == exportModeBinary {
		if targetArch != "" {
			base += "_" + SanitizeExportBaseName(targetArch)
		}
	}
	if targetOS == "windows" {
		return base + ".exe"
	}
	return base
}

func FinalizeExportPath(path string, kind string, mode string, targetOS string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	ext := strings.ToLower(filepath.Ext(trimmed))
	switch {
	case kind == "python" && ext != ".py":
		return trimmed + ".py"
	case kind == "rust" && mode == exportModeSource && ext != ".rs":
		return trimmed + ".rs"
	case mode == exportModeSource && ext != ".go":
		return trimmed + ".go"
	case (kind == "go" || kind == "rust") && targetOS == "windows" && ext != ".exe":
		return trimmed + ".exe"
	default:
		return trimmed
	}
}

func SanitizeExportBaseName(name string) string {
	trimmed := strings.TrimSpace(name)
	trimmed = strings.Trim(trimmed, ". ")
	if trimmed == "" {
		return ""
	}

	replaced := strings.Map(func(r rune) rune {
		switch r {
		case '<', '>', ':', '"', '/', '\\', '|', '?', '*':
			return '_'
		default:
			return r
		}
	}, trimmed)
	replaced = strings.Trim(replaced, ". ")
	return replaced
}
