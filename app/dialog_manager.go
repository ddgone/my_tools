package main

import (
	"fire-salamander-desktop/internal/dialog"
)

// DialogManager 是 internal/dialog.Manager 的别名，方便 main 包内使用。
type DialogManager = dialog.Manager

// FileDialogRequest 是 internal/dialog.FileDialogRequest 的别名。
type FileDialogRequest = dialog.FileDialogRequest

func NewDialogManager() *DialogManager {
	return dialog.NewManager()
}

func sanitizeDialogDefaultDirectory(dir string) string {
	return dialog.SanitizeDefaultDirectory(dir)
}

func (a *App) OpenFileDialog(req FileDialogRequest) (string, error) {
	return a.dialog.OpenFileDialog(a.state.Ctx, req)
}

func (a *App) OpenSaveFileDialog(req FileDialogRequest) (string, error) {
	return a.dialog.OpenSaveFileDialog(a.state.Ctx, req)
}

func (a *App) SaveTextFile(path string, content string) error {
	return a.dialog.SaveTextFile(path, content)
}
