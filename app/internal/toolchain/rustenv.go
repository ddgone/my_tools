package toolchain

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"fire-salamander-desktop/internal/appconfig"
	"my_tools/libs/core/procutil"
)

const configKeyRust = "rust"

type RustConfig struct {
	SelectedCargoBinary         string   `json:"selectedCargoBinary"`
	KnownCargoBinaries          []string `json:"knownCargoBinaries"`
	SelectedRustupBinary        string   `json:"selectedRustupBinary"`
	KnownRustupBinaries         []string `json:"knownRustupBinaries"`
	SelectedZigBinary           string   `json:"selectedZigBinary"`
	KnownZigBinaries            []string `json:"knownZigBinaries"`
	SelectedCargoZigbuildBinary string   `json:"selectedCargoZigbuildBinary"`
	KnownCargoZigbuildBinaries  []string `json:"knownCargoZigbuildBinaries"`
}

type RustCandidate struct {
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

type RustTargetStatus struct {
	PlatformKey   string `json:"platformKey"`
	PlatformLabel string `json:"platformLabel"`
	TargetTriple  string `json:"targetTriple"`
	Installed     bool   `json:"installed"`
	Native        bool   `json:"native"`
	Note          string `json:"note,omitempty"`
}

type RustToolchainState struct {
	Config                     RustConfig         `json:"config"`
	CargoCandidates            []RustCandidate    `json:"cargoCandidates"`
	RustupCandidates           []RustCandidate    `json:"rustupCandidates"`
	ZigCandidates              []RustCandidate    `json:"zigCandidates"`
	CargoZigbuildCandidates    []RustCandidate    `json:"cargoZigbuildCandidates"`
	InstalledTargets           []string           `json:"installedTargets"`
	TargetStatuses             []RustTargetStatus `json:"targetStatuses"`
	HasInstalledTargetInfo     bool               `json:"hasInstalledTargetInfo"`
	HasFullTargetCoverage      bool               `json:"hasFullTargetCoverage"`
	TargetStatusMessage        string             `json:"targetStatusMessage"`
	HasUsableEnvironment       bool               `json:"hasUsableEnvironment"`
	HasUsableCargo             bool               `json:"hasUsableCargo"`
	HasUsableRustup            bool               `json:"hasUsableRustup"`
	HasUsableZig               bool               `json:"hasUsableZig"`
	HasUsableCargoZigbuild     bool               `json:"hasUsableCargoZigbuild"`
	ActiveCargoBinary          string             `json:"activeCargoBinary"`
	ActiveCargoVersion         string             `json:"activeCargoVersion"`
	ActiveCargoSource          string             `json:"activeCargoSource"`
	ActiveRustupBinary         string             `json:"activeRustupBinary"`
	ActiveRustupVersion        string             `json:"activeRustupVersion"`
	ActiveRustupSource         string             `json:"activeRustupSource"`
	ActiveZigBinary            string             `json:"activeZigBinary"`
	ActiveZigVersion           string             `json:"activeZigVersion"`
	ActiveZigSource            string             `json:"activeZigSource"`
	ActiveCargoZigbuildBinary  string             `json:"activeCargoZigbuildBinary"`
	ActiveCargoZigbuildVersion string             `json:"activeCargoZigbuildVersion"`
	ActiveCargoZigbuildSource  string             `json:"activeCargoZigbuildSource"`
	StatusMessage              string             `json:"statusMessage"`
}

type rustCandidatePath struct {
	path   string
	source string
}

type rustToolVersionReader func(string) (string, error)

func LoadRustConfig() (RustConfig, error) {
	configPath, err := appconfig.ResolveConfigPath()
	if err != nil {
		return RustConfig{}, err
	}
	doc, err := appconfig.LoadDocument(configPath)
	if err != nil {
		return RustConfig{}, err
	}
	var cfg RustConfig
	if raw, ok := doc[configKeyRust]; ok && len(raw) > 0 {
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return RustConfig{}, fmt.Errorf("解析 Rust 配置失败: %w", err)
		}
	}
	return normalizeRustConfig(cfg), nil
}

func SaveRustConfig(cfg RustConfig) error {
	configPath, err := appconfig.ResolveConfigPath()
	if err != nil {
		return err
	}
	doc, err := appconfig.LoadDocument(configPath)
	if err != nil {
		return err
	}
	cfg = normalizeRustConfig(cfg)
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("序列化 Rust 配置失败: %w", err)
	}
	doc[configKeyRust] = data
	return appconfig.WriteDocument(configPath, doc)
}

func GetRustState() (RustToolchainState, error) {
	cfg, err := LoadRustConfig()
	if err != nil {
		return RustToolchainState{}, err
	}
	return InspectRustConfig(cfg)
}

func InspectRustConfig(cfg RustConfig) (RustToolchainState, error) {
	cfg = normalizeRustConfig(cfg)

	cargoCandidates, activeCargoBinary, activeCargoVersion, activeCargoSource, cargoSelectedError := inspectRustToolCandidates(
		collectRustToolCandidatePaths(cfg.SelectedCargoBinary, cfg.KnownCargoBinaries, []string{rustToolExecutableName("cargo")}, cargoToolFallbackCandidates("cargo")),
		cfg.SelectedCargoBinary,
		readCargoVersion,
	)
	rustupCandidates, activeRustupBinary, activeRustupVersion, activeRustupSource, rustupSelectedError := inspectRustToolCandidates(
		collectRustToolCandidatePaths(cfg.SelectedRustupBinary, cfg.KnownRustupBinaries, []string{rustToolExecutableName("rustup")}, cargoToolFallbackCandidates("rustup")),
		cfg.SelectedRustupBinary,
		readRustupVersion,
	)
	zigCandidates, activeZigBinary, activeZigVersion, activeZigSource, zigSelectedError := inspectRustToolCandidates(
		collectRustToolCandidatePaths(cfg.SelectedZigBinary, cfg.KnownZigBinaries, []string{rustToolExecutableName("zig")}, zigToolFallbackCandidates()),
		cfg.SelectedZigBinary,
		readZigVersion,
	)
	cargoZigbuildCandidates, activeCargoZigbuildBinary, activeCargoZigbuildVersion, activeCargoZigbuildSource, cargoZigbuildSelectedError := inspectRustToolCandidates(
		collectRustToolCandidatePaths(cfg.SelectedCargoZigbuildBinary, cfg.KnownCargoZigbuildBinaries, []string{rustToolExecutableName("cargo-zigbuild")}, cargoToolFallbackCandidates("cargo-zigbuild")),
		cfg.SelectedCargoZigbuildBinary,
		readCargoZigbuildVersion,
	)
	targetStatuses := make([]RustTargetStatus, 0, len(rustSupportedTargets()))
	installedTargets := make([]string, 0, len(rustSupportedTargets()))
	hasInstalledTargetInfo := false
	hasFullTargetCoverage := false
	targetStatusMessage := ""
	if activeRustupBinary != "" {
		statuses, installed, err := inspectRustTargets(activeRustupBinary)
		if err != nil {
			targetStatusMessage = fmt.Sprintf("无法读取已安装 Rust targets：%v", err)
		} else {
			targetStatuses = statuses
			installedTargets = installed
			hasInstalledTargetInfo = true
			hasFullTargetCoverage, targetStatusMessage = summarizeRustTargets(statuses)
		}
	}

	statusParts := make([]string, 0, 4)
	if activeCargoBinary == "" {
		if cargoSelectedError != "" {
			statusParts = append(statusParts, cargoSelectedError)
		} else {
			statusParts = append(statusParts, "未检测到可用的 cargo")
		}
	}
	if activeRustupBinary == "" {
		if rustupSelectedError != "" {
			statusParts = append(statusParts, rustupSelectedError)
		} else {
			statusParts = append(statusParts, "未检测到可用的 rustup")
		}
	}
	if activeZigBinary == "" {
		if zigSelectedError != "" {
			statusParts = append(statusParts, zigSelectedError)
		} else {
			statusParts = append(statusParts, "未检测到可用的 zig")
		}
	}
	if activeCargoZigbuildBinary == "" {
		if cargoZigbuildSelectedError != "" {
			statusParts = append(statusParts, cargoZigbuildSelectedError)
		} else {
			statusParts = append(statusParts, "未检测到可用的 cargo-zigbuild")
		}
	}

	statusMessage := "Rust 交叉编译环境已就绪"
	if len(statusParts) > 0 {
		statusMessage = strings.Join(statusParts, "；")
	} else if targetStatusMessage != "" {
		statusMessage = "Rust 交叉编译环境已就绪；" + targetStatusMessage
	}

	return RustToolchainState{
		Config:                     cfg,
		CargoCandidates:            cargoCandidates,
		RustupCandidates:           rustupCandidates,
		ZigCandidates:              zigCandidates,
		CargoZigbuildCandidates:    cargoZigbuildCandidates,
		InstalledTargets:           installedTargets,
		TargetStatuses:             targetStatuses,
		HasInstalledTargetInfo:     hasInstalledTargetInfo,
		HasFullTargetCoverage:      hasFullTargetCoverage,
		TargetStatusMessage:        targetStatusMessage,
		HasUsableEnvironment:       activeCargoBinary != "" && activeRustupBinary != "" && activeZigBinary != "" && activeCargoZigbuildBinary != "",
		HasUsableCargo:             activeCargoBinary != "",
		HasUsableRustup:            activeRustupBinary != "",
		HasUsableZig:               activeZigBinary != "",
		HasUsableCargoZigbuild:     activeCargoZigbuildBinary != "",
		ActiveCargoBinary:          activeCargoBinary,
		ActiveCargoVersion:         activeCargoVersion,
		ActiveCargoSource:          activeCargoSource,
		ActiveRustupBinary:         activeRustupBinary,
		ActiveRustupVersion:        activeRustupVersion,
		ActiveRustupSource:         activeRustupSource,
		ActiveZigBinary:            activeZigBinary,
		ActiveZigVersion:           activeZigVersion,
		ActiveZigSource:            activeZigSource,
		ActiveCargoZigbuildBinary:  activeCargoZigbuildBinary,
		ActiveCargoZigbuildVersion: activeCargoZigbuildVersion,
		ActiveCargoZigbuildSource:  activeCargoZigbuildSource,
		StatusMessage:              statusMessage,
	}, nil
}

type rustSupportedTarget struct {
	platformKey   string
	platformLabel string
	targetTriple  string
	native        bool
}

func rustSupportedTargets() []rustSupportedTarget {
	platforms := []struct {
		targetOS     string
		targetArch   string
		label        string
		targetTriple string
	}{
		{targetOS: "linux", targetArch: "amd64", label: "Linux x64", targetTriple: "x86_64-unknown-linux-musl"},
		{targetOS: "linux", targetArch: "arm64", label: "Linux ARM64", targetTriple: "aarch64-unknown-linux-musl"},
		{targetOS: "darwin", targetArch: "amd64", label: "mac Intel", targetTriple: "x86_64-apple-darwin"},
		{targetOS: "darwin", targetArch: "arm64", label: "mac Apple", targetTriple: "aarch64-apple-darwin"},
		{targetOS: "windows", targetArch: "amd64", label: "Windows x64", targetTriple: "x86_64-pc-windows-gnu"},
		{targetOS: "windows", targetArch: "arm64", label: "Windows ARM64", targetTriple: "aarch64-pc-windows-gnullvm"},
	}
	result := make([]rustSupportedTarget, 0, len(platforms))
	for _, platform := range platforms {
		result = append(result, rustSupportedTarget{
			platformKey:   platform.targetOS + "/" + platform.targetArch,
			platformLabel: platform.label,
			targetTriple:  platform.targetTriple,
			native:        platform.targetOS == runtime.GOOS && platform.targetArch == runtime.GOARCH,
		})
	}
	return result
}

func inspectRustTargets(rustupBinary string) ([]RustTargetStatus, []string, error) {
	installedTargets, err := readInstalledRustTargets(rustupBinary)
	if err != nil {
		return nil, nil, err
	}
	installedSet := make(map[string]struct{}, len(installedTargets))
	for _, target := range installedTargets {
		installedSet[target] = struct{}{}
	}
	statuses := make([]RustTargetStatus, 0, len(rustSupportedTargets()))
	for _, supported := range rustSupportedTargets() {
		status := RustTargetStatus{
			PlatformKey:   supported.platformKey,
			PlatformLabel: supported.platformLabel,
			TargetTriple:  supported.targetTriple,
			Native:        supported.native,
			Installed:     supported.native,
		}
		if supported.native {
			status.Note = "当前宿主平台走原生构建，无需额外安装 target"
		} else if _, ok := installedSet[supported.targetTriple]; ok {
			status.Installed = true
		}
		statuses = append(statuses, status)
	}
	return statuses, installedTargets, nil
}

func summarizeRustTargets(statuses []RustTargetStatus) (bool, string) {
	totalCrossTargets := 0
	installedCrossTargets := 0
	missingTargets := make([]string, 0, len(statuses))
	for _, status := range statuses {
		if status.Native {
			continue
		}
		totalCrossTargets += 1
		if status.Installed {
			installedCrossTargets += 1
			continue
		}
		missingTargets = append(missingTargets, status.TargetTriple)
	}
	if totalCrossTargets == 0 {
		return true, "当前宿主仅需原生构建，无需额外安装 rustup target"
	}
	if len(missingTargets) == 0 {
		return true, fmt.Sprintf("常用交叉编译 targets 已安装 %d/%d", installedCrossTargets, totalCrossTargets)
	}
	return false, fmt.Sprintf(
		"常用交叉编译 targets 已安装 %d/%d，缺少 %s；首次构建时会尝试自动执行 rustup target add",
		installedCrossTargets,
		totalCrossTargets,
		strings.Join(missingTargets, "、"),
	)
}

func readInstalledRustTargets(rustupBinary string) ([]string, error) {
	if _, err := os.Stat(rustupBinary); err != nil {
		return nil, fmt.Errorf("rustup 路径不存在")
	}
	output, err := procutil.Command(rustupBinary, "target", "list", "--installed").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("执行 rustup target list --installed 失败")
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	targets := make([]string, 0, len(lines))
	seen := make(map[string]struct{}, len(lines))
	for _, line := range lines {
		target := strings.TrimSpace(line)
		if target == "" {
			continue
		}
		if _, ok := seen[target]; ok {
			continue
		}
		seen[target] = struct{}{}
		targets = append(targets, target)
	}
	return targets, nil
}

func ResolveCargoBinary() (string, error) {
	state, err := GetRustState()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(state.ActiveCargoBinary) != "" {
		return state.ActiveCargoBinary, nil
	}
	if strings.TrimSpace(state.StatusMessage) != "" {
		return "", fmt.Errorf("%s", state.StatusMessage)
	}
	return "", fmt.Errorf("未检测到可用的 Rust 交叉编译环境，请先在系统设置 > Rust 中配置 cargo")
}

func ResolveRustupBinary() (string, error) {
	state, err := GetRustState()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(state.ActiveRustupBinary) != "" {
		return state.ActiveRustupBinary, nil
	}
	if strings.TrimSpace(state.StatusMessage) != "" {
		return "", fmt.Errorf("%s", state.StatusMessage)
	}
	return "", fmt.Errorf("未检测到可用的 Rust 交叉编译环境，请先在系统设置 > Rust 中配置 rustup")
}

func ResolveZigBinary() (string, error) {
	state, err := GetRustState()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(state.ActiveZigBinary) != "" {
		return state.ActiveZigBinary, nil
	}
	if strings.TrimSpace(state.StatusMessage) != "" {
		return "", fmt.Errorf("%s", state.StatusMessage)
	}
	return "", fmt.Errorf("未检测到可用的 Rust 交叉编译环境，请先在系统设置 > Rust 中配置 zig")
}

func ResolveCargoZigbuildBinary() (string, error) {
	state, err := GetRustState()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(state.ActiveCargoZigbuildBinary) != "" {
		return state.ActiveCargoZigbuildBinary, nil
	}
	if strings.TrimSpace(state.StatusMessage) != "" {
		return "", fmt.Errorf("%s", state.StatusMessage)
	}
	return "", fmt.Errorf("未检测到可用的 Rust 交叉编译环境，请先在系统设置 > Rust 中配置 cargo-zigbuild")
}

func normalizeRustConfig(cfg RustConfig) RustConfig {
	cfg.SelectedCargoBinary = strings.TrimSpace(cfg.SelectedCargoBinary)
	cfg.KnownCargoBinaries = dedupeStrings(cfg.KnownCargoBinaries)
	cfg.SelectedRustupBinary = strings.TrimSpace(cfg.SelectedRustupBinary)
	cfg.KnownRustupBinaries = dedupeStrings(cfg.KnownRustupBinaries)
	cfg.SelectedZigBinary = strings.TrimSpace(cfg.SelectedZigBinary)
	cfg.KnownZigBinaries = dedupeStrings(cfg.KnownZigBinaries)
	cfg.SelectedCargoZigbuildBinary = strings.TrimSpace(cfg.SelectedCargoZigbuildBinary)
	cfg.KnownCargoZigbuildBinaries = dedupeStrings(cfg.KnownCargoZigbuildBinaries)
	return cfg
}

func collectRustToolCandidatePaths(selected string, known []string, executableNames []string, fallbackPaths []string) []rustCandidatePath {
	seen := map[string]struct{}{}
	result := make([]rustCandidatePath, 0, 16)
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
		result = append(result, rustCandidatePath{path: path, source: source})
	}

	if selected != "" {
		add(selected, sourceConfigured)
	}
	for _, path := range known {
		add(path, sourceRemembered)
	}
	for _, name := range executableNames {
		if path, err := exec.LookPath(name); err == nil && path != "" {
			add(path, sourcePath)
		}
	}
	for _, path := range fallbackPaths {
		add(path, sourceDetected)
	}
	return result
}

func inspectRustToolCandidates(paths []rustCandidatePath, selectedBinary string, readVersion rustToolVersionReader) ([]RustCandidate, string, string, string, string) {
	candidates := make([]RustCandidate, 0, len(paths))
	activeBinary := ""
	activeVersion := ""
	activeSource := ""
	selectedError := ""
	for _, entry := range paths {
		candidate := inspectRustCandidate(entry.path, entry.source, selectedBinary, readVersion)
		if candidate.Selected && !candidate.Valid && candidate.Error != "" {
			selectedError = candidate.Error
		}
		if activeBinary == "" && candidate.Valid {
			activeBinary = candidate.Path
			activeVersion = candidate.Version
			activeSource = candidate.Source
			candidate.Active = true
		}
		candidates = append(candidates, candidate)
	}
	return candidates, activeBinary, activeVersion, activeSource, selectedError
}

func inspectRustCandidate(path string, source string, selectedBinary string, readVersion rustToolVersionReader) RustCandidate {
	candidate := RustCandidate{
		Path:     path,
		Source:   source,
		Selected: sameFilePath(path, selectedBinary),
		Detail:   path,
		Label:    filepath.Base(path),
	}
	version, err := readVersion(path)
	if err != nil {
		candidate.Error = err.Error()
		return candidate
	}
	candidate.Version = version
	candidate.Label = version
	candidate.Valid = true
	return candidate
}

func readCargoVersion(binaryPath string) (string, error) {
	return readRustToolVersion(binaryPath, "--version", "cargo")
}

func readRustupVersion(binaryPath string) (string, error) {
	return readRustToolVersion(binaryPath, "--version", "rustup")
}

func readZigVersion(binaryPath string) (string, error) {
	return readRustToolVersion(binaryPath, "version", "zig")
}

func readCargoZigbuildVersion(binaryPath string) (string, error) {
	return readRustToolVersion(binaryPath, "--version", "cargo-zigbuild")
}

func readRustToolVersion(binaryPath string, versionArg string, prefix string) (string, error) {
	if _, err := os.Stat(binaryPath); err != nil {
		return "", fmt.Errorf("路径不存在")
	}
	output, err := procutil.Command(binaryPath, versionArg).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("无法读取版本")
	}
	line := strings.TrimSpace(strings.SplitN(strings.TrimSpace(string(output)), "\n", 2)[0])
	if line == "" {
		return "", fmt.Errorf("版本输出为空")
	}
	if prefix != "" && !strings.HasPrefix(strings.ToLower(line), strings.ToLower(prefix)) {
		line = prefix + " " + line
	}
	return line, nil
}

func rustToolExecutableName(name string) string {
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(name), ".exe") {
		return name + ".exe"
	}
	return name
}

func cargoToolFallbackCandidates(name string) []string {
	home, _ := os.UserHomeDir()
	if home == "" {
		return nil
	}
	return []string{
		filepath.Join(home, ".cargo", "bin", rustToolExecutableName(name)),
	}
}

func zigToolFallbackCandidates() []string {
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join("/opt/homebrew", "bin", rustToolExecutableName("zig")),
		filepath.Join("/usr/local", "bin", rustToolExecutableName("zig")),
	}
	if home != "" {
		candidates = append(candidates,
			filepath.Join(home, ".local", "bin", rustToolExecutableName("zig")),
			filepath.Join(home, "bin", rustToolExecutableName("zig")),
		)
	}
	return candidates
}
