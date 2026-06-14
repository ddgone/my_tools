package main

import (
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

func (a *App) OpenFileDialog(req FileDialogRequest) (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("应用尚未初始化")
	}

	if req.Directory {
		dir, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
			Title: req.Title,
		})
		if err != nil {
			return "", err
		}
		return dir, nil
	}

	file, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
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

func (a *App) OpenSaveFileDialog(req FileDialogRequest) (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("应用尚未初始化")
	}

	file, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
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

func (a *App) SaveTextFile(path string, content string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("保存路径不能为空")
	}

	return os.WriteFile(path, []byte(content), 0644)
}
