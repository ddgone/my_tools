package main

import (
	"strings"
	"testing"
	"time"
)

func TestEnsureToolingLoadsLegacyTools(t *testing.T) {
	app := NewApp()
	if err := app.ensureTooling(); err != nil {
		t.Fatalf("ensureTooling failed: %v", err)
	}

	if len(app.legacy) < 4 {
		t.Fatalf("expected at least 4 legacy tools, got %d", len(app.legacy))
	}

	if _, ok := app.manifests["geojson_to_shp"]; !ok {
		t.Fatalf("expected geojson_to_shp manifest to be available")
	}
}

func TestStartLocalExecutionRunsLegacyTool(t *testing.T) {
	app := NewApp()
	if err := app.ensureTooling(); err != nil {
		t.Fatalf("ensureTooling failed: %v", err)
	}

	tempDir := t.TempDir()
	task, err := app.StartLocalExecution(ExecutionRequest{
		ToolID: "geojson_to_shp",
		Args:   `-input "` + tempDir + `"`,
	})
	if err != nil {
		t.Fatalf("StartLocalExecution failed: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		tasks := app.ListTasks()
		for _, current := range tasks {
			if current.ID != task.ID {
				continue
			}
			if current.Status == "running" {
				time.Sleep(50 * time.Millisecond)
				continue
			}
			if current.Status != "error" {
				t.Fatalf("expected error status for empty input dir, got %s", current.Status)
			}
			if !strings.Contains(current.ExitMessage, "未找到任何 GeoJSON 文件") {
				t.Fatalf("unexpected exit message: %s", current.ExitMessage)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("task %s did not finish before deadline", task.ID)
}

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
