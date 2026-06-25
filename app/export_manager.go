package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"fire-salamander-desktop/internal/runtimeenv"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type ExportManager struct {
	state  *SharedState
	dialog *DialogManager
}

func NewExportManager(state *SharedState, dialog *DialogManager) *ExportManager {
	return &ExportManager{state: state, dialog: dialog}
}

type exportProgressWriter struct {
	toolID string
	mgr    *ExportManager
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (a *App) ExportTool(req ExportToolRequest) (*ExportToolResult, error) {
	return a.export.ExportTool(req)
}

func (a *App) OpenPath(path string) error {
	return a.export.OpenPath(path)
}

func (m *ExportManager) ExportTool(req ExportToolRequest) (*ExportToolResult, error) {
	if err := ensureTooling(m.state); err != nil {
		return nil, err
	}
	progress := &exportProgressWriter{toolID: req.ToolID, mgr: m}
	defer progress.Flush()
	m.emitExportProgress(req.ToolID, "准备导出")

	m.state.Mu.RLock()
	manifest, ok := m.state.Manifests[req.ToolID]
	m.state.Mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("未找到工具: %s", req.ToolID)
	}

	layout, err := runtimeenv.ResolveLayout()
	if err != nil {
		return nil, fmt.Errorf("初始化运行时目录失败: %w", err)
	}
	m.emitExportProgress(req.ToolID, "准备运行时目录")
	if err := layout.Ensure(); err != nil {
		return nil, fmt.Errorf("准备运行时目录失败: %w", err)
	}

	repoRoot, ok := locateRepoRoot()
	if !ok {
		return nil, fmt.Errorf("当前运行环境缺少源码工作区，暂时无法构建单工具导出产物")
	}

	lastDir, err := m.loadLastExportDirectory()
	if err != nil {
		return nil, err
	}
	defaultDir := sanitizeDialogDefaultDirectory(lastDir)
	if defaultDir == "" {
		defaultDir = layout.ExportsDir()
	}

	exportMode := normalizeExportMode(string(manifest.Kind), req.Mode)
	targetOS, targetArch := normalizeExportTarget(string(manifest.Kind), exportMode, req.TargetOS, req.TargetArch)
	filePath, err := m.dialog.OpenSaveFileDialog(FileDialogRequest{
		Title:            "导出工具",
		FilterName:       exportFilterName(string(manifest.Kind), exportMode),
		FilterGlob:       exportFilterGlob(string(manifest.Kind), exportMode, targetOS),
		Directory:        false,
		DefaultDirectory: defaultDir,
		DefaultFilename:  exportDefaultFileName(manifest.Name, req.ToolID, string(manifest.Kind), exportMode, targetOS, targetArch),
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
	if err := m.saveLastExportDirectory(filepath.Dir(filePath)); err != nil {
		return nil, err
	}

	m.emitExportProgress(req.ToolID, "准备工具产物")
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
		cacheDir:    layout.BuildCacheDir(),
		progress:    progress,
	})
	if err != nil {
		return nil, err
	}
	m.emitExportProgress(req.ToolID, "导出完成")

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

func (w *exportProgressWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buffer.Write(p)
	for {
		line, err := w.buffer.ReadString('\n')
		if err != nil {
			w.buffer.WriteString(line)
			break
		}
		w.mgr.emitExportProgress(w.toolID, strings.TrimRight(line, "\r\n"))
	}

	return len(p), nil
}

func (w *exportProgressWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.buffer.Len() == 0 {
		return
	}
	w.mgr.emitExportProgress(w.toolID, strings.TrimRight(w.buffer.String(), "\r\n"))
	w.buffer.Reset()
}

func (m *ExportManager) emitExportProgress(toolID string, message string) {
	if strings.TrimSpace(message) == "" || m.state.Ctx == nil {
		return
	}
	wailsruntime.EventsEmit(m.state.Ctx, "export:progress", ExportProgressEvent{
		ToolID:   toolID,
		Message:  message,
		Recorded: time.Now().UnixMilli(),
	})
}

func (m *ExportManager) OpenPath(path string) error {
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

func (m *ExportManager) loadLastExportDirectory() (string, error) {
	cfg, err := m.loadExportConfig()
	if err != nil {
		return "", err
	}
	return sanitizeDialogDefaultDirectory(cfg.LastDirectory), nil
}

func (m *ExportManager) saveLastExportDirectory(dir string) error {
	cfg, err := m.loadExportConfig()
	if err != nil {
		return err
	}
	cfg.LastDirectory = strings.TrimSpace(dir)
	return m.writeExportConfig(cfg)
}

func (m *ExportManager) loadExportConfig() (exportConfig, error) {
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

func (m *ExportManager) writeExportConfig(cfg exportConfig) error {
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
