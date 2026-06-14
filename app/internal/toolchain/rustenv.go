package toolchain

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"fire-salamander-desktop/internal/appconfig"
	"my_tools/libs/core/procutil"
)

const (
	configKeyRust  = "rust"
	rustModeAuto   = "auto"
	rustModeNone   = "none"
	rustModeManual = "manual"
)

type RustConfig struct {
	Mode                 string   `json:"mode"`
	SelectedRustRoot     string   `json:"selectedRustRoot"`
	KnownRustRoots       []string `json:"knownRustRoots"`
	SelectedZigBinary    string   `json:"selectedZigBinary"`
	KnownZigBinaries     []string `json:"knownZigBinaries"`
	LastInstallDirectory string   `json:"lastInstallDirectory"`
	Disabled             bool     `json:"disabled"`

	SelectedCargoBinary         string   `json:"selectedCargoBinary,omitempty"`
	KnownCargoBinaries          []string `json:"knownCargoBinaries,omitempty"`
	SelectedRustupBinary        string   `json:"selectedRustupBinary,omitempty"`
	KnownRustupBinaries         []string `json:"knownRustupBinaries,omitempty"`
	SelectedCargoZigbuildBinary string   `json:"selectedCargoZigbuildBinary,omitempty"`
	KnownCargoZigbuildBinaries  []string `json:"knownCargoZigbuildBinaries,omitempty"`
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

type RustEnvironmentCandidate struct {
	RootDir             string `json:"rootDir"`
	Version             string `json:"version"`
	Source              string `json:"source"`
	Label               string `json:"label"`
	Detail              string `json:"detail"`
	Error               string `json:"error,omitempty"`
	Valid               bool   `json:"valid"`
	Selected            bool   `json:"selected"`
	Active              bool   `json:"active"`
	CargoBinary         string `json:"cargoBinary,omitempty"`
	RustupBinary        string `json:"rustupBinary,omitempty"`
	RustcBinary         string `json:"rustcBinary,omitempty"`
	CargoZigbuildBinary string `json:"cargoZigbuildBinary,omitempty"`
	HasRustup           bool   `json:"hasRustup"`
	HasCargoZigbuild    bool   `json:"hasCargoZigbuild"`
	CanManageTargets    bool   `json:"canManageTargets"`
	Managed             bool   `json:"managed"`
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
	Config                     RustConfig                 `json:"config"`
	RustCandidates             []RustEnvironmentCandidate `json:"rustCandidates"`
	ZigCandidates              []RustCandidate            `json:"zigCandidates"`
	InstalledTargets           []string                   `json:"installedTargets"`
	TargetStatuses             []RustTargetStatus         `json:"targetStatuses"`
	HasInstalledTargetInfo     bool                       `json:"hasInstalledTargetInfo"`
	HasFullTargetCoverage      bool                       `json:"hasFullTargetCoverage"`
	TargetStatusMessage        string                     `json:"targetStatusMessage"`
	CargoZigbuildStatusMessage string                     `json:"cargoZigbuildStatusMessage"`
	HasUsableEnvironment       bool                       `json:"hasUsableEnvironment"`
	HasUsableRust              bool                       `json:"hasUsableRust"`
	HasUsableCargo             bool                       `json:"hasUsableCargo"`
	HasUsableRustup            bool                       `json:"hasUsableRustup"`
	HasUsableZig               bool                       `json:"hasUsableZig"`
	HasUsableCargoZigbuild     bool                       `json:"hasUsableCargoZigbuild"`
	CanManageTargets           bool                       `json:"canManageTargets"`
	CanManageCargoZigbuild     bool                       `json:"canManageCargoZigbuild"`
	ActiveRustRoot             string                     `json:"activeRustRoot"`
	ActiveRustVersion          string                     `json:"activeRustVersion"`
	ActiveRustSource           string                     `json:"activeRustSource"`
	ActiveRustManaged          bool                       `json:"activeRustManaged"`
	ActiveCargoBinary          string                     `json:"activeCargoBinary"`
	ActiveCargoVersion         string                     `json:"activeCargoVersion"`
	ActiveCargoSource          string                     `json:"activeCargoSource"`
	ActiveRustupBinary         string                     `json:"activeRustupBinary"`
	ActiveRustupVersion        string                     `json:"activeRustupVersion"`
	ActiveRustupSource         string                     `json:"activeRustupSource"`
	ActiveRustcBinary          string                     `json:"activeRustcBinary"`
	ActiveZigBinary            string                     `json:"activeZigBinary"`
	ActiveZigVersion           string                     `json:"activeZigVersion"`
	ActiveZigSource            string                     `json:"activeZigSource"`
	ActiveCargoZigbuildBinary  string                     `json:"activeCargoZigbuildBinary"`
	ActiveCargoZigbuildVersion string                     `json:"activeCargoZigbuildVersion"`
	ActiveCargoZigbuildSource  string                     `json:"activeCargoZigbuildSource"`
	StatusMessage              string                     `json:"statusMessage"`
	SuggestedInstallDirectory  string                     `json:"suggestedInstallDirectory"`
}

type rustCandidatePath struct {
	path   string
	source string
}

type rustToolVersionReader func(string) (string, error)

type rustEnvironmentLayout struct {
	RootDir             string
	CargoHome           string
	RustupHome          string
	CargoBinary         string
	RustupBinary        string
	RustcBinary         string
	CargoZigbuildBinary string
	Managed             bool
}

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
	suggestedInstallDirectory := NormalizeRustInstallBaseDirectory(cfg.LastInstallDirectory)
	if suggestedInstallDirectory == "" {
		if defaultDir, err := defaultToolchainInstallDirectory(); err == nil {
			suggestedInstallDirectory = NormalizeRustInstallBaseDirectory(defaultDir)
		}
	}

	selectedRustRoot := ""
	selectedZigBinary := ""
	if cfg.Mode == rustModeManual {
		selectedRustRoot = cfg.SelectedRustRoot
		selectedZigBinary = cfg.SelectedZigBinary
	}

	rustCandidates, activeRustCandidate, rustSelectedError := inspectRustEnvironmentCandidates(
		collectRustEnvironmentCandidateRoots(cfg, selectedRustRoot),
		selectedRustRoot,
		cfg.Mode == rustModeAuto,
	)
	zigCandidates, activeZigBinary, activeZigVersion, activeZigSource, zigSelectedError := inspectRustToolCandidates(
		collectRustToolCandidatePaths(
			selectedZigBinary,
			cfg.KnownZigBinaries,
			[]string{rustToolExecutableName("zig")},
			zigToolFallbackCandidates(),
			discoverManagedZigBinaries(),
		),
		selectedZigBinary,
		readZigVersion,
	)

	activeRustRoot := ""
	activeRustVersion := ""
	activeRustSource := ""
	activeRustManaged := false
	activeCargoBinary := ""
	activeCargoVersion := ""
	activeCargoSource := ""
	activeRustupBinary := ""
	activeRustupVersion := ""
	activeRustupSource := ""
	activeRustcBinary := ""
	activeRustLayout := rustEnvironmentLayout{}
	activeCargoZigbuildBinary := ""
	activeCargoZigbuildVersion := ""
	activeCargoZigbuildSource := ""
	cargoZigbuildStatusMessage := ""
	canManageCargoZigbuild := false
	canManageTargets := false

	if activeRustCandidate != nil {
		activeRustRoot = activeRustCandidate.RootDir
		activeRustVersion = activeRustCandidate.Version
		activeRustSource = activeRustCandidate.Source
		activeRustManaged = activeRustCandidate.Managed
		if layout, err := resolveRustEnvironmentLayout(activeRustCandidate.RootDir); err == nil {
			activeRustLayout = layout
		}
		activeCargoBinary = activeRustCandidate.CargoBinary
		activeCargoVersion = activeRustCandidate.Version
		activeCargoSource = activeRustCandidate.Source
		activeRustupBinary = activeRustCandidate.RustupBinary
		activeRustupVersion, _ = readRustupVersion(activeRustupBinary)
		activeRustupSource = activeRustCandidate.Source
		activeRustcBinary = activeRustCandidate.RustcBinary
		activeCargoZigbuildBinary = activeRustCandidate.CargoZigbuildBinary
		activeCargoZigbuildVersion, _ = readCargoZigbuildVersion(activeCargoZigbuildBinary)
		activeCargoZigbuildSource = activeRustCandidate.Source
		canManageCargoZigbuild = activeRustManaged && activeCargoBinary != ""
		canManageTargets = activeRustManaged && activeRustupBinary != ""
		if activeCargoBinary == "" {
			cargoZigbuildStatusMessage = "当前 Rust SDK 缺少 cargo，可执行 cargo 插件安装"
		} else if !activeRustManaged {
			if activeCargoZigbuildBinary == "" {
				cargoZigbuildStatusMessage = "当前激活的是系统 Rust；为避免修改 ~/.cargo，已禁用自动补齐 cargo-zigbuild"
			} else {
				cargoZigbuildStatusMessage = "cargo-zigbuild 已就绪（系统 Rust）"
			}
		} else if activeCargoZigbuildBinary == "" {
			cargoZigbuildStatusMessage = "当前 Rust 环境缺少 cargo-zigbuild，可单独补齐"
		} else {
			cargoZigbuildStatusMessage = "cargo-zigbuild 已就绪"
		}
	}

	if cfg.Disabled {
		activeRustRoot = ""
		activeRustVersion = ""
		activeRustSource = ""
		activeRustManaged = false
		activeCargoBinary = ""
		activeCargoVersion = ""
		activeCargoSource = ""
		activeRustupBinary = ""
		activeRustupVersion = ""
		activeRustupSource = ""
		activeRustcBinary = ""
		activeRustLayout = rustEnvironmentLayout{}
		activeZigBinary = ""
		activeZigVersion = ""
		activeZigSource = ""
		activeCargoZigbuildBinary = ""
		activeCargoZigbuildVersion = ""
		activeCargoZigbuildSource = ""
		cargoZigbuildStatusMessage = ""
		canManageCargoZigbuild = false
		canManageTargets = false
	}

	targetStatuses := make([]RustTargetStatus, 0, len(rustSupportedTargets()))
	installedTargets := make([]string, 0, len(rustSupportedTargets()))
	hasInstalledTargetInfo := false
	hasFullTargetCoverage := false
	targetStatusMessage := ""
	if activeRustupBinary != "" {
		statuses, installed, err := inspectRustTargets(activeRustLayout)
		if err != nil {
			targetStatusMessage = fmt.Sprintf("无法读取已安装 Rust targets：%v", err)
		} else {
			targetStatuses = statuses
			installedTargets = installed
			hasInstalledTargetInfo = true
			hasFullTargetCoverage, targetStatusMessage = summarizeRustTargets(statuses)
		}
	} else if activeCargoBinary != "" {
		targetStatusMessage = "当前 Rust 环境缺少 rustup，无法自动补齐常用 targets"
	}
	if activeCargoBinary != "" && !activeRustManaged && !hasFullTargetCoverage {
		if targetStatusMessage == "" {
			targetStatusMessage = "当前激活的是系统 Rust；为避免修改 ~/.rustup，已禁用自动补齐常用 targets"
		} else {
			targetStatusMessage += "；当前激活的是系统 Rust，已禁用自动补齐常用 targets"
		}
	}

	statusParts := make([]string, 0, 5)
	switch {
	case cfg.Disabled:
		statusParts = append(statusParts, "已关闭 Rust SDK，远程执行、导出和交叉编译缓存将不可用")
	case activeCargoBinary == "":
		if rustSelectedError != "" {
			statusParts = append(statusParts, rustSelectedError)
		} else {
			statusParts = append(statusParts, "未检测到可用的 Rust SDK")
		}
	default:
		if activeZigBinary == "" {
			if zigSelectedError != "" {
				statusParts = append(statusParts, zigSelectedError)
			} else {
				statusParts = append(statusParts, "未检测到可用的 Zig SDK")
			}
		}
		if activeCargoZigbuildBinary == "" {
			statusParts = append(statusParts, cargoZigbuildStatusMessage)
		}
		if targetStatusMessage != "" && !hasFullTargetCoverage {
			statusParts = append(statusParts, targetStatusMessage)
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
		RustCandidates:             rustCandidates,
		ZigCandidates:              zigCandidates,
		InstalledTargets:           installedTargets,
		TargetStatuses:             targetStatuses,
		HasInstalledTargetInfo:     hasInstalledTargetInfo,
		HasFullTargetCoverage:      hasFullTargetCoverage,
		TargetStatusMessage:        targetStatusMessage,
		CargoZigbuildStatusMessage: cargoZigbuildStatusMessage,
		HasUsableEnvironment:       activeCargoBinary != "" && activeZigBinary != "" && activeCargoZigbuildBinary != "" && hasFullTargetCoverage,
		HasUsableRust:              activeCargoBinary != "",
		HasUsableCargo:             activeCargoBinary != "",
		HasUsableRustup:            activeRustupBinary != "",
		HasUsableZig:               activeZigBinary != "",
		HasUsableCargoZigbuild:     activeCargoZigbuildBinary != "",
		CanManageTargets:           canManageTargets,
		CanManageCargoZigbuild:     canManageCargoZigbuild,
		ActiveRustRoot:             activeRustRoot,
		ActiveRustVersion:          activeRustVersion,
		ActiveRustSource:           activeRustSource,
		ActiveRustManaged:          activeRustManaged,
		ActiveCargoBinary:          activeCargoBinary,
		ActiveCargoVersion:         activeCargoVersion,
		ActiveCargoSource:          activeCargoSource,
		ActiveRustupBinary:         activeRustupBinary,
		ActiveRustupVersion:        activeRustupVersion,
		ActiveRustupSource:         activeRustupSource,
		ActiveRustcBinary:          activeRustcBinary,
		ActiveZigBinary:            activeZigBinary,
		ActiveZigVersion:           activeZigVersion,
		ActiveZigSource:            activeZigSource,
		ActiveCargoZigbuildBinary:  activeCargoZigbuildBinary,
		ActiveCargoZigbuildVersion: activeCargoZigbuildVersion,
		ActiveCargoZigbuildSource:  activeCargoZigbuildSource,
		StatusMessage:              statusMessage,
		SuggestedInstallDirectory:  suggestedInstallDirectory,
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

func inspectRustTargets(layout rustEnvironmentLayout) ([]RustTargetStatus, []string, error) {
	installedTargets, err := readInstalledRustTargets(layout)
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

func readInstalledRustTargets(layout rustEnvironmentLayout) ([]string, error) {
	rustupBinary := layout.RustupBinary
	if _, err := os.Stat(rustupBinary); err != nil {
		return nil, fmt.Errorf("rustup 路径不存在")
	}
	cmd := procutil.Command(rustupBinary, "target", "list", "--installed")
	cmd.Env = append(os.Environ(), rustEnvironmentVars(layout, false)...)
	output, err := cmd.CombinedOutput()
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
	cfg.Mode = strings.ToLower(strings.TrimSpace(cfg.Mode))
	if cfg.Mode == "" {
		switch {
		case cfg.Disabled:
			cfg.Mode = rustModeNone
		case strings.TrimSpace(cfg.SelectedRustRoot) != "" || strings.TrimSpace(cfg.SelectedZigBinary) != "" || strings.TrimSpace(cfg.SelectedCargoBinary) != "" || strings.TrimSpace(cfg.SelectedRustupBinary) != "":
			cfg.Mode = rustModeManual
		default:
			cfg.Mode = rustModeAuto
		}
	}
	switch cfg.Mode {
	case rustModeAuto, rustModeManual:
		cfg.Disabled = false
	case rustModeNone:
		cfg.Disabled = true
	default:
		cfg.Mode = rustModeAuto
		cfg.Disabled = false
	}
	cfg.SelectedRustRoot = normalizeRustEnvironmentRoot(cfg.SelectedRustRoot)
	if cfg.SelectedRustRoot == "" {
		cfg.SelectedRustRoot = normalizeRustEnvironmentRoot(firstNonEmpty(
			inferRustEnvironmentRootFromBinary(cfg.SelectedCargoBinary),
			inferRustEnvironmentRootFromBinary(cfg.SelectedRustupBinary),
			inferRustEnvironmentRootFromBinary(cfg.SelectedCargoZigbuildBinary),
		))
	}
	knownRustRoots := make([]string, 0, len(cfg.KnownRustRoots)+len(cfg.KnownCargoBinaries)+len(cfg.KnownRustupBinaries)+len(cfg.KnownCargoZigbuildBinaries))
	knownRustRoots = append(knownRustRoots, cfg.KnownRustRoots...)
	for _, path := range cfg.KnownCargoBinaries {
		knownRustRoots = append(knownRustRoots, inferRustEnvironmentRootFromBinary(path))
	}
	for _, path := range cfg.KnownRustupBinaries {
		knownRustRoots = append(knownRustRoots, inferRustEnvironmentRootFromBinary(path))
	}
	for _, path := range cfg.KnownCargoZigbuildBinaries {
		knownRustRoots = append(knownRustRoots, inferRustEnvironmentRootFromBinary(path))
	}
	cfg.KnownRustRoots = dedupeStrings(normalizeRustRoots(knownRustRoots))
	cfg.SelectedCargoBinary = strings.TrimSpace(cfg.SelectedCargoBinary)
	cfg.KnownCargoBinaries = dedupeStrings(cfg.KnownCargoBinaries)
	cfg.SelectedRustupBinary = strings.TrimSpace(cfg.SelectedRustupBinary)
	cfg.KnownRustupBinaries = dedupeStrings(cfg.KnownRustupBinaries)
	cfg.SelectedZigBinary = strings.TrimSpace(cfg.SelectedZigBinary)
	cfg.KnownZigBinaries = dedupeStrings(cfg.KnownZigBinaries)
	cfg.SelectedCargoZigbuildBinary = strings.TrimSpace(cfg.SelectedCargoZigbuildBinary)
	cfg.KnownCargoZigbuildBinaries = dedupeStrings(cfg.KnownCargoZigbuildBinaries)
	cfg.LastInstallDirectory = NormalizeRustInstallBaseDirectory(cfg.LastInstallDirectory)
	return cfg
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func normalizeRustRoots(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if root := normalizeRustEnvironmentRoot(value); root != "" {
			result = append(result, root)
		}
	}
	return result
}

func collectRustEnvironmentCandidateRoots(cfg RustConfig, selectedRoot string) []rustCandidatePath {
	seen := map[string]int{}
	result := make([]rustCandidatePath, 0, 16)
	add := func(root string, source string) {
		root = normalizeRustEnvironmentRoot(root)
		if root == "" {
			return
		}
		identity := fileIdentity(root)
		if index, ok := seen[identity]; ok {
			result[index].source = preferCandidateSource(result[index].source, source)
			return
		}
		seen[identity] = len(result)
		result = append(result, rustCandidatePath{path: root, source: source})
	}
	if selectedRoot != "" {
		add(selectedRoot, sourceConfigured)
	}
	for _, root := range cfg.KnownRustRoots {
		add(root, sourceRemembered)
	}
	for _, path := range []string{cfg.SelectedCargoBinary, cfg.SelectedRustupBinary, cfg.SelectedCargoZigbuildBinary} {
		add(inferRustEnvironmentRootFromBinary(path), sourceConfigured)
	}
	for _, path := range append(append([]string{}, cfg.KnownCargoBinaries...), append(cfg.KnownRustupBinaries, cfg.KnownCargoZigbuildBinaries...)...) {
		add(inferRustEnvironmentRootFromBinary(path), sourceRemembered)
	}
	for _, name := range []string{rustToolExecutableName("cargo"), rustToolExecutableName("rustup")} {
		if path, err := exec.LookPath(name); err == nil && path != "" {
			add(inferRustEnvironmentRootFromBinary(path), sourcePath)
		}
	}
	for _, path := range cargoToolFallbackCandidates("cargo") {
		add(inferRustEnvironmentRootFromBinary(path), sourceDetected)
	}
	for _, root := range discoverManagedRustRoots() {
		add(root, sourceManaged)
	}
	return result
}

func inspectRustEnvironmentCandidates(paths []rustCandidatePath, selectedRoot string, preferManaged bool) ([]RustEnvironmentCandidate, *RustEnvironmentCandidate, string) {
	candidates := make([]RustEnvironmentCandidate, 0, len(paths))
	var activeCandidate *RustEnvironmentCandidate
	selectedError := ""
	for _, entry := range paths {
		candidate := inspectRustEnvironmentCandidate(entry.path, entry.source, selectedRoot)
		if candidate.Selected && !candidate.Valid && candidate.Error != "" {
			selectedError = candidate.Error
		}
		if candidate.Valid {
			switch {
			case activeCandidate == nil:
				candidateCopy := candidate
				activeCandidate = &candidateCopy
			case preferManaged && candidate.Managed && !activeCandidate.Managed:
				candidateCopy := candidate
				activeCandidate = &candidateCopy
			}
		}
		candidates = append(candidates, candidate)
	}
	if activeCandidate != nil {
		for i := range candidates {
			candidates[i].Active = candidates[i].Valid && candidates[i].RootDir == activeCandidate.RootDir
		}
	}
	return candidates, activeCandidate, selectedError
}

func inspectRustEnvironmentCandidate(root string, source string, selectedRoot string) RustEnvironmentCandidate {
	candidate := RustEnvironmentCandidate{
		RootDir:  root,
		Source:   source,
		Selected: sameNormalizedPath(root, selectedRoot),
		Detail:   root,
		Label:    filepath.Base(root),
	}
	layout, err := resolveRustEnvironmentLayout(root)
	if err != nil {
		candidate.Error = err.Error()
		return candidate
	}
	version, err := readCargoVersion(layout.CargoBinary)
	if err != nil {
		candidate.Error = err.Error()
		return candidate
	}
	candidate.Valid = true
	candidate.Version = version
	candidate.Label = version
	candidate.CargoBinary = layout.CargoBinary
	candidate.RustupBinary = existingPathOrEmpty(layout.RustupBinary)
	candidate.RustcBinary = existingPathOrEmpty(layout.RustcBinary)
	candidate.CargoZigbuildBinary = existingPathOrEmpty(layout.CargoZigbuildBinary)
	candidate.HasRustup = candidate.RustupBinary != ""
	candidate.HasCargoZigbuild = candidate.CargoZigbuildBinary != ""
	candidate.CanManageTargets = candidate.HasRustup
	candidate.Managed = layout.Managed
	return candidate
}

func existingPathOrEmpty(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return path
}

func sameNormalizedPath(left string, right string) bool {
	left = normalizeRustEnvironmentRoot(left)
	right = normalizeRustEnvironmentRoot(right)
	if left == "" || right == "" {
		return false
	}
	return sameFilePath(left, right)
}

func normalizeRustEnvironmentRoot(root string) string {
	root = filepath.Clean(strings.TrimSpace(root))
	if root == "" || root == "." {
		return ""
	}
	if existingPathOrEmpty(filepath.Join(root, "cargo", "bin", rustToolExecutableName("cargo"))) != "" {
		return root
	}
	if existingPathOrEmpty(filepath.Join(root, "bin", rustToolExecutableName("cargo"))) != "" {
		base := strings.ToLower(filepath.Base(root))
		if base == "cargo" && existingPathOrEmpty(filepath.Join(filepath.Dir(root), "cargo", "bin", rustToolExecutableName("cargo"))) != "" {
			return filepath.Dir(root)
		}
		return root
	}
	if strings.EqualFold(filepath.Base(root), "bin") && existingPathOrEmpty(filepath.Join(root, rustToolExecutableName("cargo"))) != "" {
		return normalizeRustEnvironmentRoot(filepath.Dir(root))
	}
	if strings.HasSuffix(strings.ToLower(filepath.Base(root)), strings.ToLower(rustToolExecutableName("cargo"))) {
		return inferRustEnvironmentRootFromBinary(root)
	}
	return root
}

func inferRustEnvironmentRootFromBinary(path string) string {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" || path == "." {
		return ""
	}
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return normalizeRustEnvironmentRoot(path)
	}
	dir := filepath.Dir(path)
	if strings.EqualFold(filepath.Base(dir), "bin") {
		parent := filepath.Dir(dir)
		if strings.EqualFold(filepath.Base(parent), "cargo") && existingPathOrEmpty(filepath.Join(filepath.Dir(parent), "cargo", "bin", rustToolExecutableName("cargo"))) != "" {
			return filepath.Dir(parent)
		}
		return parent
	}
	return normalizeRustEnvironmentRoot(dir)
}

func resolveRustEnvironmentLayout(root string) (rustEnvironmentLayout, error) {
	root = normalizeRustEnvironmentRoot(root)
	if root == "" {
		return rustEnvironmentLayout{}, fmt.Errorf("Rust SDK 目录为空")
	}
	managedCargoBinary := filepath.Join(root, "cargo", "bin", rustToolExecutableName("cargo"))
	if existingPathOrEmpty(managedCargoBinary) != "" {
		return rustEnvironmentLayout{
			RootDir:             root,
			CargoHome:           filepath.Join(root, "cargo"),
			RustupHome:          filepath.Join(root, "rustup"),
			CargoBinary:         managedCargoBinary,
			RustupBinary:        filepath.Join(root, "cargo", "bin", rustToolExecutableName("rustup")),
			RustcBinary:         filepath.Join(root, "cargo", "bin", rustToolExecutableName("rustc")),
			CargoZigbuildBinary: filepath.Join(root, "cargo", "bin", rustToolExecutableName("cargo-zigbuild")),
			Managed:             true,
		}, nil
	}
	cargoHome := root
	cargoBinary := filepath.Join(cargoHome, "bin", rustToolExecutableName("cargo"))
	if existingPathOrEmpty(cargoBinary) == "" {
		return rustEnvironmentLayout{}, fmt.Errorf("目录中未发现 cargo：%s", root)
	}
	return rustEnvironmentLayout{
		RootDir:             root,
		CargoHome:           cargoHome,
		RustupHome:          defaultUserRustupHome(),
		CargoBinary:         cargoBinary,
		RustupBinary:        filepath.Join(cargoHome, "bin", rustToolExecutableName("rustup")),
		RustcBinary:         filepath.Join(cargoHome, "bin", rustToolExecutableName("rustc")),
		CargoZigbuildBinary: filepath.Join(cargoHome, "bin", rustToolExecutableName("cargo-zigbuild")),
		Managed:             false,
	}, nil
}

func defaultUserRustupHome() string {
	if value := strings.TrimSpace(os.Getenv("RUSTUP_HOME")); value != "" {
		return value
	}
	home, _ := os.UserHomeDir()
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".rustup")
}

func discoverManagedRustRoots() []string {
	root, err := defaultToolchainInstallDirectory()
	if err != nil {
		return nil
	}
	entries, err := os.ReadDir(filepath.Join(root, rustManagedDirName))
	if err != nil {
		return nil
	}
	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, rustManagedDirName, entry.Name())
		if existingPathOrEmpty(filepath.Join(dir, "cargo", "bin", rustToolExecutableName("cargo"))) != "" {
			result = append(result, dir)
		}
	}
	sort.Strings(result)
	return result
}

func collectRustToolCandidatePaths(selected string, known []string, executableNames []string, fallbackPaths []string, managedPaths []string) []rustCandidatePath {
	seen := map[string]int{}
	result := make([]rustCandidatePath, 0, 16)
	add := func(path string, source string) {
		path = filepath.Clean(strings.TrimSpace(path))
		if path == "" {
			return
		}
		identity := fileIdentity(path)
		if index, ok := seen[identity]; ok {
			result[index].source = preferCandidateSource(result[index].source, source)
			return
		}
		seen[identity] = len(result)
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
	for _, path := range managedPaths {
		add(path, sourceManaged)
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
