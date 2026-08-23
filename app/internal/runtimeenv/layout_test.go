package runtimeenv

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindRepoRootFromPathFindsRepoRoot(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	mustWriteFile(t, filepath.Join(repoRoot, "go.work"))
	mustWriteFile(t, filepath.Join(repoRoot, "app", "wails.json"))

	start := filepath.Join(repoRoot, "app", "frontend", "src")
	if err := os.MkdirAll(start, 0755); err != nil {
		t.Fatalf("创建测试目录失败: %v", err)
	}

	got, ok := findRepoRootFromPath(start)
	if !ok {
		t.Fatalf("期望找到仓库根目录")
	}
	if got != repoRoot {
		t.Fatalf("仓库根目录不正确: got=%s want=%s", got, repoRoot)
	}
}

func TestFindRepoRootFromPathFindsRepoRootFromAppBundle(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	mustWriteFile(t, filepath.Join(repoRoot, "go.work"))
	mustWriteFile(t, filepath.Join(repoRoot, "app", "wails.json"))

	start := filepath.Join(repoRoot, "build", "image", "host", "fire-salamander-desktop.app", "Contents", "MacOS")
	if err := os.MkdirAll(start, 0755); err != nil {
		t.Fatalf("创建测试目录失败: %v", err)
	}

	got, ok := findRepoRootFromPath(start)
	if !ok {
		t.Fatalf("期望从 .app 路径反推出仓库根目录")
	}
	if got != repoRoot {
		t.Fatalf("仓库根目录不正确: got=%s want=%s", got, repoRoot)
	}
}

func TestFindRepoRootFromPathReturnsFalseWhenMarkersMissing(t *testing.T) {
	t.Parallel()

	start := filepath.Join(t.TempDir(), "build", "image", "host")
	if err := os.MkdirAll(start, 0755); err != nil {
		t.Fatalf("创建测试目录失败: %v", err)
	}

	if got, ok := findRepoRootFromPath(start); ok {
		t.Fatalf("不应找到仓库根目录: %s", got)
	}
}

func mustWriteFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("创建父目录失败: %v", err)
	}
	if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
}

func TestProgramToolsDirEmptyWhenNonPortable(t *testing.T) {
	layout := Layout{Root: t.TempDir()}
	if got := layout.ProgramToolsDir(); got != "" {
		t.Fatalf("非便携态 ProgramToolsDir 应为空: got=%s", got)
	}
}

func TestProgramToolsDirUnderPortableProgramRoot(t *testing.T) {
	programRoot := filepath.Join(t.TempDir(), "program")
	layout := Layout{Root: filepath.Join(t.TempDir(), "data"), ProgramRoot: programRoot}
	want := filepath.Join(programRoot, "tools")
	if got := layout.ProgramToolsDir(); got != want {
		t.Fatalf("便携态 ProgramToolsDir 不正确: got=%s want=%s", got, want)
	}
}

func TestDataRootFromPortable(t *testing.T) {
	root := filepath.Join(t.TempDir(), "FireSalamander")
	if got := dataRootFromPortable(root); got != filepath.Join(root, "data") {
		t.Fatalf("便携态数据根不正确: got=%s", got)
	}
}
