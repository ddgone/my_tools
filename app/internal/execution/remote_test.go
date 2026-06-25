package execution

import (
	"strings"
	"testing"
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
	cmd, chmodCmd, err := buildRemoteRunCommand("/tmp/fire-salamander-abcd/restore_pcd_by_mgrs.py", remoteExecParams{
		kind:      "python",
		pythonEnv: "python3",
		args:      `-input "/data/source dir"`,
	})
	if err != nil {
		t.Fatalf("buildRemoteRunCommand failed: %v", err)
	}

	expectedCmd := "cd '/tmp/fire-salamander-abcd' && 'python3' './restore_pcd_by_mgrs.py' '-input' '/data/source dir'"
	if cmd != expectedCmd {
		t.Fatalf("unexpected python command:\nwant: %s\ngot:  %s", expectedCmd, cmd)
	}

	if chmodCmd != "" {
		t.Fatalf("python command should not need chmod, got %s", chmodCmd)
	}
}

func TestBuildRemoteRunCommandForRustTool(t *testing.T) {
	cmd, chmodCmd, err := buildRemoteRunCommand("/tmp/fire-salamander-abcd/las_voxelizer_linux_amd64", remoteExecParams{
		kind: "rust",
		args: `-input "/data/source.laz" -output "/data/output.laz" -raster-only`,
	})
	if err != nil {
		t.Fatalf("buildRemoteRunCommand failed: %v", err)
	}

	expectedCmd := "cd '/tmp/fire-salamander-abcd' && './las_voxelizer_linux_amd64' '--input' '/data/source.laz' '--output' '/data/output.laz' '--raster-only'"
	if cmd != expectedCmd {
		t.Fatalf("unexpected rust command:\nwant: %s\ngot:  %s", expectedCmd, cmd)
	}

	expectedChmod := "chmod +x '/tmp/fire-salamander-abcd/las_voxelizer_linux_amd64'"
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
