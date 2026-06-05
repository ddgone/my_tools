package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"fire-salamander-desktop/internal/builder"
	"fire-salamander-desktop/internal/runtimeenv"
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

type exportConfig struct {
	LastDirectory string `json:"lastDirectory"`
}

const (
	exportModeBinary = "binary"
	exportModeSource = "source"
)

func (a *App) ExportTool(req ExportToolRequest) (*ExportToolResult, error) {
	if err := a.ensureTooling(); err != nil {
		return nil, err
	}

	a.mu.RLock()
	manifest, ok := a.manifests[req.ToolID]
	a.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("未找到工具: %s", req.ToolID)
	}

	layout, err := runtimeenv.ResolveLayout()
	if err != nil {
		return nil, fmt.Errorf("初始化运行时目录失败: %w", err)
	}
	if err := layout.Ensure(); err != nil {
		return nil, fmt.Errorf("准备运行时目录失败: %w", err)
	}

	repoRoot, ok := locateRepoRoot()
	if !ok {
		return nil, fmt.Errorf("当前运行环境缺少源码工作区，暂时无法构建单工具导出产物")
	}

	lastDir, err := a.loadLastExportDirectory()
	if err != nil {
		return nil, err
	}
	defaultDir := lastDir
	if defaultDir == "" {
		defaultDir = layout.ExportsDir()
	}

	exportMode := normalizeExportMode(string(manifest.Kind), req.Mode)
	targetOS, targetArch := normalizeExportTarget(string(manifest.Kind), exportMode, req.TargetOS, req.TargetArch)
	filePath, err := a.OpenSaveFileDialog(FileDialogRequest{
		Title:            "导出工具",
		FilterName:       exportFilterName(string(manifest.Kind), exportMode),
		FilterGlob:       exportFilterGlob(string(manifest.Kind), exportMode, targetOS),
		Directory:        false,
		DefaultDirectory: defaultDir,
		DefaultFilename:  exportDefaultFileName(manifest.Name, req.ToolID, string(manifest.Kind), exportMode, targetOS),
	})
	if err != nil {
		return nil, err
	}
	filePath = finalizeExportPath(filePath, string(manifest.Kind), exportMode, targetOS)
	if strings.TrimSpace(filePath) == "" {
		return nil, nil
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return nil, fmt.Errorf("创建导出目录失败: %w", err)
	}
	if err := a.saveLastExportDirectory(filepath.Dir(filePath)); err != nil {
		return nil, err
	}

	exportedPath, err := exportArtifact(exportArtifactRequest{
		toolID:      manifest.ID,
		toolName:    manifest.Name,
		kind:        string(manifest.Kind),
		mode:        exportMode,
		outputPath:  filePath,
		repoRoot:    repoRoot,
		sourceEntry: manifest.Source.Entry,
		targetOS:    targetOS,
		targetArch:  targetArch,
	})
	if err != nil {
		return nil, err
	}

	return &ExportToolResult{
		ToolID:     manifest.ID,
		ToolName:   manifest.Name,
		Strategy:   string(manifest.Export.Strategy),
		Mode:       exportMode,
		FilePath:   exportedPath,
		Directory:  filepath.Dir(exportedPath),
		TargetOS:   targetOS,
		TargetArch: targetArch,
	}, nil
}

func (a *App) OpenPath(path string) error {
	target := strings.TrimSpace(path)
	if target == "" {
		return fmt.Errorf("路径不能为空")
	}
	if _, err := os.Stat(target); err != nil {
		return fmt.Errorf("路径不存在: %w", err)
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", target)
	case "darwin":
		cmd = exec.Command("open", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("打开路径失败: %w", err)
	}
	return nil
}

func (a *App) loadLastExportDirectory() (string, error) {
	cfg, err := a.loadExportConfig()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(cfg.LastDirectory), nil
}

func (a *App) saveLastExportDirectory(dir string) error {
	cfg, err := a.loadExportConfig()
	if err != nil {
		return err
	}
	cfg.LastDirectory = strings.TrimSpace(dir)
	return a.writeExportConfig(cfg)
}

func (a *App) loadExportConfig() (exportConfig, error) {
	layout, err := runtimeenv.ResolveLayout()
	if err != nil {
		return exportConfig{}, fmt.Errorf("解析运行时目录失败: %w", err)
	}
	configPath := filepath.Join(layout.ConfigDir(), "app.json")
	doc, err := loadConfigDocument(configPath)
	if err != nil {
		return exportConfig{}, err
	}

	var cfg exportConfig
	if raw, ok := doc["export"]; ok && len(raw) > 0 {
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return exportConfig{}, fmt.Errorf("解析导出配置失败: %w", err)
		}
	}
	return cfg, nil
}

func (a *App) writeExportConfig(cfg exportConfig) error {
	layout, err := runtimeenv.ResolveLayout()
	if err != nil {
		return fmt.Errorf("解析运行时目录失败: %w", err)
	}
	if err := os.MkdirAll(layout.ConfigDir(), 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}
	configPath := filepath.Join(layout.ConfigDir(), "app.json")
	doc, err := loadConfigDocument(configPath)
	if err != nil {
		return err
	}

	exportData, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("序列化导出配置失败: %w", err)
	}
	doc["export"] = exportData

	output, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("格式化配置文件失败: %w", err)
	}
	if err := os.WriteFile(configPath, output, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}
	return nil
}

type exportArtifactRequest struct {
	toolID, toolName string
	kind, mode       string
	outputPath       string
	repoRoot         string
	sourceEntry      string
	targetOS         string
	targetArch       string
}

func exportArtifact(req exportArtifactRequest) (string, error) {
	if req.mode == exportModeSource {
		return exportSourceArtifact(req)
	}

	buildReq := builder.BuildRequest{
		ToolID:      req.toolID,
		ToolName:    req.toolName,
		Kind:        builderKind(req.kind),
		OutputDir:   filepath.Dir(req.outputPath),
		OutputName:  filepath.Base(req.outputPath),
		RepoRoot:    req.repoRoot,
		SourceEntry: req.sourceEntry,
		TargetOS:    req.targetOS,
		TargetArch:  req.targetArch,
	}
	return builder.BuildPackage(buildReq)
}

func exportSourceArtifact(req exportArtifactRequest) (string, error) {
	sourcePath, err := resolveSourceEntryPath(req.repoRoot, req.sourceEntry)
	if err != nil {
		return "", err
	}
	if err := copyExportFile(sourcePath, req.outputPath, 0644); err != nil {
		return "", fmt.Errorf("导出源码失败: %w", err)
	}
	return req.outputPath, nil
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

func normalizeExportMode(kind string, mode string) string {
	normalized := strings.TrimSpace(mode)
	if kind == "python" {
		return exportModeSource
	}
	if normalized == exportModeSource {
		return exportModeSource
	}
	return exportModeBinary
}

func normalizeExportTarget(kind string, mode string, targetOS string, targetArch string) (string, string) {
	if kind != "go" || mode != exportModeBinary {
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
		return "Go 源码"
	}
	if kind == "go" {
		return "可执行文件"
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
		return "*.go"
	}
	if targetOS == "windows" {
		return "*.exe"
	}
	return "*"
}

func exportDefaultFileName(toolName string, toolID string, kind string, mode string, targetOS string) string {
	base := sanitizeExportBaseName(toolName)
	if base == "" {
		base = sanitizeExportBaseName(toolID)
	}
	if base == "" {
		base = "tool"
	}
	if kind == "python" {
		return base + ".py"
	}
	if mode == exportModeSource {
		return base + ".go"
	}
	if targetOS == "windows" {
		return base + ".exe"
	}
	return base
}

func finalizeExportPath(path string, kind string, mode string, targetOS string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	ext := strings.ToLower(filepath.Ext(trimmed))
	switch {
	case kind == "python" && ext != ".py":
		return trimmed + ".py"
	case mode == exportModeSource && ext != ".go":
		return trimmed + ".go"
	case kind == "go" && targetOS == "windows" && ext != ".exe":
		return trimmed + ".exe"
	default:
		return trimmed
	}
}

func sanitizeExportBaseName(name string) string {
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
