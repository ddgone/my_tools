package execution

import (
	"strings"
	"testing"

	"my_tools/libs/core/toolspec"
)

func TestBuildRemoteRunCommandForGoTool(t *testing.T) {
	cmd, chmodCmd, err := buildRemoteRunCommand("/tmp/fire-salamander-abcd/geojson_to_shp_linux_amd64", remoteExecParams{
		kind: "go",
		args: `-input "/data/demo file.geojson" -workers 4`,
	})
	if err != nil {
		t.Fatalf("buildRemoteRunCommand failed: %v", err)
	}

	expectedCmd := "cd '/tmp/fire-salamander-abcd' && './geojson_to_shp_linux_amd64' '-input' '/data/demo file.geojson' '-workers' '4'"
	if cmd != expectedCmd {
		t.Fatalf("unexpected go command:\nwant: %s\ngot:  %s", expectedCmd, cmd)
	}

	expectedChmod := "chmod +x '/tmp/fire-salamander-abcd/geojson_to_shp_linux_amd64'"
	if chmodCmd != expectedChmod {
		t.Fatalf("unexpected chmod command:\nwant: %s\ngot:  %s", expectedChmod, chmodCmd)
	}
}

func TestBuildRemoteRunCommandForPythonTool(t *testing.T) {
	cmd, chmodCmd, err := buildRemoteRunCommand("/tmp/fire-salamander-abcd/python_env_diagnostics.py", remoteExecParams{
		kind:      "python",
		pythonEnv: "python3",
		args:      `-input "/data/source dir"`,
	})
	if err != nil {
		t.Fatalf("buildRemoteRunCommand failed: %v", err)
	}

	expectedCmd := "cd '/tmp/fire-salamander-abcd' && 'python3' './python_env_diagnostics.py' '-input' '/data/source dir'"
	if cmd != expectedCmd {
		t.Fatalf("unexpected python command:\nwant: %s\ngot:  %s", expectedCmd, cmd)
	}

	if chmodCmd != "" {
		t.Fatalf("python command should not need chmod, got %s", chmodCmd)
	}
}

func TestBuildRemoteRunCommandForRustTool(t *testing.T) {
	cmd, chmodCmd, err := buildRemoteRunCommand("/tmp/fire-salamander-abcd/bxn_delivery_point_cloud_qc_linux_amd64", remoteExecParams{
		kind: "rust",
		args: `-input "/data/source.laz" -output "/data/output.laz" -raster-only`,
	})
	if err != nil {
		t.Fatalf("buildRemoteRunCommand failed: %v", err)
	}

	expectedCmd := "cd '/tmp/fire-salamander-abcd' && './bxn_delivery_point_cloud_qc_linux_amd64' '--input' '/data/source.laz' '--output' '/data/output.laz' '--raster-only'"
	if cmd != expectedCmd {
		t.Fatalf("unexpected rust command:\nwant: %s\ngot:  %s", expectedCmd, cmd)
	}

	expectedChmod := "chmod +x '/tmp/fire-salamander-abcd/bxn_delivery_point_cloud_qc_linux_amd64'"
	if chmodCmd != expectedChmod {
		t.Fatalf("unexpected chmod command:\nwant: %s\ngot:  %s", expectedChmod, chmodCmd)
	}
}

func TestBuildRemoteRunCommandRejectsMalformedArgs(t *testing.T) {
	_, _, err := buildRemoteRunCommand("/tmp/fire-salamander-abcd/demo", remoteExecParams{
		kind: "go",
		args: `-input "unterminated`,
	})
	if err == nil {
		t.Fatal("expected malformed args to return error")
	}
	if !strings.Contains(err.Error(), "双引号未闭合") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveRemoteResultHintForOutputDirectory(t *testing.T) {
	hint, err := resolveRemoteResultHint([]toolspec.ParameterSpec{
		{
			Key:    "input",
			ArgKey: "input",
			Type:   toolspec.FieldTypePath,
		},
		{
			Key:    "output",
			ArgKey: "output",
			Type:   toolspec.FieldTypePath,
		},
	}, `-input /data/source -output output`, "/tmp/fire-salamander-123")
	if err != nil {
		t.Fatalf("resolveRemoteResultHint failed: %v", err)
	}
	if hint.Path != "/tmp/fire-salamander-123/output" {
		t.Fatalf("unexpected path: %s", hint.Path)
	}
	if hint.Kind != "directory" {
		t.Fatalf("unexpected kind: %s", hint.Kind)
	}
}

func TestResolveRemoteResultHintForOutputFile(t *testing.T) {
	hint, err := resolveRemoteResultHint([]toolspec.ParameterSpec{
		{
			Key:      "output",
			ArgKey:   "output",
			Type:     toolspec.FieldTypePath,
			PathMode: "file",
		},
	}, `-output report.json`, "/tmp/fire-salamander-123")
	if err != nil {
		t.Fatalf("resolveRemoteResultHint failed: %v", err)
	}
	if hint.Path != "/tmp/fire-salamander-123/report.json" {
		t.Fatalf("unexpected path: %s", hint.Path)
	}
	if hint.Kind != "file" {
		t.Fatalf("unexpected kind: %s", hint.Kind)
	}
}

func TestResolveRemoteResultHintWithoutOutputValue(t *testing.T) {
	hint, err := resolveRemoteResultHint([]toolspec.ParameterSpec{
		{
			Key:    "output",
			ArgKey: "output",
			Type:   toolspec.FieldTypePath,
		},
	}, `-input /data/source`, "/tmp/fire-salamander-123")
	if err != nil {
		t.Fatalf("resolveRemoteResultHint failed: %v", err)
	}
	if hint.Path != "" {
		t.Fatalf("expected empty path, got %s", hint.Path)
	}
}

func TestResolveRemoteResultHintWithDefaultOutputForFileInput(t *testing.T) {
	hint, err := resolveRemoteResultHint([]toolspec.ParameterSpec{
		{
			Key:      "input",
			ArgKey:   "input",
			Type:     toolspec.FieldTypePath,
			PathMode: "file",
		},
		{
			Key:         "output",
			ArgKey:      "output",
			Type:        toolspec.FieldTypePath,
			PathMode:    "directory",
			Placeholder: "留空则自动在输入位置生成 output",
		},
	}, `-input /data/source/report.geojson`, "/tmp/fire-salamander-123")
	if err != nil {
		t.Fatalf("resolveRemoteResultHint failed: %v", err)
	}
	if hint.Path != "/data/source/output" {
		t.Fatalf("unexpected path: %s", hint.Path)
	}
	if hint.Kind != "directory" {
		t.Fatalf("unexpected kind: %s", hint.Kind)
	}
}

func TestResolveRemoteResultHintWithDefaultOutputForDirectoryInput(t *testing.T) {
	hint, err := resolveRemoteResultHint([]toolspec.ParameterSpec{
		{
			Key:      "input",
			ArgKey:   "input",
			Type:     toolspec.FieldTypePath,
			PathMode: "directory",
		},
		{
			Key:      "output",
			ArgKey:   "output",
			Type:     toolspec.FieldTypePath,
			PathMode: "directory",
			Help:     "不填写时会沿用旧版默认行为，在输入目录下创建 output 目录。",
		},
	}, `-input /data/source-dir`, "/tmp/fire-salamander-123")
	if err != nil {
		t.Fatalf("resolveRemoteResultHint failed: %v", err)
	}
	if hint.Path != "/data/source-dir/output" {
		t.Fatalf("unexpected path: %s", hint.Path)
	}
}

func TestResolveRemoteResultHintWithDefaultOutputForFileOrDirectoryInput(t *testing.T) {
	hint, err := resolveRemoteResultHint([]toolspec.ParameterSpec{
		{
			Key:      "convertPath",
			ArgKey:   "convert",
			Type:     toolspec.FieldTypePath,
			PathMode: "fileOrDirectory",
		},
		{
			Key:      "output",
			ArgKey:   "output",
			Type:     toolspec.FieldTypePath,
			PathMode: "directory",
			Help:     "可选。默认在输入路径旁边创建 output 目录。",
		},
	}, `-convert /data/tracks/utm.txt`, "/tmp/fire-salamander-123")
	if err != nil {
		t.Fatalf("resolveRemoteResultHint failed: %v", err)
	}
	if hint.Path != "/data/tracks/output" {
		t.Fatalf("unexpected path: %s", hint.Path)
	}
}

func TestPathWithinRemoteBase(t *testing.T) {
	if !pathWithinRemoteBase("/tmp/fire-salamander-123", "/tmp/fire-salamander-123/output/result.json") {
		t.Fatal("expected result path to be within remote base")
	}
	if pathWithinRemoteBase("/tmp/fire-salamander-123", "/tmp/fire-salamander-1234/output") {
		t.Fatal("unexpected prefix-only match")
	}
}
