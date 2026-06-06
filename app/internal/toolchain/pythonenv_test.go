package toolchain

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"fire-salamander-desktop/internal/appconfig"
	"my_tools/libs/core/toolspec"
)

func TestInspectPythonConfigRequiresManagedEnvironment(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-based fake python is only used on unix-like systems")
	}

	binaryPath := writeFakePythonBaseBinary(t, fakePythonOptions{})
	state, err := InspectPythonConfig(PythonConfig{
		SelectedBinary: binaryPath,
	})
	if err != nil {
		t.Fatalf("InspectPythonConfig failed: %v", err)
	}
	if !state.HasUsableBaseBinary {
		t.Fatal("expected usable base python binary")
	}
	if state.HasUsableBinary {
		t.Fatal("expected managed environment to be absent before prepare")
	}
	if !state.NeedsRebuild {
		t.Fatal("expected managed environment to require creation")
	}
	if !strings.Contains(state.StatusMessage, "创建托管 Python 工具环境") {
		t.Fatalf("unexpected status: %s", state.StatusMessage)
	}
}

func TestResolvePythonBinaryForToolRequiresDependencies(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-based fake python is only used on unix-like systems")
	}

	binaryPath := writeFakePythonBaseBinary(t, fakePythonOptions{})
	configureTempPythonConfig(t, PythonConfig{
		SelectedBinary: binaryPath,
	})
	if _, err := PrepareManagedPythonEnvironment(); err != nil {
		t.Fatalf("PrepareManagedPythonEnvironment failed: %v", err)
	}

	_, err := ResolvePythonBinaryForTool("restore_pcd_by_mgrs")
	if err == nil {
		t.Fatal("expected missing dependency error")
	}
	if !strings.Contains(err.Error(), "依赖未安装") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInstallPythonDependenciesMarksStateReady(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-based fake python is only used on unix-like systems")
	}

	binaryPath := writeFakePythonBaseBinary(t, fakePythonOptions{})
	configureTempPythonConfig(t, PythonConfig{
		SelectedBinary: binaryPath,
	})

	state, err := InstallPythonDependencies()
	if err != nil {
		t.Fatalf("InstallPythonDependencies failed: %v", err)
	}
	if !state.DependenciesReady {
		t.Fatal("expected dependencies to be ready after install")
	}
	if !state.HasUsableBinary {
		t.Fatal("expected managed environment binary to exist")
	}
}

func TestInspectPythonConfigRespectsDisabledState(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-based fake python is only used on unix-like systems")
	}

	binaryPath := writeFakePythonBaseBinary(t, fakePythonOptions{dependenciesInstalled: true})
	state, err := InspectPythonConfig(PythonConfig{
		SelectedBinary: binaryPath,
		Disabled:       true,
	})
	if err != nil {
		t.Fatalf("InspectPythonConfig failed: %v", err)
	}
	if state.HasUsableBinary {
		t.Fatal("expected disabled config to suppress active python")
	}
	if state.ActiveBinary != "" {
		t.Fatalf("expected no active binary, got %s", state.ActiveBinary)
	}
}

func TestPrepareManagedPythonEnvironmentCreatesManagedEnv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-based fake python is only used on unix-like systems")
	}

	binaryPath := writeFakePythonBaseBinary(t, fakePythonOptions{})
	configureTempPythonConfig(t, PythonConfig{
		SelectedBinary: binaryPath,
	})

	state, err := PrepareManagedPythonEnvironment()
	if err != nil {
		t.Fatalf("PrepareManagedPythonEnvironment failed: %v", err)
	}
	if !state.HasUsableBinary {
		t.Fatal("expected managed environment to be ready")
	}
	if state.NeedsRebuild {
		t.Fatal("expected managed environment to stop requiring rebuild")
	}
	if !strings.Contains(state.ActiveBinary, string(filepath.Separator)+"bin"+string(filepath.Separator)+"python") {
		t.Fatalf("unexpected managed python path: %s", state.ActiveBinary)
	}
}

func TestInspectPythonCandidateRejectsPython2(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-based fake python is only used on unix-like systems")
	}

	binaryPath := writeFakePythonBaseBinary(t, fakePythonOptions{
		version:      "Python 2.7.18",
		supportsVenv: false,
	})
	candidate := inspectPythonCandidate(binaryPath, sourceConfigured, binaryPath)
	if candidate.Valid {
		t.Fatal("expected python2 candidate to be rejected")
	}
	if !strings.Contains(candidate.Error, "仅支持 Python 3") {
		t.Fatalf("unexpected candidate error: %s", candidate.Error)
	}
}

func TestPythonExecutableNamesIncludesPython312(t *testing.T) {
	names := pythonExecutableNames()
	foundPython3 := false
	foundPython312 := false
	for _, name := range names {
		if name == "python3" || name == "python3.exe" {
			foundPython3 = true
		}
		if name == "python3.12" || name == "python3.12.exe" {
			foundPython312 = true
		}
	}
	if !foundPython3 {
		t.Fatal("expected python3 to be preferred in candidate names")
	}
	if !foundPython312 {
		t.Fatal("expected python3.12 variant to be scanned")
	}
}

func TestCollectPythonCandidatePathsDedupesSymlinkAliases(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink path expectations differ on windows")
	}

	dir := t.TempDir()
	target := filepath.Join(dir, "python-real")
	alias := filepath.Join(dir, "python-link")
	if err := os.WriteFile(target, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write target failed: %v", err)
	}
	if err := os.Symlink(target, alias); err != nil {
		t.Fatalf("symlink failed: %v", err)
	}

	paths := collectPythonCandidatePaths(PythonConfig{
		SelectedBinary: alias,
		KnownBinaries:  []string{target},
	})
	matches := 0
	for _, entry := range paths {
		if sameFilePath(entry.path, alias) {
			matches++
			if entry.path != alias {
				t.Fatalf("expected first path to win, got %q", entry.path)
			}
		}
	}
	if matches != 1 {
		t.Fatalf("expected symlink aliases to collapse into one candidate, got %d matches in %#v", matches, paths)
	}
}

func TestManagedMetadataLoadMissing(t *testing.T) {
	dir := t.TempDir()
	metadata, err := loadManagedPythonMetadata(dir)
	if err != nil {
		t.Fatalf("loadManagedPythonMetadata failed: %v", err)
	}
	if metadata != nil {
		t.Fatal("expected nil metadata for missing file")
	}
}

func TestReadPythonVersionInfoParsesVersion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-based fake python is only used on unix-like systems")
	}

	binaryPath := writeFakePythonBaseBinary(t, fakePythonOptions{version: "Python 3.12.7"})
	info, err := readPythonVersionInfo(binaryPath)
	if err != nil {
		t.Fatalf("readPythonVersionInfo failed: %v", err)
	}
	if info.Major != 3 || info.Minor != 12 || info.Patch != 7 {
		t.Fatalf("unexpected version info: %#v", info)
	}
}

func TestManagedPythonBinaryPathUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-only path expectation")
	}

	got := managedPythonBinaryPath("/tmp/example")
	if got != "/tmp/example/bin/python" {
		t.Fatalf("unexpected managed python path: %s", got)
	}
}

func TestCommandErrorDetailFallsBackToError(t *testing.T) {
	err := fmt.Errorf("boom")
	if got := commandErrorDetail(err, nil); got != "boom" {
		t.Fatalf("unexpected error detail: %s", got)
	}
}

func TestLoadPythonConfigRoundTrip(t *testing.T) {
	cfg := PythonConfig{
		SelectedBinary: "/tmp/python3",
		KnownBinaries:  []string{"/tmp/python3", "/tmp/python3.12"},
	}
	configureTempPythonConfig(t, cfg)
	loaded, err := LoadPythonConfig()
	if err != nil {
		t.Fatalf("LoadPythonConfig failed: %v", err)
	}
	if loaded.SelectedBinary != cfg.SelectedBinary {
		t.Fatalf("unexpected selected binary: %s", loaded.SelectedBinary)
	}
}

func TestInstallPythonDependenciesAutoPreparesManagedEnvironment(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-based fake python is only used on unix-like systems")
	}

	binaryPath := writeFakePythonBaseBinary(t, fakePythonOptions{})
	configureTempPythonConfig(t, PythonConfig{
		SelectedBinary: binaryPath,
	})
	state, err := InstallPythonDependencies()
	if err != nil {
		t.Fatalf("InstallPythonDependencies failed: %v", err)
	}
	if !state.HasUsableBinary || !state.DependenciesReady {
		t.Fatalf("expected install to prepare environment and install deps, state=%#v", state)
	}
}

func TestPrepareManagedPythonEnvironmentWritesMetadata(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-based fake python is only used on unix-like systems")
	}

	binaryPath := writeFakePythonBaseBinary(t, fakePythonOptions{})
	configureTempPythonConfig(t, PythonConfig{
		SelectedBinary: binaryPath,
	})
	state, err := PrepareManagedPythonEnvironment()
	if err != nil {
		t.Fatalf("PrepareManagedPythonEnvironment failed: %v", err)
	}
	metadata, err := loadManagedPythonMetadata(state.ManagedEnvDirectory)
	if err != nil {
		t.Fatalf("loadManagedPythonMetadata failed: %v", err)
	}
	if metadata == nil {
		t.Fatal("expected managed environment metadata")
	}
	if !sameFilePath(metadata.BaseBinary, binaryPath) {
		t.Fatalf("unexpected metadata base binary: %s", metadata.BaseBinary)
	}
}

func TestPrepareManagedPythonEnvironmentCreatesExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-based fake python is only used on unix-like systems")
	}

	binaryPath := writeFakePythonBaseBinary(t, fakePythonOptions{})
	configureTempPythonConfig(t, PythonConfig{
		SelectedBinary: binaryPath,
	})
	state, err := PrepareManagedPythonEnvironment()
	if err != nil {
		t.Fatalf("PrepareManagedPythonEnvironment failed: %v", err)
	}
	cmd := exec.Command(state.ActiveBinary, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("managed python should be executable: %v", err)
	}
	if !strings.Contains(string(output), "Python 3.12.5") {
		t.Fatalf("unexpected managed python version: %s", string(output))
	}
}

func TestPrepareManagedPythonEnvironmentRebuildsWhenBaseChanges(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-based fake python is only used on unix-like systems")
	}

	first := writeFakePythonBaseBinary(t, fakePythonOptions{version: "Python 3.11.9"})
	second := writeFakePythonBaseBinary(t, fakePythonOptions{version: "Python 3.12.5"})
	configureTempPythonConfig(t, PythonConfig{
		SelectedBinary: first,
	})
	if _, err := PrepareManagedPythonEnvironment(); err != nil {
		t.Fatalf("PrepareManagedPythonEnvironment failed: %v", err)
	}
	if err := SavePythonConfig(PythonConfig{
		SelectedBinary: second,
		KnownBinaries:  []string{first, second},
	}); err != nil {
		t.Fatalf("SavePythonConfig failed: %v", err)
	}
	state, err := GetPythonState()
	if err != nil {
		t.Fatalf("GetPythonState failed: %v", err)
	}
	if !state.NeedsRebuild {
		t.Fatal("expected managed environment to require rebuild after base change")
	}
	if !sameFilePath(state.ActiveBaseBinary, second) {
		t.Fatalf("expected second base binary to be active, got %s", state.ActiveBaseBinary)
	}
}

func TestPrepareManagedPythonEnvironmentKeepsOtherBaseEnv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-based fake python is only used on unix-like systems")
	}

	first := writeFakePythonBaseBinary(t, fakePythonOptions{version: "Python 3.11.9"})
	second := writeFakePythonBaseBinary(t, fakePythonOptions{version: "Python 3.12.5"})
	configureTempPythonConfig(t, PythonConfig{
		SelectedBinary: first,
	})
	firstState, err := PrepareManagedPythonEnvironment()
	if err != nil {
		t.Fatalf("PrepareManagedPythonEnvironment failed: %v", err)
	}
	firstEnv := firstState.ManagedEnvDirectory
	if err := SavePythonConfig(PythonConfig{
		SelectedBinary: second,
		KnownBinaries:  []string{first, second},
	}); err != nil {
		t.Fatalf("SavePythonConfig failed: %v", err)
	}
	secondState, err := PrepareManagedPythonEnvironment()
	if err != nil {
		t.Fatalf("PrepareManagedPythonEnvironment failed for second env: %v", err)
	}
	if firstEnv == secondState.ManagedEnvDirectory {
		t.Fatalf("expected different managed env directories, got %s", firstEnv)
	}
	if !isExistingFile(managedPythonBinaryPath(firstEnv)) {
		t.Fatalf("expected first managed env to remain on disk: %s", firstEnv)
	}
}

func TestGetPythonStateReusesExistingEnvironmentWhenSwitchingBack(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-based fake python is only used on unix-like systems")
	}

	first := writeFakePythonBaseBinary(t, fakePythonOptions{version: "Python 3.11.9"})
	second := writeFakePythonBaseBinary(t, fakePythonOptions{version: "Python 3.12.5"})
	configureTempPythonConfig(t, PythonConfig{
		SelectedBinary: first,
	})
	firstState, err := InstallPythonDependencies()
	if err != nil {
		t.Fatalf("InstallPythonDependencies failed for first env: %v", err)
	}
	firstEnv := firstState.ManagedEnvDirectory
	if err := SavePythonConfig(PythonConfig{
		SelectedBinary: second,
		KnownBinaries:  []string{first, second},
	}); err != nil {
		t.Fatalf("SavePythonConfig failed: %v", err)
	}
	if _, err := PrepareManagedPythonEnvironment(); err != nil {
		t.Fatalf("PrepareManagedPythonEnvironment failed for second env: %v", err)
	}
	if err := SavePythonConfig(PythonConfig{
		SelectedBinary: first,
		KnownBinaries:  []string{first, second},
	}); err != nil {
		t.Fatalf("SavePythonConfig failed: %v", err)
	}
	state, err := GetPythonState()
	if err != nil {
		t.Fatalf("GetPythonState failed: %v", err)
	}
	if state.NeedsRebuild {
		t.Fatal("expected switching back to existing env to avoid rebuild")
	}
	if !state.DependenciesReady {
		t.Fatal("expected reused environment to keep installed dependencies")
	}
	if state.ManagedEnvDirectory != firstEnv {
		t.Fatalf("expected reused env %s, got %s", firstEnv, state.ManagedEnvDirectory)
	}
}

func TestInstallPythonDependenciesReportsInstallProgressWhenPreparing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-based fake python is only used on unix-like systems")
	}

	binaryPath := writeFakePythonBaseBinary(t, fakePythonOptions{})
	configureTempPythonConfig(t, PythonConfig{
		SelectedBinary: binaryPath,
	})
	kinds := []PythonOperationKind{}
	percents := []float64{}
	_, err := InstallPythonDependenciesWithOptions(context.Background(), &PythonOperationHooks{
		OnProgress: func(progress PythonOperationProgress) {
			kinds = append(kinds, progress.Kind)
			percents = append(percents, progress.ProgressPercent)
		},
	})
	if err != nil {
		t.Fatalf("InstallPythonDependenciesWithOptions failed: %v", err)
	}
	if len(kinds) == 0 {
		t.Fatal("expected progress events")
	}
	for _, kind := range kinds {
		if kind != PythonOperationInstall {
			t.Fatalf("expected install progress kind, got %s", kind)
		}
	}
	if percents[0] != 0 {
		t.Fatalf("expected install progress to start from 0, got %v", percents[0])
	}
	foundAnalyze := false
	for index, percent := range percents {
		if index == 0 {
			continue
		}
		if percent == 45 {
			foundAnalyze = true
			break
		}
	}
	if !foundAnalyze {
		t.Fatalf("expected prepared install flow to include analysis progress at 45, got %v", percents)
	}
}

func TestInstallPythonDependenciesKeepsExistingEnvProgressLowBeforeInstall(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-based fake python is only used on unix-like systems")
	}

	binaryPath := writeFakePythonBaseBinary(t, fakePythonOptions{})
	configureTempPythonConfig(t, PythonConfig{
		SelectedBinary: binaryPath,
	})
	if _, err := PrepareManagedPythonEnvironment(); err != nil {
		t.Fatalf("PrepareManagedPythonEnvironment failed: %v", err)
	}

	var analyzePercent float64 = -1
	var firstInstallingPercent float64 = -1
	_, err := InstallPythonDependenciesWithOptions(context.Background(), &PythonOperationHooks{
		OnProgress: func(progress PythonOperationProgress) {
			switch progress.Message {
			case "解析缺失依赖":
				analyzePercent = progress.ProgressPercent
			}
			if strings.HasPrefix(progress.Message, "安装依赖 ") && firstInstallingPercent < 0 {
				firstInstallingPercent = progress.ProgressPercent
			}
		},
	})
	if err != nil {
		t.Fatalf("InstallPythonDependenciesWithOptions failed: %v", err)
	}
	if analyzePercent != 10 {
		t.Fatalf("expected existing env analysis progress to stay low at 10, got %v", analyzePercent)
	}
	if firstInstallingPercent != 20 {
		t.Fatalf("expected first install step to start at 20, got %v", firstInstallingPercent)
	}
}

func TestCheckManagedPythonEnvironmentReportsReady(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-based fake python is only used on unix-like systems")
	}

	binaryPath := writeFakePythonBaseBinary(t, fakePythonOptions{})
	configureTempPythonConfig(t, PythonConfig{
		SelectedBinary: binaryPath,
	})
	if _, err := InstallPythonDependencies(); err != nil {
		t.Fatalf("InstallPythonDependencies failed: %v", err)
	}
	state, err := CheckManagedPythonEnvironment()
	if err != nil {
		t.Fatalf("CheckManagedPythonEnvironment failed: %v", err)
	}
	if !state.DependenciesReady {
		t.Fatal("expected checked environment to be ready")
	}
}

func TestDeleteManagedPythonEnvironmentRemovesCurrentEnv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-based fake python is only used on unix-like systems")
	}

	binaryPath := writeFakePythonBaseBinary(t, fakePythonOptions{})
	configureTempPythonConfig(t, PythonConfig{
		SelectedBinary: binaryPath,
	})
	state, err := PrepareManagedPythonEnvironment()
	if err != nil {
		t.Fatalf("PrepareManagedPythonEnvironment failed: %v", err)
	}
	envDir := state.ManagedEnvDirectory
	state, err = DeleteManagedPythonEnvironment()
	if err != nil {
		t.Fatalf("DeleteManagedPythonEnvironment failed: %v", err)
	}
	if isExistingFile(managedPythonBinaryPath(envDir)) {
		t.Fatalf("expected managed env to be removed: %s", envDir)
	}
	if state.HasUsableBinary {
		t.Fatal("expected no usable managed env after deletion")
	}
	if !state.NeedsRebuild {
		t.Fatal("expected deleted env to require rebuild")
	}
}

func TestBuildPythonRequirementManifestScansThirdPartyImports(t *testing.T) {
	manifest := buildPythonRequirementManifestWithLoader([]toolspec.ToolManifest{
		{
			ID:   "demo_tool",
			Kind: toolspec.ToolKindPython,
			Source: toolspec.SourceSpec{
				Entry: "tools/python_tools/scripts/demo_tool.py",
			},
		},
	}, func(string) ([]byte, error) {
		script := strings.Join([]string{
			"import os",
			"import numpy as np",
			"import rich",
			"from pandas import DataFrame",
			"from concurrent.futures import ThreadPoolExecutor",
		}, "\n")
		return []byte(script), nil
	})
	tool, ok := manifest.Tools["demo_tool"]
	if !ok {
		t.Fatalf("expected demo_tool requirements to be present, got %#v", manifest.Tools)
	}
	if len(tool.Packages) != 3 {
		t.Fatalf("expected three scanned packages, got %#v", tool.Packages)
	}
	names := []string{tool.Packages[0].Name, tool.Packages[1].Name, tool.Packages[2].Name}
	if names[0] != "numpy" || names[1] != "rich" || names[2] != "pandas" {
		t.Fatalf("unexpected scanned package names: %#v", names)
	}
	modules := []string{tool.Packages[0].Module, tool.Packages[1].Module, tool.Packages[2].Module}
	if modules[0] != "numpy" || modules[1] != "rich" || modules[2] != "pandas" {
		t.Fatalf("unexpected scanned module names: %#v", modules)
	}
}

func TestBuildPythonRequirementManifestMapsKnownPackageAliases(t *testing.T) {
	manifest := buildPythonRequirementManifestWithLoader([]toolspec.ToolManifest{
		{
			ID:   "demo_tool",
			Kind: toolspec.ToolKindPython,
			Source: toolspec.SourceSpec{
				Entry: "tools/python_tools/scripts/demo_tool.py",
			},
		},
	}, func(string) ([]byte, error) {
		script := strings.Join([]string{
			"from PIL import Image",
			"import yaml",
			"import cv2",
			"from sklearn import metrics",
		}, "\n")
		return []byte(script), nil
	})
	tool, ok := manifest.Tools["demo_tool"]
	if !ok {
		t.Fatalf("expected demo_tool requirements to be present, got %#v", manifest.Tools)
	}
	if len(tool.Packages) != 4 {
		t.Fatalf("expected four scanned packages, got %#v", tool.Packages)
	}
	expected := []pythonPackageSpec{
		{Name: "Pillow", Module: "PIL"},
		{Name: "PyYAML", Module: "yaml"},
		{Name: "opencv-python", Module: "cv2"},
		{Name: "scikit_learn", Module: "sklearn"},
	}
	for index, pkg := range tool.Packages {
		if pkg != expected[index] {
			t.Fatalf("unexpected package mapping at %d: got %#v want %#v", index, pkg, expected[index])
		}
	}
}

func TestParsePythonImportMappingAndStdlibModules(t *testing.T) {
	aliases := parsePythonImportMapping(strings.Join([]string{
		"PIL:Pillow",
		"bs4:beautifulsoup4",
		"",
		"# comment",
	}, "\n"))
	if aliases["PIL"] != "Pillow" {
		t.Fatalf("expected PIL alias to be loaded, got %#v", aliases)
	}
	if aliases["bs4"] != "beautifulsoup4" {
		t.Fatalf("expected bs4 alias to be loaded, got %#v", aliases)
	}

	stdlib := parsePythonStdlibModules(strings.Join([]string{
		"os",
		"json",
		"",
		"# comment",
	}, "\n"))
	if _, ok := stdlib["os"]; !ok {
		t.Fatalf("expected os to be marked as stdlib, got %#v", stdlib)
	}
	if _, ok := stdlib["json"]; !ok {
		t.Fatalf("expected json to be marked as stdlib, got %#v", stdlib)
	}
	if _, ok := stdlib["requests"]; ok {
		t.Fatalf("did not expect requests to be marked as stdlib, got %#v", stdlib)
	}
}

func configureTempPythonConfig(t *testing.T, cfg PythonConfig) string {
	t.Helper()
	runtimeDir := filepath.Join(t.TempDir(), "runtime")
	t.Setenv("FIRE_SALAMANDER_RUNTIME_DIR", runtimeDir)
	configPath, err := appconfig.ResolveConfigPath()
	if err != nil {
		t.Fatalf("ResolveConfigPath failed: %v", err)
	}
	if err := SavePythonConfig(cfg); err != nil {
		t.Fatalf("SavePythonConfig failed: %v", err)
	}
	return configPath
}

type fakePythonOptions struct {
	version               string
	supportsVenv          bool
	dependenciesInstalled bool
}

func writeFakePythonBaseBinary(t *testing.T, options fakePythonOptions) string {
	t.Helper()
	dir := t.TempDir()
	if strings.TrimSpace(options.version) == "" {
		options.version = "Python 3.12.5"
	}
	if !options.supportsVenv {
		options.supportsVenv = true
	}
	baseScriptPath := filepath.Join(dir, "python3")
	versionLiteral := shellQuote(options.version)
	supportsVenv := "0"
	if options.supportsVenv {
		supportsVenv = "1"
	}
	dependenciesInstalled := "0"
	if options.dependenciesInstalled {
		dependenciesInstalled = "1"
	}
	script := strings.Join([]string{
		"#!/bin/sh",
		"VERSION=" + versionLiteral,
		"SUPPORTS_VENV=\"" + supportsVenv + "\"",
		"DEFAULT_INSTALLED=\"" + dependenciesInstalled + "\"",
		"if [ \"$1\" = \"--version\" ] || [ \"$1\" = \"-V\" ]; then",
		"  echo \"$VERSION\"",
		"  exit 0",
		"fi",
		"if [ \"$1\" = \"-c\" ]; then",
		"  case \"$2\" in",
		"    *\"import venv\"*)",
		"      if [ \"$SUPPORTS_VENV\" = \"1\" ]; then",
		"        exit 0",
		"      fi",
		"      exit 1",
		"      ;;",
		"  esac",
		"fi",
		"if [ \"$1\" = \"-m\" ] && [ \"$2\" = \"venv\" ]; then",
		"  if [ \"$SUPPORTS_VENV\" != \"1\" ]; then",
		"    echo \"venv unavailable\" >&2",
		"    exit 1",
		"  fi",
		"  TARGET=\"$3\"",
		"  mkdir -p \"$TARGET/bin\"",
		"  cat > \"$TARGET/bin/python\" <<'PYEOF'",
		managedPythonScript(options.version),
		"PYEOF",
		"  chmod +x \"$TARGET/bin/python\"",
		"  if [ \"$DEFAULT_INSTALLED\" = \"1\" ]; then",
		"    touch \"$TARGET/deps-installed\"",
		"  fi",
		"  exit 0",
		"fi",
		"exit 1",
	}, "\n")
	if err := os.WriteFile(baseScriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("write fake base python failed: %v", err)
	}
	return baseScriptPath
}

func managedPythonScript(version string) string {
	versionLiteral := shellQuote(version)
	return strings.Join([]string{
		"#!/bin/sh",
		"VERSION=" + versionLiteral,
		"ROOT=$(CDPATH= cd -- \"$(dirname \"$0\")/..\" && pwd)",
		"MARKER=\"$ROOT/deps-installed\"",
		"if [ \"$1\" = \"--version\" ] || [ \"$1\" = \"-V\" ]; then",
		"  echo \"$VERSION\"",
		"  exit 0",
		"fi",
		"if [ \"$1\" = \"-m\" ] && [ \"$2\" = \"pip\" ] && [ \"$3\" = \"--version\" ]; then",
		"  echo \"pip 24.0\"",
		"  exit 0",
		"fi",
		"if [ \"$1\" = \"-m\" ] && [ \"$2\" = \"ensurepip\" ] && [ \"$3\" = \"--upgrade\" ]; then",
		"  exit 0",
		"fi",
		"if [ \"$1\" = \"-m\" ] && [ \"$2\" = \"pip\" ] && [ \"$3\" = \"install\" ]; then",
		"  touch \"$MARKER\"",
		"  exit 0",
		"fi",
		"if [ \"$1\" = \"-c\" ]; then",
		"  case \"$2\" in",
		"    *\"import venv\"*)",
		"      exit 0",
		"      ;;",
		"  esac",
		"  if [ -f \"$MARKER\" ]; then",
		"    cat <<'EOF'",
		fakeDependencyProbeJSON(true),
		"EOF",
		"  else",
		"    cat <<'EOF'",
		fakeDependencyProbeJSON(false),
		"EOF",
		"  fi",
		"  exit 0",
		"fi",
		"exit 1",
	}, "\n")
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\"'\"'`) + "'"
}

func fakeDependencyProbeJSON(installed bool) string {
	version := ""
	if installed {
		version = "1.0.0"
	}
	rows := []string{
		fmt.Sprintf(`{"name":"beautifulsoup4","module":"bs4","installed":%s,"version":"%s","error":""}`, boolLiteral(installed), version),
		fmt.Sprintf(`{"name":"open3d","module":"open3d","installed":%s,"version":"%s","error":""}`, boolLiteral(installed), version),
		fmt.Sprintf(`{"name":"laspy","module":"laspy","installed":%s,"version":"%s","error":""}`, boolLiteral(installed), version),
		fmt.Sprintf(`{"name":"numpy","module":"numpy","installed":%s,"version":"%s","error":""}`, boolLiteral(installed), version),
		fmt.Sprintf(`{"name":"mgrs","module":"mgrs","installed":%s,"version":"%s","error":""}`, boolLiteral(installed), version),
		fmt.Sprintf(`{"name":"Pillow","module":"PIL","installed":%s,"version":"%s","error":""}`, boolLiteral(installed), version),
		fmt.Sprintf(`{"name":"pyproj","module":"pyproj","installed":%s,"version":"%s","error":""}`, boolLiteral(installed), version),
		fmt.Sprintf(`{"name":"Requests","module":"requests","installed":%s,"version":"%s","error":""}`, boolLiteral(installed), version),
	}
	return "[" + strings.Join(rows, ",") + "]"
}

func boolLiteral(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
