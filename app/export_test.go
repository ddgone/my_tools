package main

import "testing"

func TestExportDefaultFileNameUsesStableEnglishBinaryName(t *testing.T) {
	got := exportDefaultFileName("递归目录内容比对", "recursive_content_dir_diff", "go", exportModeBinary, "linux", "amd64")
	if got != "recursive_content_dir_diff_linux_amd64" {
		t.Fatalf("unexpected binary export name: %s", got)
	}
}

func TestExportDefaultFileNameUsesExeSuffixForWindows(t *testing.T) {
	got := exportDefaultFileName("递归目录内容比对", "recursive_content_dir_diff", "go", exportModeBinary, "windows", "amd64")
	if got != "recursive_content_dir_diff_windows_amd64.exe" {
		t.Fatalf("unexpected windows export name: %s", got)
	}
}

func TestExportDefaultFileNameKeepsPythonStable(t *testing.T) {
	got := exportDefaultFileName("目录打包", "dir_pack", "python", exportModeBinary, "", "")
	if got != "dir_pack.py" {
		t.Fatalf("unexpected python export name: %s", got)
	}
}
