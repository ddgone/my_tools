package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type DialogManager struct {
	state *SharedState
}

func NewDialogManager(state *SharedState) *DialogManager {
	return &DialogManager{state: state}
}

func (m *DialogManager) OpenFileDialog(req FileDialogRequest) (string, error) {
	if m.state.Ctx == nil {
		return "", fmt.Errorf("应用尚未初始化")
	}

	if req.Directory {
		dir, err := wailsruntime.OpenDirectoryDialog(m.state.Ctx, wailsruntime.OpenDialogOptions{
			Title: req.Title,
		})
		if err != nil {
			return "", err
		}
		return dir, nil
	}

	file, err := wailsruntime.OpenFileDialog(m.state.Ctx, wailsruntime.OpenDialogOptions{
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

func (m *DialogManager) OpenSaveFileDialog(req FileDialogRequest) (string, error) {
	if m.state.Ctx == nil {
		return "", fmt.Errorf("应用尚未初始化")
	}

	file, err := wailsruntime.SaveFileDialog(m.state.Ctx, wailsruntime.SaveDialogOptions{
		Title:            req.Title,
		DefaultDirectory: sanitizeDialogDefaultDirectory(req.DefaultDirectory),
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

func (m *DialogManager) SaveTextFile(path string, content string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("保存路径不能为空")
	}

	return os.WriteFile(path, []byte(content), 0644)
}

func sanitizeDialogDefaultDirectory(dir string) string {
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
