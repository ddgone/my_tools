package exportpkg

import "testing"

func TestExportDefaultFileNameUsesStableEnglishBinaryName(t *testing.T) {
	got := ExportDefaultFileName("递归目录内容比对", "recursive_content_dir_diff", "go", exportModeBinary, "linux", "amd64")
	if got != "recursive_content_dir_diff_linux_amd64" {
		t.Fatalf("unexpected binary export name: %s", got)
	}
}

func TestExportDefaultFileNameUsesExeSuffixForWindows(t *testing.T) {
	got := ExportDefaultFileName("递归目录内容比对", "recursive_content_dir_diff", "go", exportModeBinary, "windows", "amd64")
	if got != "recursive_content_dir_diff_windows_amd64.exe" {
		t.Fatalf("unexpected windows export name: %s", got)
	}
}

func TestExportDefaultFileNameKeepsPythonStable(t *testing.T) {
	got := ExportDefaultFileName("目录打包", "dir_pack", "python", exportModeBinary, "", "")
	if got != "dir_pack.py" {
		t.Fatalf("unexpected python export name: %s", got)
	}
}

func TestExportDefaultFileNameUsesRustBinaryConvention(t *testing.T) {
	got := ExportDefaultFileName("白犀牛交付点云质检预处理", "bxn_delivery_point_cloud_qc", "rust", exportModeBinary, "linux", "arm64")
	if got != "bxn_delivery_point_cloud_qc_linux_arm64" {
		t.Fatalf("unexpected rust export name: %s", got)
	}
}

func TestNormalizeExportModeForRustAlwaysUsesBinary(t *testing.T) {
	if got := NormalizeExportMode("rust", exportModeSource); got != exportModeBinary {
		t.Fatalf("unexpected rust export mode: %s", got)
	}
}
