package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSanitizeDialogDefaultDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "demo.txt")
	if err := os.WriteFile(file, []byte("demo"), 0644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}

	if got := sanitizeDialogDefaultDirectory(dir); got != dir {
		t.Fatalf("期望保留现有目录 %q，得到 %q", dir, got)
	}
	if got := sanitizeDialogDefaultDirectory(file); got != "" {
		t.Fatalf("文件路径不应作为默认目录，得到 %q", got)
	}
	if got := sanitizeDialogDefaultDirectory(filepath.Join(dir, "missing")); got != "" {
		t.Fatalf("不存在的路径不应作为默认目录，得到 %q", got)
	}
	if got := sanitizeDialogDefaultDirectory("   "); got != "" {
		t.Fatalf("空白路径不应作为默认目录，得到 %q", got)
	}
}
