package execution

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"fire-salamander-desktop/internal/builder"
	"fire-salamander-desktop/internal/runtimeenv"
)

func TestNormalizeRustCLIArgs(t *testing.T) {
	input := []string{"-input", "/tmp/source.laz", "-output", "/tmp/out.laz", "-raster-only", "-h", "--threads", "4"}
	got := normalizeRustCLIArgs(input)
	want := []string{"--input", "/tmp/source.laz", "--output", "/tmp/out.laz", "--raster-only", "-h", "--threads", "4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected normalized args:\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestResolveProgramPrebuiltHitsPrebuiltArtifact(t *testing.T) {
	programRoot := filepath.Join(t.TempDir(), "program")
	layout := runtimeenv.Layout{Root: filepath.Join(t.TempDir(), "data"), ProgramRoot: programRoot}

	req := builder.BuildRequest{
		ToolID:     "recursive_content_dir_diff",
		Kind:       builder.KindGo,
		TargetOS:   runtime.GOOS,
		TargetArch: runtime.GOARCH,
	}
	artifactPath, _ := builder.ResolveProgramToolPath(filepath.Join(programRoot, "tools"), req)
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(artifactPath, []byte("bin"), 0755); err != nil {
		t.Fatalf("write artifact: %v", err)
	}

	got, ok := resolveProgramPrebuilt(layout, "recursive_content_dir_diff", builder.KindGo, runtime.GOOS, runtime.GOARCH)
	if !ok {
		t.Fatal("expected prebuilt artifact to be found")
	}
	if got != artifactPath {
		t.Fatalf("unexpected path: got=%s want=%s", got, artifactPath)
	}
}

func TestResolveProgramPrebuiltFallsBackWhenNonPortable(t *testing.T) {
	layout := runtimeenv.Layout{Root: t.TempDir()}
	if _, ok := resolveProgramPrebuilt(layout, "recursive_content_dir_diff", builder.KindGo, runtime.GOOS, runtime.GOARCH); ok {
		t.Fatal("非便携态不应命中预置产物")
	}
}
