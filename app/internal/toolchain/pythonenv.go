package toolchain

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"fire-salamander-desktop/internal/appconfig"
	"fire-salamander-desktop/internal/runtimeenv"
	"my_tools/libs/catalog/builtin"
	"my_tools/libs/core/toolspec"
	python_tools "my_tools/tools/python_tools"
)

const (
	configKeyPython         = "python"
	pythonToolchainDirName  = "python"
	pythonToolchainMetaName = ".fire-salamander-python.json"
)

// pipreqs data snapshot: import-name -> pip-package mapping.
// Keep this file in sync periodically when new Python tools or alias cases appear.
//
//go:embed pythondata/mapping.txt
var embeddedPythonImportMapping string

// pipreqs data snapshot: Python standard-library module list used for filtering.
// Keep this file in sync periodically to reduce false positives/negatives in dependency scanning.
//
//go:embed pythondata/stdlib.txt
var embeddedPythonStdlibModules string

type PythonConfig struct {
	SelectedBinary string   `json:"selectedBinary"`
	KnownBinaries  []string `json:"knownBinaries"`
	Disabled       bool     `json:"disabled"`
}

type PythonCandidate struct {
	Path     string `json:"path"`
	Version  string `json:"version"`
	Source   string `json:"source"`
	Label    string `json:"label"`
	Detail   string `json:"detail"`
	Error    string `json:"error,omitempty"`
	Valid    bool   `json:"valid"`
	Selected bool   `json:"selected"`
	Active   bool   `json:"active"`
}

type PythonDependency struct {
	PackageName string   `json:"packageName"`
	ModuleName  string   `json:"moduleName"`
	Installed   bool     `json:"installed"`
	Version     string   `json:"version,omitempty"`
	Error       string   `json:"error,omitempty"`
	RequiredBy  []string `json:"requiredBy"`
}

type PythonState struct {
	Config               PythonConfig       `json:"config"`
	Candidates           []PythonCandidate  `json:"candidates"`
	HasUsableBaseBinary  bool               `json:"hasUsableBaseBinary"`
	ActiveBaseBinary     string             `json:"activeBaseBinary"`
	ActiveBaseVersion    string             `json:"activeBaseVersion"`
	ActiveBaseSource     string             `json:"activeBaseSource"`
	HasUsableBinary      bool               `json:"hasUsableBinary"`
	ActiveBinary         string             `json:"activeBinary"`
	ActiveVersion        string             `json:"activeVersion"`
	ActiveSource         string             `json:"activeSource"`
	PipAvailable         bool               `json:"pipAvailable"`
	DependenciesReady    bool               `json:"dependenciesReady"`
	MissingPackages      []string           `json:"missingPackages"`
	StatusMessage        string             `json:"statusMessage"`
	Dependencies         []PythonDependency `json:"dependencies"`
	DependencyToolCount  int                `json:"dependencyToolCount"`
	DependencyTotalCount int                `json:"dependencyTotalCount"`
	ManagedEnvDirectory  string             `json:"managedEnvDirectory"`
	NeedsRebuild         bool               `json:"needsRebuild"`
	ManagedBaseBinary    string             `json:"managedBaseBinary"`
	ManagedBaseVersion   string             `json:"managedBaseVersion"`
}

type pythonRequirementManifest struct {
	Tools map[string]pythonToolRequirement `json:"tools"`
}

type pythonToolRequirement struct {
	Packages []pythonPackageSpec `json:"packages"`
}

type pythonPackageSpec struct {
	Name   string `json:"name"`
	Module string `json:"module"`
}

type pythonCandidatePath struct {
	path   string
	source string
}

type pythonDependencyProbe struct {
	Name      string `json:"name"`
	Module    string `json:"module"`
	Installed bool   `json:"installed"`
	Version   string `json:"version"`
	Error     string `json:"error"`
}

type pythonVersionInfo struct {
	Label string
	Major int
	Minor int
	Patch int
}

type managedPythonMetadata struct {
	BaseBinary   string `json:"baseBinary"`
	BaseIdentity string `json:"baseIdentity"`
	BaseVersion  string `json:"baseVersion"`
}

var (
	pythonImportPattern      = regexp.MustCompile(`^\s*import\s+([^#]+)`)
	pythonFromImportPattern  = regexp.MustCompile(`^\s*from\s+([A-Za-z_][A-Za-z0-9_\.]*)\s+import\b`)
	pythonDependencyDataOnce sync.Once
	pythonDependencyData     pythonDependencyScanData
)

type pythonDependencyScanData struct {
	modulePackageAliases map[string]string
	standardLibrary      map[string]struct{}
}

func LoadPythonConfig() (PythonConfig, error) {
	configPath, err := appconfig.ResolveConfigPath()
	if err != nil {
		return PythonConfig{}, err
	}
	doc, err := appconfig.LoadDocument(configPath)
	if err != nil {
		return PythonConfig{}, err
	}
	var cfg PythonConfig
	if raw, ok := doc[configKeyPython]; ok && len(raw) > 0 {
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return PythonConfig{}, fmt.Errorf("解析 Python 配置失败: %w", err)
		}
	}
	return normalizePythonConfig(cfg), nil
}

func SavePythonConfig(cfg PythonConfig) error {
	configPath, err := appconfig.ResolveConfigPath()
	if err != nil {
		return err
	}
	doc, err := appconfig.LoadDocument(configPath)
	if err != nil {
		return err
	}
	cfg = normalizePythonConfig(cfg)
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("序列化 Python 配置失败: %w", err)
	}
	doc[configKeyPython] = data
	return appconfig.WriteDocument(configPath, doc)
}

func GetPythonState() (PythonState, error) {
	cfg, err := LoadPythonConfig()
	if err != nil {
		return PythonState{}, err
	}
	return InspectPythonConfig(cfg)
}

func InspectPythonConfig(cfg PythonConfig) (PythonState, error) {
	cfg = normalizePythonConfig(cfg)
	paths := collectPythonCandidatePaths(cfg)
	candidates := make([]PythonCandidate, 0, len(paths))

	activeBaseBinary := ""
	activeBaseVersion := ""
	activeBaseSource := ""
	selectedError := ""

	for _, entry := range paths {
		candidate := inspectPythonCandidate(entry.path, entry.source, cfg.SelectedBinary)
		if candidate.Selected && !candidate.Valid && candidate.Error != "" {
			selectedError = candidate.Error
		}
		if !cfg.Disabled && activeBaseBinary == "" && candidate.Valid {
			activeBaseBinary = candidate.Path
			activeBaseVersion = candidate.Version
			activeBaseSource = candidate.Source
			candidate.Active = true
		}
		candidates = append(candidates, candidate)
	}

	managedEnvDirectory := ""
	activeBinary := ""
	activeVersion := ""
	activeSource := ""
	pipAvailable := false
	dependencies := []PythonDependency{}
	missingPackages := []string{}
	dependenciesReady := false
	dependencyToolCount := 0
	dependencyTotalCount := 0
	needsRebuild := false
	managedBaseBinary := ""
	managedBaseVersion := ""

	manifest, err := loadPythonRequirementManifest()
	if err != nil {
		return PythonState{}, err
	}
	dependencyToolCount = len(manifest.Tools)
	dependencyTotalCount = len(aggregatePythonRequirements(manifest))

	if layout, err := runtimeenv.ResolveLayout(); err == nil {
		if activeBaseBinary != "" {
			managedEnvDirectory = pythonManagedEnvDirectoryForBase(layout, activeBaseBinary)
		}
	}

	if !cfg.Disabled && activeBaseBinary != "" && managedEnvDirectory != "" {
		metadata, _ := loadManagedPythonMetadata(managedEnvDirectory)
		if metadata != nil {
			managedBaseBinary = strings.TrimSpace(metadata.BaseBinary)
			managedBaseVersion = strings.TrimSpace(metadata.BaseVersion)
		}
		managedBinary := managedPythonBinaryPath(managedEnvDirectory)
		hasManagedBinary := isExistingFile(managedBinary)
		matchesBase := metadata != nil && metadata.BaseIdentity != "" && metadata.BaseIdentity == fileIdentity(activeBaseBinary)
		switch {
		case hasManagedBinary && matchesBase:
			version, err := readPythonVersion(managedBinary)
			if err == nil {
				activeBinary = managedBinary
				activeVersion = version
				activeSource = sourceManaged
				dependencies, pipAvailable = inspectPythonDependencies(activeBinary, manifest)
				missingPackages = collectMissingPythonPackages(dependencies)
				dependenciesReady = pipAvailable && len(missingPackages) == 0
				dependencyTotalCount = len(dependencies)
			} else {
				needsRebuild = true
			}
		case hasManagedBinary || metadata != nil:
			needsRebuild = true
		case activeBaseBinary != "":
			needsRebuild = true
		}
	}

	statusMessage := ""
	switch {
	case cfg.Disabled:
		statusMessage = "当前未启用 Python 工具环境，可重新选择基础 Python"
	case activeBaseBinary == "":
		statusMessage = "未检测到可用的基础 Python，请先在系统设置 > Python 中选择 Python 3"
	case activeBinary == "" && needsRebuild && managedBaseBinary != "":
		statusMessage = "基础 Python 已变化，托管 Python 工具环境需要重建"
	case activeBinary == "" && needsRebuild:
		statusMessage = "已选择基础 Python，请先创建托管 Python 工具环境"
	case !pipAvailable:
		statusMessage = "托管 Python 工具环境缺少 pip，请重建工具环境"
	case len(missingPackages) > 0:
		statusMessage = fmt.Sprintf("托管 Python 工具环境仍缺少依赖：%s", strings.Join(missingPackages, "、"))
	case selectedError != "":
		statusMessage = fmt.Sprintf("已回退到自动检测的基础 Python；之前配置的路径不可用：%s", selectedError)
	default:
		statusMessage = "托管 Python 工具环境与动态扫描依赖已就绪"
	}

	return PythonState{
		Config:               cfg,
		Candidates:           candidates,
		HasUsableBaseBinary:  activeBaseBinary != "",
		ActiveBaseBinary:     activeBaseBinary,
		ActiveBaseVersion:    activeBaseVersion,
		ActiveBaseSource:     activeBaseSource,
		HasUsableBinary:      activeBinary != "",
		ActiveBinary:         activeBinary,
		ActiveVersion:        activeVersion,
		ActiveSource:         activeSource,
		PipAvailable:         pipAvailable,
		DependenciesReady:    dependenciesReady,
		MissingPackages:      missingPackages,
		StatusMessage:        statusMessage,
		Dependencies:         dependencies,
		DependencyToolCount:  dependencyToolCount,
		DependencyTotalCount: dependencyTotalCount,
		ManagedEnvDirectory:  managedEnvDirectory,
		NeedsRebuild:         needsRebuild,
		ManagedBaseBinary:    managedBaseBinary,
		ManagedBaseVersion:   managedBaseVersion,
	}, nil
}

func ResolvePythonBinary() (string, error) {
	state, err := GetPythonState()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(state.ActiveBinary) != "" {
		return state.ActiveBinary, nil
	}
	if strings.TrimSpace(state.StatusMessage) != "" {
		return "", fmt.Errorf("%s", state.StatusMessage)
	}
	return "", fmt.Errorf("未检测到可用的 Python 工具环境，请先在系统设置 > Python 中创建托管工具环境")
}

func ResolvePythonBinaryForTool(toolID string) (string, error) {
	state, err := GetPythonState()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(state.ActiveBaseBinary) == "" {
		return "", fmt.Errorf("未检测到可用的基础 Python，请先在系统设置 > Python 中选择 Python 3")
	}
	if strings.TrimSpace(state.ActiveBinary) == "" || state.NeedsRebuild {
		return "", fmt.Errorf("当前 Python 工具环境尚未准备好，请先在系统设置 > Python 中创建或重建工具环境")
	}
	if !state.PipAvailable {
		return "", fmt.Errorf("当前 Python 工具环境缺少 pip，请先在系统设置 > Python 中重建工具环境")
	}
	missing, err := missingPackagesForTool(toolID, state.Dependencies)
	if err != nil {
		return "", err
	}
	if len(missing) > 0 {
		return "", fmt.Errorf("当前 Python 工具依赖未安装，请先在系统设置 > Python 中一键安装：%s", strings.Join(missing, "、"))
	}
	return state.ActiveBinary, nil
}

func normalizePythonConfig(cfg PythonConfig) PythonConfig {
	cfg.SelectedBinary = strings.TrimSpace(cfg.SelectedBinary)
	cfg.KnownBinaries = dedupeStrings(cfg.KnownBinaries)
	if cfg.SelectedBinary != "" {
		cfg.KnownBinaries = dedupeStrings(append([]string{cfg.SelectedBinary}, cfg.KnownBinaries...))
	}
	return cfg
}

func collectPythonCandidatePaths(cfg PythonConfig) []pythonCandidatePath {
	seen := map[string]struct{}{}
	result := make([]pythonCandidatePath, 0, 16)
	add := func(path string, source string) {
		path = filepath.Clean(strings.TrimSpace(path))
		if path == "" {
			return
		}
		identity := fileIdentity(path)
		if _, ok := seen[identity]; ok {
			return
		}
		seen[identity] = struct{}{}
		result = append(result, pythonCandidatePath{path: path, source: source})
	}

	if cfg.SelectedBinary != "" {
		add(cfg.SelectedBinary, sourceConfigured)
	}
	for _, path := range cfg.KnownBinaries {
		add(path, sourceRemembered)
	}
	for _, name := range pythonExecutableNames() {
		if path, err := exec.LookPath(name); err == nil && path != "" {
			add(path, sourcePath)
		}
	}
	for _, path := range candidatePythonBinaryPaths() {
		add(path, sourceDetected)
	}
	return result
}

func inspectPythonCandidate(path string, source string, selectedBinary string) PythonCandidate {
	candidate := PythonCandidate{
		Path:     path,
		Source:   source,
		Selected: sameFilePath(path, selectedBinary),
		Detail:   path,
	}
	version, err := readPythonVersionInfo(path)
	if err != nil {
		candidate.Label = filepath.Base(path)
		candidate.Error = err.Error()
		return candidate
	}
	candidate.Version = version.Label
	candidate.Label = version.Label
	if version.Major < 3 {
		candidate.Error = "仅支持 Python 3 作为基础解释器"
		return candidate
	}
	if !hasPythonVenvSupport(path) {
		candidate.Error = "当前 Python 缺少 venv 模块，无法创建托管工具环境"
		return candidate
	}
	candidate.Valid = true
	return candidate
}

func readPythonVersion(binaryPath string) (string, error) {
	info, err := readPythonVersionInfo(binaryPath)
	if err != nil {
		return "", err
	}
	return info.Label, nil
}

func readPythonVersionInfo(binaryPath string) (pythonVersionInfo, error) {
	if _, err := os.Stat(binaryPath); err != nil {
		return pythonVersionInfo{}, fmt.Errorf("路径不存在")
	}
	output, err := exec.Command(binaryPath, "--version").CombinedOutput()
	if err != nil {
		output, err = exec.Command(binaryPath, "-V").CombinedOutput()
		if err != nil {
			return pythonVersionInfo{}, fmt.Errorf("无法读取版本")
		}
	}
	line := strings.TrimSpace(string(output))
	if line == "" {
		return pythonVersionInfo{}, fmt.Errorf("版本输出为空")
	}
	if !strings.HasPrefix(line, "Python ") {
		line = "Python " + line
	}
	info := pythonVersionInfo{Label: line}
	parts := strings.Fields(line)
	if len(parts) >= 2 {
		versionParts := strings.Split(parts[1], ".")
		if len(versionParts) > 0 {
			info.Major, _ = strconv.Atoi(versionParts[0])
		}
		if len(versionParts) > 1 {
			info.Minor, _ = strconv.Atoi(versionParts[1])
		}
		if len(versionParts) > 2 {
			info.Patch, _ = strconv.Atoi(versionParts[2])
		}
	}
	return info, nil
}

func hasPythonVenvSupport(binaryPath string) bool {
	return exec.Command(binaryPath, "-c", "import venv").Run() == nil
}

func pythonExecutableNames() []string {
	names := []string{}
	if runtime.GOOS == "windows" {
		names = append(names, "python3.exe")
		for minor := 14; minor >= 7; minor-- {
			names = append(names, fmt.Sprintf("python3.%d.exe", minor))
		}
		names = append(names, "python.exe", "py.exe")
		return dedupePackageNames(names)
	}
	names = append(names, "python3")
	for minor := 14; minor >= 7; minor-- {
		names = append(names, fmt.Sprintf("python3.%d", minor))
	}
	names = append(names, "python")
	return dedupePackageNames(names)
}

func candidatePythonBinaryPaths() []string {
	paths := make([]string, 0, 16)
	if runtime.GOOS == "windows" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			matches, _ := filepath.Glob(filepath.Join(home, "AppData", "Local", "Programs", "Python", "Python*", "python.exe"))
			sort.Strings(matches)
			for i := len(matches) - 1; i >= 0; i-- {
				paths = append(paths, matches[i])
			}
		}
		for _, envKey := range []string{"LocalAppData", "ProgramFiles"} {
			base := strings.TrimSpace(os.Getenv(envKey))
			if base == "" {
				continue
			}
			matches, _ := filepath.Glob(filepath.Join(base, "Programs", "Python", "Python*", "python.exe"))
			sort.Strings(matches)
			for i := len(matches) - 1; i >= 0; i-- {
				paths = append(paths, matches[i])
			}
		}
		return dedupeStrings(paths)
	}

	paths = append(paths,
		"/opt/homebrew/bin/python3",
		"/usr/local/bin/python3",
		"/usr/bin/python3",
		"/Library/Frameworks/Python.framework/Versions/Current/bin/python3",
	)
	if matches, _ := filepath.Glob("/Library/Frameworks/Python.framework/Versions/*/bin/python3"); len(matches) > 0 {
		sort.Strings(matches)
		for i := len(matches) - 1; i >= 0; i-- {
			paths = append(paths, matches[i])
		}
	}
	return dedupeStrings(paths)
}

func inspectPythonDependencies(binaryPath string, manifest pythonRequirementManifest) ([]PythonDependency, bool) {
	pipAvailable := hasUsablePip(binaryPath)
	requirements := aggregatePythonRequirements(manifest)
	if len(requirements) == 0 {
		return []PythonDependency{}, pipAvailable
	}
	if !pipAvailable {
		dependencies := make([]PythonDependency, 0, len(requirements))
		for _, spec := range requirements {
			dependencies = append(dependencies, PythonDependency{
				PackageName: spec.Name,
				ModuleName:  spec.Module,
				Installed:   false,
				Error:       "未检测到 pip",
				RequiredBy:  append([]string{}, spec.RequiredBy...),
			})
		}
		return dependencies, false
	}

	probes, err := probePythonDependencies(binaryPath, requirements)
	if err != nil {
		dependencies := make([]PythonDependency, 0, len(requirements))
		for _, spec := range requirements {
			dependencies = append(dependencies, PythonDependency{
				PackageName: spec.Name,
				ModuleName:  spec.Module,
				Installed:   false,
				Error:       err.Error(),
				RequiredBy:  append([]string{}, spec.RequiredBy...),
			})
		}
		return dependencies, true
	}

	result := make([]PythonDependency, 0, len(requirements))
	for _, spec := range requirements {
		probe, ok := probes[spec.Name]
		dep := PythonDependency{
			PackageName: spec.Name,
			ModuleName:  spec.Module,
			RequiredBy:  append([]string{}, spec.RequiredBy...),
		}
		if ok {
			dep.Installed = probe.Installed
			dep.Version = strings.TrimSpace(probe.Version)
			dep.Error = strings.TrimSpace(probe.Error)
		}
		result = append(result, dep)
	}
	return result, true
}

func hasUsablePip(binaryPath string) bool {
	return exec.Command(binaryPath, "-m", "pip", "--version").Run() == nil
}

func probePythonDependencies(binaryPath string, requirements []pythonPackageRequirement) (map[string]pythonDependencyProbe, error) {
	specs := make([]map[string]string, 0, len(requirements))
	for _, spec := range requirements {
		specs = append(specs, map[string]string{
			"name":   spec.Name,
			"module": spec.Module,
		})
	}
	payload, err := json.Marshal(specs)
	if err != nil {
		return nil, fmt.Errorf("序列化 Python 依赖探测请求失败: %w", err)
	}
	script := strings.Join([]string{
		"import importlib.util",
		"import json",
		"import sys",
		"try:",
		"    from importlib import metadata as importlib_metadata",
		"except Exception:",
		"    importlib_metadata = None",
		"specs = json.loads(sys.stdin.read())",
		"result = []",
		"for spec in specs:",
		"    installed = importlib.util.find_spec(spec['module']) is not None",
		"    version = ''",
		"    error = ''",
		"    if installed and importlib_metadata is not None:",
		"        try:",
		"            version = importlib_metadata.version(spec['name'])",
		"        except Exception as exc:",
		"            error = str(exc)",
		"    result.append({",
		"        'name': spec['name'],",
		"        'module': spec['module'],",
		"        'installed': installed,",
		"        'version': version,",
		"        'error': error,",
		"    })",
		"print(json.dumps(result))",
	}, "\n")
	cmd := exec.Command(binaryPath, "-c", script)
	cmd.Stdin = bytes.NewReader(payload)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("检查 Python 依赖失败: %s", strings.TrimSpace(string(output)))
	}
	var probes []pythonDependencyProbe
	if err := json.Unmarshal(output, &probes); err != nil {
		return nil, fmt.Errorf("解析 Python 依赖状态失败: %w", err)
	}
	result := make(map[string]pythonDependencyProbe, len(probes))
	for _, probe := range probes {
		result[probe.Name] = probe
	}
	return result, nil
}

type pythonPackageRequirement struct {
	Name       string
	Module     string
	RequiredBy []string
}

func aggregatePythonRequirements(manifest pythonRequirementManifest) []pythonPackageRequirement {
	byName := map[string]pythonPackageRequirement{}
	for toolID, tool := range manifest.Tools {
		for _, pkg := range tool.Packages {
			name := strings.TrimSpace(pkg.Name)
			module := strings.TrimSpace(pkg.Module)
			if name == "" || module == "" {
				continue
			}
			current := byName[name]
			current.Name = name
			current.Module = module
			current.RequiredBy = append(current.RequiredBy, toolID)
			byName[name] = current
		}
	}
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]pythonPackageRequirement, 0, len(names))
	for _, name := range names {
		entry := byName[name]
		sort.Strings(entry.RequiredBy)
		result = append(result, entry)
	}
	return result
}

func uniquePythonPackageNames(manifest pythonRequirementManifest) []string {
	requirements := aggregatePythonRequirements(manifest)
	result := make([]string, 0, len(requirements))
	for _, req := range requirements {
		result = append(result, req.Name)
	}
	return result
}

func collectMissingPythonPackages(dependencies []PythonDependency) []string {
	result := make([]string, 0)
	for _, dep := range dependencies {
		if dep.Installed {
			continue
		}
		result = append(result, dep.PackageName)
	}
	return result
}

func missingPackagesForTool(toolID string, dependencies []PythonDependency) ([]string, error) {
	manifest, err := loadPythonRequirementManifest()
	if err != nil {
		return nil, err
	}
	tool, ok := manifest.Tools[toolID]
	if !ok {
		return nil, nil
	}
	installed := make(map[string]bool, len(dependencies))
	for _, dep := range dependencies {
		installed[dep.PackageName] = dep.Installed
	}
	missing := make([]string, 0)
	for _, pkg := range tool.Packages {
		if installed[pkg.Name] {
			continue
		}
		missing = append(missing, pkg.Name)
	}
	return dedupePackageNames(missing), nil
}

func dedupePackageNames(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func loadPythonRequirementManifest() (pythonRequirementManifest, error) {
	toolManifests, err := builtin.Load()
	if err != nil {
		return pythonRequirementManifest{}, fmt.Errorf("读取内置工具清单失败: %w", err)
	}
	return buildPythonRequirementManifest(toolManifests), nil
}

func buildPythonRequirementManifest(toolManifests []toolspec.ToolManifest) pythonRequirementManifest {
	return buildPythonRequirementManifestWithLoader(toolManifests, readPythonToolSource)
}

func buildPythonRequirementManifestWithLoader(toolManifests []toolspec.ToolManifest, loadSource func(string) ([]byte, error)) pythonRequirementManifest {
	manifest := pythonRequirementManifest{
		Tools: map[string]pythonToolRequirement{},
	}
	for _, manifestEntry := range toolManifests {
		if manifestEntry.Kind != toolspec.ToolKindPython {
			continue
		}
		sourceEntry := strings.TrimSpace(manifestEntry.Source.Entry)
		if sourceEntry == "" {
			continue
		}
		packages := scanPythonRequirementPackages(sourceEntry, loadSource)
		if len(packages) == 0 {
			continue
		}
		manifest.Tools[manifestEntry.ID] = pythonToolRequirement{
			Packages: packages,
		}
	}
	return manifest
}

func scanPythonRequirementPackages(sourceEntry string, loadSource func(string) ([]byte, error)) []pythonPackageSpec {
	source, err := loadSource(sourceEntry)
	if err != nil {
		return []pythonPackageSpec{}
	}
	moduleNames := scanPythonImportedModules(string(source))
	packages := make([]pythonPackageSpec, 0, len(moduleNames))
	for _, moduleName := range moduleNames {
		packageName := pythonPackageNameForModule(moduleName)
		packages = append(packages, pythonPackageSpec{
			Name:   packageName,
			Module: moduleName,
		})
	}
	return packages
}

func readPythonToolSource(sourceEntry string) ([]byte, error) {
	if repoRoot, ok := runtimeenv.FindRepoRoot(); ok {
		sourcePath := filepath.Join(repoRoot, filepath.FromSlash(sourceEntry))
		if source, err := os.ReadFile(sourcePath); err == nil {
			return source, nil
		}
	}
	return python_tools.ReadEmbeddedScript(sourceEntry)
}

func scanPythonImportedModules(source string) []string {
	result := make([]string, 0, 8)
	lines := strings.Split(source, "\n")
	for _, line := range lines {
		if match := pythonImportPattern.FindStringSubmatch(line); len(match) == 2 {
			for _, item := range strings.Split(match[1], ",") {
				moduleName := normalizePythonImportModule(item)
				if shouldIncludePythonDependencyModule(moduleName) {
					result = append(result, moduleName)
				}
			}
			continue
		}
		if match := pythonFromImportPattern.FindStringSubmatch(line); len(match) == 2 {
			moduleName := normalizePythonImportModule(match[1])
			if shouldIncludePythonDependencyModule(moduleName) {
				result = append(result, moduleName)
			}
		}
	}
	return dedupePackageNames(result)
}

func normalizePythonImportModule(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, ".") {
		return ""
	}
	if beforeAlias, _, found := strings.Cut(raw, " as "); found {
		raw = strings.TrimSpace(beforeAlias)
	}
	if idx := strings.Index(raw, "."); idx >= 0 {
		raw = raw[:idx]
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	return raw
}

func shouldIncludePythonDependencyModule(moduleName string) bool {
	if moduleName == "" {
		return false
	}
	if isPythonStandardLibraryModule(moduleName) {
		return false
	}
	return true
}

func pythonPackageNameForModule(moduleName string) string {
	data := getPythonDependencyScanData()
	if mapped, ok := data.modulePackageAliases[moduleName]; ok {
		return mapped
	}
	return moduleName
}

func isPythonStandardLibraryModule(moduleName string) bool {
	data := getPythonDependencyScanData()
	_, ok := data.standardLibrary[moduleName]
	return ok
}

func getPythonDependencyScanData() pythonDependencyScanData {
	pythonDependencyDataOnce.Do(func() {
		pythonDependencyData = pythonDependencyScanData{
			modulePackageAliases: parsePythonImportMapping(embeddedPythonImportMapping),
			standardLibrary:      parsePythonStdlibModules(embeddedPythonStdlibModules),
		}
	})
	return pythonDependencyData
}

func parsePythonImportMapping(content string) map[string]string {
	result := map[string]string{}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		moduleName, packageName, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		moduleName = strings.TrimSpace(moduleName)
		packageName = strings.TrimSpace(packageName)
		if moduleName == "" || packageName == "" {
			continue
		}
		result[moduleName] = packageName
	}
	return result
}

func parsePythonStdlibModules(content string) map[string]struct{} {
	result := map[string]struct{}{}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		result[line] = struct{}{}
	}
	return result
}

func pythonManagedEnvDirectory(layout runtimeenv.Layout) string {
	return filepath.Join(layout.Root, "toolchains", pythonToolchainDirName)
}

func managedPythonBinaryPath(envDir string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(envDir, "Scripts", "python.exe")
	}
	return filepath.Join(envDir, "bin", "python")
}

func managedPythonMetadataPath(envDir string) string {
	return filepath.Join(envDir, pythonToolchainMetaName)
}

func loadManagedPythonMetadata(envDir string) (*managedPythonMetadata, error) {
	data, err := os.ReadFile(managedPythonMetadataPath(envDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("读取 Python 工具环境元数据失败: %w", err)
	}
	var metadata managedPythonMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("解析 Python 工具环境元数据失败: %w", err)
	}
	return &metadata, nil
}

func writeManagedPythonMetadata(envDir string, metadata managedPythonMetadata) error {
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 Python 工具环境元数据失败: %w", err)
	}
	if err := os.WriteFile(managedPythonMetadataPath(envDir), data, 0644); err != nil {
		return fmt.Errorf("写入 Python 工具环境元数据失败: %w", err)
	}
	return nil
}

func isExistingFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func commandErrorDetail(err error, output []byte) string {
	detail := strings.TrimSpace(string(output))
	if detail != "" {
		return detail
	}
	return err.Error()
}
