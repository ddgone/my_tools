package builder

import (
	"os"
	"path/filepath"
	"strings"
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

func TestResolveZigBinaryUsesExplicitOverride(t *testing.T) {
	fakeDir := t.TempDir()
	fakeZig := filepath.Join(fakeDir, executableName("zig"))
	if err := os.WriteFile(fakeZig, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("write fake zig: %v", err)
	}
	t.Setenv(envZigBinary, fakeZig)

	got, err := resolveZigBinary()
	if err != nil {
		t.Fatalf("resolveZigBinary returned error: %v", err)
	}
	if got != fakeZig {
		t.Fatalf("expected %s, got %s", fakeZig, got)
	}
}

func TestRustCommandEnvPrependsCargoAndZigDirs(t *testing.T) {
	// Use filepath.Join so paths match OS separators (backslash on Windows)
	cargoDir := filepath.Join("/", "tmp", "cargo-home", "bin")
	zigDir := filepath.Join("/", "tmp", "zig-home", "bin")
	targetDir := filepath.Join("/", "tmp", "target-dir")

	env := rustCommandEnv(
		filepath.Join(cargoDir, "cargo"),
		filepath.Join(zigDir, "zig"),
		filepath.Join(cargoDir, "cargo-zigbuild"),
		targetDir,
	)
	var pathValue string
	var targetValue string
	for _, entry := range env {
		switch {
		case strings.HasPrefix(entry, "PATH="):
			pathValue = strings.TrimPrefix(entry, "PATH=")
		case strings.HasPrefix(entry, "CARGO_TARGET_DIR="):
			targetValue = strings.TrimPrefix(entry, "CARGO_TARGET_DIR=")
		}
	}
	if targetValue != targetDir {
		t.Fatalf("unexpected target dir env: %s", targetValue)
	}
	if !strings.HasPrefix(pathValue, zigDir+string(os.PathListSeparator)+cargoDir) &&
		!strings.HasPrefix(pathValue, cargoDir+string(os.PathListSeparator)+zigDir) {
		t.Fatalf("expected PATH to include cargo and zig dirs first, got %s", pathValue)
	}
}

func TestResolveRustBuildTargetSupportsWindowsArm64(t *testing.T) {
	got, native, err := resolveRustBuildTarget("windows", "arm64")
	if err != nil {
		t.Fatalf("resolveRustBuildTarget returned error: %v", err)
	}
	if native {
		t.Fatal("windows/arm64 should be treated as cross build on this host")
	}
	if got != "aarch64-pc-windows-gnullvm" {
		t.Fatalf("unexpected target triple: %s", got)
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

func TestResolveCachePathsUsesStableEnglishToolDirectory(t *testing.T) {
	cacheDir := t.TempDir()
	req := BuildRequest{
		ToolID:     "recursive_content_dir_diff",
		ToolName:   "递归目录内容比对",
		Kind:       KindGo,
		CacheDir:   cacheDir,
		TargetOS:   "linux",
		TargetArch: "amd64",
	}

	artifactPath, sourcePath, cacheKeyPath, err := resolveCachePaths(req, "linux_amd64", "递归目录内容比对", filepath.Join(cacheDir, "tool.go"))
	if err != nil {
		t.Fatalf("resolveCachePaths returned error: %v", err)
	}
	toolDir := filepath.Join(cacheDir, "recursive_content_dir_diff")
	if filepath.Dir(filepath.Dir(filepath.Dir(artifactPath))) != toolDir {
		t.Fatalf("artifact path should use stable toolID directory: %s", artifactPath)
	}
	if filepath.Dir(filepath.Dir(filepath.Dir(sourcePath))) != toolDir {
		t.Fatalf("source path should use stable toolID directory: %s", sourcePath)
	}
	if filepath.Dir(cacheKeyPath) != filepath.Join(toolDir, "linux_amd64") {
		t.Fatalf("cache key path should use stable toolID directory: %s", cacheKeyPath)
	}
}

func TestProbeBuildCacheReportsHitAfterPythonBuild(t *testing.T) {
	repoRoot := t.TempDir()
	scriptPath := filepath.Join(repoRoot, "tool.py")
	if err := os.WriteFile(scriptPath, []byte("print('cache')\n"), 0644); err != nil {
		t.Fatalf("write script: %v", err)
	}
	cacheDir := filepath.Join(repoRoot, "cache")

	_, err := BuildPackage(BuildRequest{
		ToolID:      "probe_python",
		Kind:        KindPython,
		OutputDir:   filepath.Join(repoRoot, "out"),
		CacheDir:    cacheDir,
		OutputName:  "probe.py",
		RepoRoot:    repoRoot,
		SourceEntry: scriptPath,
	})
	if err != nil {
		t.Fatalf("build package: %v", err)
	}

	probe, err := ProbeBuildCache(BuildRequest{
		ToolID:      "probe_python",
		Kind:        KindPython,
		OutputDir:   filepath.Join(repoRoot, "out"),
		CacheDir:    cacheDir,
		OutputName:  "probe.py",
		RepoRoot:    repoRoot,
		SourceEntry: scriptPath,
	})
	if err != nil {
		t.Fatalf("probe cache: %v", err)
	}
	if !probe.CacheHit {
		t.Fatal("expected probe to report cache hit")
	}
}

func TestResolveProgramToolPathHitsPrebuiltArtifactWithoutSource(t *testing.T) {
	programToolsDir := t.TempDir()
	req := BuildRequest{
		ToolID:     "recursive_content_dir_diff",
		Kind:       KindGo,
		TargetOS:   "linux",
		TargetArch: "amd64",
	}

	// 用既有命名规则预先铺一个产物
	artifactPath, _ := ResolveProgramToolPath(programToolsDir, req)
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(artifactPath, []byte("bin"), 0755); err != nil {
		t.Fatalf("write artifact: %v", err)
	}

	got, ok := ResolveProgramToolPath(programToolsDir, req)
	if !ok {
		t.Fatal("expected prebuilt artifact to be found")
	}
	if got != artifactPath {
		t.Fatalf("unexpected path: got=%s want=%s", got, artifactPath)
	}
}

func TestResolveProgramToolPathReturnsFalseWhenMissingOrEmptyDir(t *testing.T) {
	programToolsDir := t.TempDir()
	req := BuildRequest{
		ToolID:     "abc_tool",
		Kind:       KindGo,
		TargetOS:   "linux",
		TargetArch: "arm64",
	}
	if _, ok := ResolveProgramToolPath(programToolsDir, req); ok {
		t.Fatal("missing artifact should not be found")
	}
	if _, ok := ResolveProgramToolPath("", req); ok {
		t.Fatal("empty program tools dir should not be found")
	}
}
