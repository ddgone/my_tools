package main

import "fire-salamander-desktop/internal/exportpkg"

func NewExportManager(state *SharedState, dialog *DialogManager) *ExportManager {
	return exportpkg.NewManager(state, dialog, ensureTooling)
}

func (a *App) ExportTool(req ExportToolRequest) (*ExportToolResult, error) {
	return a.export.ExportTool(req)
}

func (a *App) OpenPath(path string) error {
	return a.export.OpenPath(path)
}
