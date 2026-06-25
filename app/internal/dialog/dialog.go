package dialog

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type FileDialogRequest struct {
	Title            string `json:"title"`
	FilterName       string `json:"filterName"`
	FilterGlob       string `json:"filterGlob"`
	Directory        bool   `json:"directory"`
	DefaultDirectory string `json:"defaultDirectory"`
	DefaultFilename  string `json:"defaultFilename"`
}

type Manager struct{}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) OpenFileDialog(ctx context.Context, req FileDialogRequest) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("应用尚未初始化")
	}

	if req.Directory {
		dir, err := wailsruntime.OpenDirectoryDialog(ctx, wailsruntime.OpenDialogOptions{
			Title: req.Title,
		})
		if err != nil {
			return "", err
		}
		return dir, nil
	}

	file, err := wailsruntime.OpenFileDialog(ctx, wailsruntime.OpenDialogOptions{
		Title: req.Title,
		Filters: []wailsruntime.FileFilter{
			{
				DisplayName: req.FilterName,
				Pattern:     req.FilterGlob,
			},
		},
	})
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(file), nil
}

func (m *Manager) OpenSaveFileDialog(ctx context.Context, req FileDialogRequest) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("应用尚未初始化")
	}

	file, err := wailsruntime.SaveFileDialog(ctx, wailsruntime.SaveDialogOptions{
		Title:            req.Title,
		DefaultDirectory: SanitizeDefaultDirectory(req.DefaultDirectory),
		DefaultFilename:  strings.TrimSpace(req.DefaultFilename),
		Filters: []wailsruntime.FileFilter{
			{
				DisplayName: req.FilterName,
				Pattern:     req.FilterGlob,
			},
		},
	})
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(file), nil
}

func (m *Manager) SaveTextFile(path string, content string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("保存路径不能为空")
	}

	return os.WriteFile(path, []byte(content), 0644)
}

// SanitizeDefaultDirectory 验证并清理目录路径，无效时返回空字符串。
func SanitizeDefaultDirectory(dir string) string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return ""
	}
	dir = filepath.Clean(dir)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return ""
	}
	return dir
}
