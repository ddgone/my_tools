package builder

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildImportPathUsesForwardSlashes(t *testing.T) {
	cases := map[string]string{
		`tools\go_tools\geojson_to_shapefile\tool.go`:   "my_tools/tools/go_tools/geojson_to_shapefile",
		"tools/go_tools/geojson_to_shapefile/tool.go":   "my_tools/tools/go_tools/geojson_to_shapefile",
		"./tools/go_tools/geojson_to_shapefile/tool.go": "my_tools/tools/go_tools/geojson_to_shapefile",
	}

	for input, want := range cases {
		if got := buildImportPath(input); got != want {
			t.Fatalf("buildImportPath(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestResolveGoBinaryUsesExplicitOverride(t *testing.T) {
	t.Setenv(envGoBinary, "")
	fakeDir := t.TempDir()
	fakeGo := filepath.Join(fakeDir, goExecutableName())
	if err := os.WriteFile(fakeGo, []byte("fake"), 0644); err != nil {
		t.Fatalf("write fake go binary: %v", err)
	}

	t.Setenv(envGoBinary, fakeGo)

	got, err := resolveGoBinary()
	if err != nil {
		t.Fatalf("resolveGoBinary returned error: %v", err)
	}
	if got != fakeGo {
		t.Fatalf("expected %s, got %s", fakeGo, got)
	}
}

func TestResolveGoBinaryRejectsMissingOverride(t *testing.T) {
	t.Setenv(envGoBinary, filepath.Join(t.TempDir(), "missing", goExecutableName()))

	_, err := resolveGoBinary()
	if err == nil {
		t.Fatal("expected error for missing go override")
	}
}

func TestDedupeStrings(t *testing.T) {
	got := dedupeStrings([]string{"a", "b", "a", "", "c", "b"})
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("unexpected length: got %d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected value at %d: got %s want %s", i, got[i], want[i])
		}
	}
}

func TestBuildPackageReusesPythonCache(t *testing.T) {
	repoRoot := t.TempDir()
	scriptPath := filepath.Join(repoRoot, "tool.py")
	if err := os.WriteFile(scriptPath, []byte("print('hello')\n"), 0644); err != nil {
		t.Fatalf("write script: %v", err)
	}

	cacheDir := filepath.Join(repoRoot, "cache")
	firstOut := filepath.Join(repoRoot, "out-1")
	secondOut := filepath.Join(repoRoot, "out-2")

	first, err := BuildPackage(BuildRequest{
		ToolID:      "demo_python",
		Kind:        KindPython,
		OutputDir:   firstOut,
		CacheDir:    cacheDir,
		OutputName:  "demo.py",
		RepoRoot:    repoRoot,
		SourceEntry: scriptPath,
	})
	if err != nil {
		t.Fatalf("first build returned error: %v", err)
	}
	if first.CacheHit {
		t.Fatal("first build should not hit cache")
	}
	firstBytes, err := os.ReadFile(first.Path)
	if err != nil {
		t.Fatalf("read first output: %v", err)
	}

	second, err := BuildPackage(BuildRequest{
		ToolID:      "demo_python",
		Kind:        KindPython,
		OutputDir:   secondOut,
		CacheDir:    cacheDir,
		OutputName:  "demo.py",
		RepoRoot:    repoRoot,
		SourceEntry: scriptPath,
	})
	if err != nil {
		t.Fatalf("second build returned error: %v", err)
	}
	if !second.CacheHit {
		t.Fatal("second build should hit cache")
	}
	secondBytes, err := os.ReadFile(second.Path)
	if err != nil {
		t.Fatalf("read second output: %v", err)
	}
	if string(firstBytes) != string(secondBytes) {
		t.Fatalf("cached output mismatch: got %q want %q", string(secondBytes), string(firstBytes))
	}
}

func TestCacheArtifactFileNameUsesStableToolID(t *testing.T) {
	req := BuildRequest{
		ToolID:     "recursive_content_dir_diff",
		ToolName:   "递归目录内容比对",
		Kind:       KindGo,
		TargetOS:   "linux",
		TargetArch: "amd64",
	}

	gotChinese := cacheArtifactFileName(req, "递归目录内容比对")
	gotEnglish := cacheArtifactFileName(req, "recursive_content_dir_diff_linux_amd64")
	if gotChinese != "recursive_content_dir_diff_linux_amd64" {
		t.Fatalf("unexpected cache artifact name for localized output: %s", gotChinese)
	}
	if gotEnglish != gotChinese {
		t.Fatalf("cache artifact name should ignore output display name: %s vs %s", gotEnglish, gotChinese)
	}
}

func TestCacheArtifactFileNameKeepsWindowsExeSuffix(t *testing.T) {
	req := BuildRequest{
		ToolID:     "recursive_content_dir_diff",
		ToolName:   "递归目录内容比对",
		Kind:       KindGo,
		TargetOS:   "windows",
		TargetArch: "amd64",
	}

	got := cacheArtifactFileName(req, "递归目录内容比对.exe")
	if got != "recursive_content_dir_diff_windows_amd64.exe" {
		t.Fatalf("unexpected windows cache artifact name: %s", got)
	}
}
