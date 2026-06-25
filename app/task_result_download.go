package main

import (
	"fmt"
	"path"
	"strings"
)

func calculateDownloadProgress(downloaded int64, total int64) float64 {
	if total <= 0 {
		if downloaded > 0 {
			return 8
		}
		return 0
	}
	percent := (float64(downloaded) / float64(total)) * 100
	if percent < 0 {
		return 0
	}
	if percent > 100 {
		return 100
	}
	return percent
}

func buildDownloadTaskMessage(downloaded int64, total int64) string {
	if total <= 0 {
		return fmt.Sprintf("%s / 正在估算大小", formatByteCount(downloaded))
	}
	return fmt.Sprintf("%s / %s", formatByteCount(downloaded), formatByteCount(total))
}

func initialDownloadTaskMessage(remoteResultKind string) string {
	if remoteResultKind == "directory" {
		return "正在准备下载归档"
	}
	return "正在下载输出结果"
}

func formatByteCount(value int64) string {
	if value < 1024 {
		return fmt.Sprintf("%d B", value)
	}
	const unit = 1024
	div, exp := int64(unit), 0
	for n := value / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(value)/float64(div), "KMGTPE"[exp])
}

func buildResultDownloadFileName(toolName string, remoteResultPath string, remoteResultKind string) string {
	base := path.Base(strings.TrimSpace(remoteResultPath))
	base = strings.TrimSpace(base)
	if base == "" || base == "." || base == "/" {
		base = sanitizeExportBaseName(toolName)
	}
	if base == "" {
		base = "result"
	}
	if remoteResultKind == "directory" && !strings.HasSuffix(strings.ToLower(base), ".tar.gz") {
		base += ".tar.gz"
	}
	return base
}

func finalizeResultDownloadPath(selectedPath string, remoteResultKind string) string {
	trimmed := strings.TrimSpace(selectedPath)
	if trimmed == "" {
		return ""
	}
	if remoteResultKind == "directory" && !strings.HasSuffix(strings.ToLower(trimmed), ".tar.gz") {
		return trimmed + ".tar.gz"
	}
	return trimmed
}

func resultDownloadFilterName(remoteResultKind string) string {
	if remoteResultKind == "directory" {
		return "Tar Gzip 归档"
	}
	return "所有文件"
}

func resultDownloadFilterGlob(remoteResultKind string) string {
	if remoteResultKind == "directory" {
		return "*.tar.gz"
	}
	return "*.*"
}
