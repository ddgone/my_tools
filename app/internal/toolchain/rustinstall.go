package toolchain

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"fire-salamander-desktop/internal/runtimeenv"
)

const (
	rustManagedDirName           = "rust"
	zigManagedDirName            = "zig"
	rustReleaseNotesURL          = "https://doc.rust-lang.org/nightly/releases.html"
	rustupOfficialDistServer     = "https://static.rust-lang.org"
	rustupOfficialUpdateRoot     = "https://static.rust-lang.org/rustup"
	rustupMirrorDistServer       = "https://mirrors.ustc.edu.cn/rust-static"
	rustupMirrorUpdateRoot       = "https://mirrors.ustc.edu.cn/rust-static/rustup"
	rustupInitDownloadBase       = "https://static.rust-lang.org/rustup/dist"
	rustupInitMirrorDownloadBase = "https://mirrors.ustc.edu.cn/rust-static/rustup/dist"
	zigIndexURL                  = "https://ziglang.org/download/index.json"
	zigCommunityMirrorsURL       = "https://ziglang.org/download/community-mirrors.txt"
	cargoMirrorSparseRegistry    = "sparse+https://rsproxy.cn/index/"
	rustReleaseCountLimit        = 20
	zigReleaseCountLimit         = 20
	defaultNetworkTimeout        = 20 * time.Second
)

var (
	rustVersionPattern = regexp.MustCompile(`Version (\d+\.\d+\.\d+)`)
	zigVersionPattern  = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
)

type RustOfficialRelease struct {
	Version string `json:"version"`
	Stable  bool   `json:"stable"`
	Channel bool   `json:"channel"`
}

type ZigOfficialRelease struct {
	Version string `json:"version"`
	Stable  bool   `json:"stable"`
	Date    string `json:"date,omitempty"`
}

type RustInstallProgress struct {
	Message          string  `json:"message"`
	Detail           string  `json:"detail,omitempty"`
	CurrentItem      string  `json:"currentItem,omitempty"`
	CurrentSource    string  `json:"currentSource,omitempty"`
	ProgressPercent  float64 `json:"progressPercent"`
	Step             int     `json:"step"`
	TotalSteps       int     `json:"totalSteps"`
	RustVersion      string  `json:"rustVersion,omitempty"`
	ZigVersion       string  `json:"zigVersion,omitempty"`
	Directory        string  `json:"directory,omitempty"`
	TransferredBytes int64   `json:"transferredBytes,omitempty"`
	TotalBytes       int64   `json:"totalBytes,omitempty"`
	TransferSpeed    string  `json:"transferSpeed,omitempty"`
}

type RustInstallHooks struct {
	OnProgress func(progress RustInstallProgress)
}

type ManagedRustInstallResult struct {
	RustVersion         string `json:"rustVersion"`
	ZigVersion          string `json:"zigVersion"`
	Directory           string `json:"directory"`
	RustDirectory       string `json:"rustDirectory"`
	ZigDirectory        string `json:"zigDirectory"`
	CargoBinary         string `json:"cargoBinary"`
	RustupBinary        string `json:"rustupBinary"`
	CargoZigbuildBinary string `json:"cargoZigbuildBinary"`
	ZigBinary           string `json:"zigBinary"`
}

type rustInstallError struct {
	Message string
	Detail  string
	Err     error
}

type zigDownloadEntry struct {
	Tarball string `json:"tarball"`
	Shasum  string `json:"shasum"`
	Size    string `json:"size"`
}

type zigIndexRelease struct {
	Version string `json:"version"`
	Date    string `json:"date"`
}

func (e *rustInstallError) Error() string {
	if text := strings.TrimSpace(e.Detail); text != "" {
		return text
	}
	if text := strings.TrimSpace(e.Message); text != "" {
		return text
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "Rust 交叉编译环境安装失败"
}

func (e *rustInstallError) Unwrap() error {
	return e.Err
}

func wrapRustInstallError(message string, err error) error {
	if err == nil {
		return &rustInstallError{
			Message: strings.TrimSpace(message),
			Detail:  strings.TrimSpace(message),
		}
	}
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	detail := strings.TrimSpace(err.Error())
	if detail == "" {
		detail = strings.TrimSpace(message)
	}
	return &rustInstallError{
		Message: strings.TrimSpace(message),
		Detail:  detail,
		Err:     err,
	}
}

func DescribeRustInstallError(err error) (string, string) {
	if err == nil {
		return "", ""
	}
	if errors.Is(err, context.Canceled) {
		return "Rust 交叉编译环境安装任务已停止", ""
	}
	var installErr *rustInstallError
	if errors.As(err, &installErr) {
		message := strings.TrimSpace(installErr.Message)
		detail := strings.TrimSpace(installErr.Detail)
		if message == "" {
			message = "Rust 交叉编译环境安装失败"
		}
		if detail == message {
			detail = ""
		}
		return message, detail
	}
	detail := strings.TrimSpace(err.Error())
	if detail == "" {
		detail = "Rust 交叉编译环境安装失败"
	}
	return "Rust 交叉编译环境安装失败", detail
}

func ListOfficialRustReleases() ([]RustOfficialRelease, error) {
	req, err := http.NewRequest(http.MethodGet, rustReleaseNotesURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建 Rust 版本列表请求失败: %w", err)
	}
	resp, err := (&http.Client{Timeout: defaultNetworkTimeout}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Rust 版本列表失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("请求 Rust 版本列表失败: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 Rust 版本列表失败: %w", err)
	}
	matches := rustVersionPattern.FindAllStringSubmatch(string(body), -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("未从官方页面解析到 Rust 版本列表")
	}
	seen := map[string]struct{}{
		"stable":  {},
		"beta":    {},
		"nightly": {},
	}
	result := []RustOfficialRelease{
		{Version: "stable", Stable: true, Channel: true},
		{Version: "beta", Stable: false, Channel: true},
		{Version: "nightly", Stable: false, Channel: true},
	}
	for _, match := range matches {
		version := strings.TrimSpace(match[1])
		if version == "" {
			continue
		}
		if _, ok := seen[version]; ok {
			continue
		}
		seen[version] = struct{}{}
		result = append(result, RustOfficialRelease{
			Version: version,
			Stable:  true,
		})
		if len(result) >= rustReleaseCountLimit+3 {
			break
		}
	}
	return result, nil
}

func ListOfficialZigReleases() ([]ZigOfficialRelease, error) {
	index, err := fetchZigIndex()
	if err != nil {
		return nil, err
	}
	type versionEntry struct {
		version string
		date    string
	}
	versions := make([]versionEntry, 0, len(index))
	for key, raw := range index {
		if !zigVersionPattern.MatchString(strings.TrimSpace(key)) {
			continue
		}
		release, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		entry := versionEntry{version: key}
		if date, ok := release["date"].(string); ok {
			entry.date = strings.TrimSpace(date)
		}
		if !zigReleaseSupportsHost(release) {
			continue
		}
		versions = append(versions, entry)
	}
	sort.Slice(versions, func(i, j int) bool {
		return compareSemverDesc(versions[i].version, versions[j].version) < 0
	})
	result := make([]ZigOfficialRelease, 0, minInt(len(versions), zigReleaseCountLimit))
	for _, version := range versions {
		result = append(result, ZigOfficialRelease{
			Version: version.version,
			Stable:  true,
			Date:    version.date,
		})
		if len(result) >= zigReleaseCountLimit {
			break
		}
	}
	return result, nil
}

func InstallManagedRustEnvironmentWithOptions(ctx context.Context, rustVersion string, zigVersion string, directory string, hooks *RustInstallHooks) (ManagedRustInstallResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	rustVersion = strings.TrimSpace(rustVersion)
	zigVersion = strings.TrimSpace(zigVersion)
	directory = filepath.Clean(strings.TrimSpace(directory))
	installRust := rustVersion != ""
	installZig := zigVersion != ""
	if !installRust && !installZig {
		return ManagedRustInstallResult{}, fmt.Errorf("至少选择一个需要安装的组件")
	}
	if directory == "" || directory == "." {
		return ManagedRustInstallResult{}, fmt.Errorf("安装位置不能为空")
	}

	rustDir := resolveManagedRustInstallDirectory(rustVersion, directory)
	zigDir := resolveManagedZigInstallDirectory(zigVersion, directory)
	cargoBinary := filepath.Join(rustDir, "cargo", "bin", rustToolExecutableName("cargo"))
	rustupBinary := filepath.Join(rustDir, "cargo", "bin", rustToolExecutableName("rustup"))
	cargoZigbuildBinary := filepath.Join(rustDir, "cargo", "bin", rustToolExecutableName("cargo-zigbuild"))
	zigBinary := filepath.Join(zigDir, rustToolExecutableName("zig"))

	emitRustInstallProgress(hooks, RustInstallProgress{
		Message:         "准备安装 Rust 交叉编译环境",
		CurrentItem:     rustVersion,
		ProgressPercent: 0,
		Step:            1,
		TotalSteps:      7,
		RustVersion:     rustVersion,
		ZigVersion:      zigVersion,
		Directory:       directory,
	})
	if installRust {
		if err := os.MkdirAll(filepath.Join(directory, rustManagedDirName), 0755); err != nil {
			return ManagedRustInstallResult{}, wrapRustInstallError("创建 Rust 安装目录失败", err)
		}
	}
	if installZig {
		if err := os.MkdirAll(filepath.Join(directory, zigManagedDirName), 0755); err != nil {
			return ManagedRustInstallResult{}, wrapRustInstallError("创建 Zig 安装目录失败", err)
		}
	}

	if installRust {
		emitRustInstallProgress(hooks, RustInstallProgress{
			Message:         "正在准备 Rust toolchain",
			CurrentItem:     rustVersion,
			ProgressPercent: 8,
			Step:            2,
			TotalSteps:      7,
			RustVersion:     rustVersion,
			ZigVersion:      zigVersion,
			Directory:       rustDir,
		})
		if err := installManagedRustToolchain(ctx, rustVersion, rustDir, hooks); err != nil {
			return ManagedRustInstallResult{}, err
		}

		emitRustInstallProgress(hooks, RustInstallProgress{
			Message:         "正在安装 cargo-zigbuild",
			CurrentItem:     "cargo-zigbuild",
			ProgressPercent: 42,
			Step:            3,
			TotalSteps:      7,
			RustVersion:     rustVersion,
			ZigVersion:      zigVersion,
			Directory:       rustDir,
		})
		if err := installCargoZigbuild(ctx, rustDir, cargoBinary, hooks); err != nil {
			return ManagedRustInstallResult{}, err
		}

		emitRustInstallProgress(hooks, RustInstallProgress{
			Message:         "正在补齐常用 Rust targets",
			CurrentItem:     rustVersion,
			ProgressPercent: 56,
			Step:            4,
			TotalSteps:      7,
			RustVersion:     rustVersion,
			ZigVersion:      zigVersion,
			Directory:       rustDir,
		})
		if err := installManagedRustTargets(ctx, rustDir, rustupBinary, hooks); err != nil {
			return ManagedRustInstallResult{}, err
		}
	}

	if installZig {
		emitRustInstallProgress(hooks, RustInstallProgress{
			Message:         "正在读取 Zig 版本索引",
			CurrentItem:     zigVersion,
			ProgressPercent: 68,
			Step:            5,
			TotalSteps:      7,
			RustVersion:     rustVersion,
			ZigVersion:      zigVersion,
			Directory:       zigDir,
		})
		zigAsset, err := resolveZigAsset(zigVersion)
		if err != nil {
			return ManagedRustInstallResult{}, wrapRustInstallError("获取 Zig 版本索引失败", err)
		}

		emitRustInstallProgress(hooks, RustInstallProgress{
			Message:         "正在下载 Zig",
			CurrentItem:     zigAsset.filename,
			ProgressPercent: 72,
			Step:            6,
			TotalSteps:      7,
			RustVersion:     rustVersion,
			ZigVersion:      zigVersion,
			Directory:       zigDir,
		})
		if err := installManagedZig(ctx, zigVersion, zigDir, zigAsset, hooks); err != nil {
			return ManagedRustInstallResult{}, err
		}
	}

	if installRust {
		if _, err := readCargoVersion(cargoBinary); err != nil {
			return ManagedRustInstallResult{}, wrapRustInstallError("Rust toolchain 安装不完整", err)
		}
		if _, err := readRustupVersion(rustupBinary); err != nil {
			return ManagedRustInstallResult{}, wrapRustInstallError("Rust toolchain 安装不完整", err)
		}
		if _, err := readCargoZigbuildVersion(cargoZigbuildBinary); err != nil {
			return ManagedRustInstallResult{}, wrapRustInstallError("cargo-zigbuild 安装不完整", err)
		}
	}
	if installZig {
		if _, err := readZigVersion(zigBinary); err != nil {
			return ManagedRustInstallResult{}, wrapRustInstallError("Zig 安装不完整", err)
		}
	}

	emitRustInstallProgress(hooks, RustInstallProgress{
		Message:         "Rust 交叉编译环境安装完成",
		Detail:          directory,
		CurrentItem:     rustVersion,
		ProgressPercent: 100,
		Step:            7,
		TotalSteps:      7,
		RustVersion:     rustVersion,
		ZigVersion:      zigVersion,
		Directory:       directory,
	})
	return ManagedRustInstallResult{
		RustVersion:         rustVersion,
		ZigVersion:          zigVersion,
		Directory:           directory,
		RustDirectory:       rustDir,
		ZigDirectory:        zigDir,
		CargoBinary:         cargoBinary,
		RustupBinary:        rustupBinary,
		CargoZigbuildBinary: cargoZigbuildBinary,
		ZigBinary:           zigBinary,
	}, nil
}

func InstallCargoZigbuildForActiveRust(ctx context.Context, hooks *RustInstallHooks) error {
	if ctx == nil {
		ctx = context.Background()
	}
	state, err := GetRustState()
	if err != nil {
		return err
	}
	if strings.TrimSpace(state.ActiveRustRoot) == "" || strings.TrimSpace(state.ActiveCargoBinary) == "" {
		return wrapRustInstallError("当前未选择可用的 Rust SDK", fmt.Errorf("请先选择或安装一个可用的 Rust SDK"))
	}
	if !state.ActiveRustManaged {
		return wrapRustInstallError("当前激活的是系统 Rust", fmt.Errorf("为避免修改 ~/.cargo，当前仅允许对托管 Rust 执行自动补齐 cargo-zigbuild"))
	}
	layout, err := resolveRustEnvironmentLayout(state.ActiveRustRoot)
	if err != nil {
		return wrapRustInstallError("解析当前 Rust SDK 目录失败", err)
	}
	emitRustInstallProgress(hooks, RustInstallProgress{
		Message:         "正在补齐 cargo-zigbuild",
		CurrentItem:     "cargo-zigbuild",
		ProgressPercent: 10,
		Step:            1,
		TotalSteps:      1,
		Directory:       state.ActiveRustRoot,
	})
	if err := installCargoZigbuildWithLayout(ctx, layout, hooks); err != nil {
		return err
	}
	return nil
}

func InstallTargetsForActiveRust(ctx context.Context, hooks *RustInstallHooks) error {
	if ctx == nil {
		ctx = context.Background()
	}
	state, err := GetRustState()
	if err != nil {
		return err
	}
	if strings.TrimSpace(state.ActiveRustRoot) == "" || strings.TrimSpace(state.ActiveRustupBinary) == "" {
		return wrapRustInstallError("当前 Rust 环境缺少 rustup", fmt.Errorf("请先选择带 rustup 的 Rust SDK"))
	}
	if !state.ActiveRustManaged {
		return wrapRustInstallError("当前激活的是系统 Rust", fmt.Errorf("为避免修改 ~/.rustup，当前仅允许对托管 Rust 执行自动补齐常用 targets"))
	}
	layout, err := resolveRustEnvironmentLayout(state.ActiveRustRoot)
	if err != nil {
		return wrapRustInstallError("解析当前 Rust SDK 目录失败", err)
	}
	emitRustInstallProgress(hooks, RustInstallProgress{
		Message:         "正在补齐常用 Rust targets",
		CurrentItem:     state.ActiveRustVersion,
		ProgressPercent: 10,
		Step:            1,
		TotalSteps:      1,
		Directory:       state.ActiveRustRoot,
	})
	if err := installManagedRustTargetsWithLayout(ctx, layout, hooks); err != nil {
		return err
	}
	return nil
}

func NormalizeRustInstallBaseDirectory(directory string) string {
	directory = filepath.Clean(strings.TrimSpace(directory))
	if directory == "" || directory == "." {
		return ""
	}
	base := strings.ToLower(strings.TrimSpace(filepath.Base(directory)))
	parent := filepath.Dir(directory)
	parentBase := strings.ToLower(strings.TrimSpace(filepath.Base(parent)))
	switch {
	case zigVersionPattern.MatchString(base) && (parentBase == rustManagedDirName || parentBase == zigManagedDirName):
		return filepath.Dir(parent)
	case base == rustManagedDirName || base == zigManagedDirName:
		return parent
	default:
		return directory
	}
}

func defaultToolchainInstallDirectory() (string, error) {
	layout, err := runtimeenv.ResolveLayout()
	if err != nil {
		return "", err
	}
	return filepath.Join(layout.Root, "toolchains"), nil
}

func resolveManagedRustInstallDirectory(version string, baseDir string) string {
	baseDir = filepath.Clean(strings.TrimSpace(baseDir))
	return filepath.Join(baseDir, rustManagedDirName, strings.ToLower(strings.TrimSpace(version)))
}

func resolveManagedZigInstallDirectory(version string, baseDir string) string {
	baseDir = filepath.Clean(strings.TrimSpace(baseDir))
	return filepath.Join(baseDir, zigManagedDirName, strings.ToLower(strings.TrimSpace(version)))
}

func discoverManagedRustBinaries() []string {
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
		binaryPath := filepath.Join(root, rustManagedDirName, entry.Name(), "cargo", "bin", rustToolExecutableName("cargo"))
		if _, err := os.Stat(binaryPath); err == nil {
			result = append(result, binaryPath)
		}
	}
	sort.Strings(result)
	return result
}

func discoverManagedRustupBinaries() []string {
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
		binaryPath := filepath.Join(root, rustManagedDirName, entry.Name(), "cargo", "bin", rustToolExecutableName("rustup"))
		if _, err := os.Stat(binaryPath); err == nil {
			result = append(result, binaryPath)
		}
	}
	sort.Strings(result)
	return result
}

func discoverManagedCargoZigbuildBinaries() []string {
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
		binaryPath := filepath.Join(root, rustManagedDirName, entry.Name(), "cargo", "bin", rustToolExecutableName("cargo-zigbuild"))
		if _, err := os.Stat(binaryPath); err == nil {
			result = append(result, binaryPath)
		}
	}
	sort.Strings(result)
	return result
}

func discoverManagedZigBinaries() []string {
	root, err := defaultToolchainInstallDirectory()
	if err != nil {
		return nil
	}
	entries, err := os.ReadDir(filepath.Join(root, zigManagedDirName))
	if err != nil {
		return nil
	}
	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		binaryPath := filepath.Join(root, zigManagedDirName, entry.Name(), rustToolExecutableName("zig"))
		if _, err := os.Stat(binaryPath); err == nil {
			result = append(result, binaryPath)
		}
	}
	sort.Strings(result)
	return result
}

func resolveManagedRustDirectory(binaryPath string) (string, bool) {
	root, err := defaultToolchainInstallDirectory()
	if err != nil {
		return "", false
	}
	binaryPath = filepath.Clean(strings.TrimSpace(binaryPath))
	if binaryPath == "" {
		return "", false
	}
	envDir := filepath.Dir(filepath.Dir(filepath.Dir(binaryPath)))
	managedRoot := filepath.Join(root, rustManagedDirName)
	rel, err := filepath.Rel(managedRoot, envDir)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return "", false
	}
	return envDir, true
}

func resolveManagedZigDirectory(binaryPath string) (string, bool) {
	root, err := defaultToolchainInstallDirectory()
	if err != nil {
		return "", false
	}
	binaryPath = filepath.Clean(strings.TrimSpace(binaryPath))
	if binaryPath == "" {
		return "", false
	}
	envDir := filepath.Dir(binaryPath)
	managedRoot := filepath.Join(root, zigManagedDirName)
	rel, err := filepath.Rel(managedRoot, envDir)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return "", false
	}
	return envDir, true
}

func DeleteManagedRustEnvironment() (RustToolchainState, error) {
	state, err := GetRustState()
	if err != nil {
		return RustToolchainState{}, err
	}
	activeRustDir, hasManagedRust := resolveManagedRustDirectory(state.ActiveCargoBinary)
	activeZigDir, hasManagedZig := resolveManagedZigDirectory(state.ActiveZigBinary)
	if !hasManagedRust && !hasManagedZig {
		return state, fmt.Errorf("当前 Rust 环境不是托管工具链，无法在这里删除")
	}
	if hasManagedRust {
		if err := os.RemoveAll(activeRustDir); err != nil {
			return RustToolchainState{}, fmt.Errorf("删除托管 Rust 环境失败: %w", err)
		}
	}
	if hasManagedZig {
		if err := os.RemoveAll(activeZigDir); err != nil {
			return RustToolchainState{}, fmt.Errorf("删除托管 Zig 环境失败: %w", err)
		}
	}
	cfg, err := LoadRustConfig()
	if err != nil {
		return RustToolchainState{}, err
	}
	cfg.KnownCargoBinaries = filterKnownRustPaths(cfg.KnownCargoBinaries, activeRustDir)
	cfg.KnownRustupBinaries = filterKnownRustPaths(cfg.KnownRustupBinaries, activeRustDir)
	cfg.KnownCargoZigbuildBinaries = filterKnownRustPaths(cfg.KnownCargoZigbuildBinaries, activeRustDir)
	cfg.KnownZigBinaries = filterKnownRustPaths(cfg.KnownZigBinaries, activeZigDir)
	if hasManagedRust && isPathWithinDir(cfg.SelectedCargoBinary, activeRustDir) {
		cfg.SelectedCargoBinary = ""
	}
	if hasManagedRust && isPathWithinDir(cfg.SelectedRustupBinary, activeRustDir) {
		cfg.SelectedRustupBinary = ""
	}
	if hasManagedRust && isPathWithinDir(cfg.SelectedCargoZigbuildBinary, activeRustDir) {
		cfg.SelectedCargoZigbuildBinary = ""
	}
	if hasManagedZig && isPathWithinDir(cfg.SelectedZigBinary, activeZigDir) {
		cfg.SelectedZigBinary = ""
	}
	if err := SaveRustConfig(cfg); err != nil {
		return RustToolchainState{}, err
	}
	return GetRustState()
}

func installManagedRustToolchain(ctx context.Context, rustVersion string, rustDir string, hooks *RustInstallHooks) error {
	cargoBinary := filepath.Join(rustDir, "cargo", "bin", rustToolExecutableName("cargo"))
	rustupBinary := filepath.Join(rustDir, "cargo", "bin", rustToolExecutableName("rustup"))
	if _, cargoErr := readCargoVersion(cargoBinary); cargoErr == nil {
		if _, rustupErr := readRustupVersion(rustupBinary); rustupErr == nil {
			return nil
		}
	}

	if err := os.MkdirAll(rustDir, 0755); err != nil {
		return wrapRustInstallError("创建 Rust 版本目录失败", err)
	}
	tempDir, err := os.MkdirTemp("", "fire-salamander-rustup-init-*")
	if err != nil {
		return wrapRustInstallError("创建 rustup-init 临时目录失败", err)
	}
	defer os.RemoveAll(tempDir)
	tempPath := filepath.Join(tempDir, rustupInitExecutableName())
	tempFile, err := os.OpenFile(tempPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0755)
	if err != nil {
		return wrapRustInstallError("创建 rustup-init 下载缓存失败", err)
	}
	defer tempFile.Close()

	hostTriple, err := rustupHostTriple()
	if err != nil {
		return wrapRustInstallError("当前宿主平台暂不支持自动安装 Rust", err)
	}
	filename := "rustup-init"
	if runtime.GOOS == "windows" {
		filename = "rustup-init.exe"
	}
	downloadURLs := []namedDownloadSource{
		{
			Name: "static.rust-lang.org",
			URL:  strings.TrimRight(rustupInitDownloadBase, "/") + "/" + hostTriple + "/" + filename,
		},
		{
			Name: "mirrors.ustc.edu.cn",
			URL:  strings.TrimRight(rustupInitMirrorDownloadBase, "/") + "/" + hostTriple + "/" + filename,
		},
	}

	if err := downloadFromSources(ctx, tempFile, downloadURLs, func(downloaded int64, total int64, speed string, source string) {
		progress := 12.0
		if total > 0 {
			progress = 12 + float64(downloaded)*18/float64(total)
		}
		emitRustInstallProgress(hooks, RustInstallProgress{
			Message:          "正在下载 rustup-init",
			CurrentItem:      filename,
			CurrentSource:    source,
			ProgressPercent:  roundProgressPercent(progress),
			Step:             2,
			TotalSteps:       7,
			RustVersion:      rustVersion,
			Directory:        rustDir,
			TransferredBytes: downloaded,
			TotalBytes:       total,
			TransferSpeed:    speed,
		})
	}); err != nil {
		return wrapRustInstallError("下载 rustup-init 失败", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(tempPath, 0755); err != nil {
			return wrapRustInstallError("设置 rustup-init 可执行权限失败", err)
		}
	}
	if err := tempFile.Close(); err != nil {
		return wrapRustInstallError("写入 rustup-init 下载缓存失败", err)
	}

	runInstaller := func(useMirror bool) error {
		sourceName := "static.rust-lang.org"
		if useMirror {
			sourceName = "mirrors.ustc.edu.cn"
		}
		emitRustInstallProgress(hooks, RustInstallProgress{
			Message:         "正在安装 Rust toolchain",
			Detail:          "rustup-init 已下载完成，正在启动安装器",
			CurrentItem:     rustVersion,
			CurrentSource:   sourceName,
			ProgressPercent: 32,
			Step:            2,
			TotalSteps:      7,
			RustVersion:     rustVersion,
			Directory:       rustDir,
		})
		args := []string{
			"-y",
			"--profile", "minimal",
			"--default-toolchain", rustVersion,
			"--no-modify-path",
		}
		cmd := exec.CommandContext(ctx, tempPath, args...)
		cmd.Env = append(os.Environ(), managedRustEnvironmentVars(rustDir, useMirror)...)
		output, err := runCommandWithLiveOutput(
			ctx,
			cmd,
			func(line string) {
				emitRustInstallProgress(hooks, RustInstallProgress{
					Message:         "正在安装 Rust toolchain",
					Detail:          summarizeCommandProgressLine(line),
					CurrentItem:     rustVersion,
					CurrentSource:   sourceName,
					ProgressPercent: 34,
					Step:            2,
					TotalSteps:      7,
					RustVersion:     rustVersion,
					Directory:       rustDir,
				})
			},
			func(elapsed time.Duration, lastLine string) {
				detail := fmt.Sprintf("rustup-init 正在运行，已耗时 %s", formatElapsedDuration(elapsed))
				if text := summarizeCommandProgressLine(lastLine); text != "" {
					detail = fmt.Sprintf("%s，已耗时 %s", text, formatElapsedDuration(elapsed))
				}
				emitRustInstallProgress(hooks, RustInstallProgress{
					Message:         "正在安装 Rust toolchain",
					Detail:          detail,
					CurrentItem:     rustVersion,
					CurrentSource:   sourceName,
					ProgressPercent: 34,
					Step:            2,
					TotalSteps:      7,
					RustVersion:     rustVersion,
					Directory:       rustDir,
				})
			},
		)
		if err != nil {
			if errors.Is(ctx.Err(), context.Canceled) {
				return context.Canceled
			}
			detail := strings.TrimSpace(string(output))
			if detail == "" {
				detail = strings.TrimSpace(err.Error())
			}
			return fmt.Errorf("%s", detail)
		}
		return nil
	}
	if err := runInstaller(false); err != nil {
		emitRustInstallProgress(hooks, RustInstallProgress{
			Message:         "官方源安装失败，正在切换镜像源重试",
			Detail:          summarizeCommandProgressLine(err.Error()),
			CurrentItem:     rustVersion,
			CurrentSource:   "mirrors.ustc.edu.cn",
			ProgressPercent: 34,
			Step:            2,
			TotalSteps:      7,
			RustVersion:     rustVersion,
			Directory:       rustDir,
		})
		if mirrorErr := runInstaller(true); mirrorErr != nil {
			return wrapRustInstallError("安装 Rust toolchain 失败", fmt.Errorf("官方源失败：%v；镜像源失败：%v", err, mirrorErr))
		}
	}
	if _, err := readCargoVersion(cargoBinary); err != nil {
		return wrapRustInstallError("Rust toolchain 安装不完整", fmt.Errorf("未在托管目录发现 cargo：%w", err))
	}
	if _, err := readRustupVersion(rustupBinary); err != nil {
		return wrapRustInstallError("Rust toolchain 安装不完整", fmt.Errorf("未在托管目录发现 rustup：%w", err))
	}
	return nil
}

func installCargoZigbuild(ctx context.Context, rustDir string, cargoBinary string, _ *RustInstallHooks) error {
	layout, err := resolveRustEnvironmentLayout(rustDir)
	if err != nil {
		return wrapRustInstallError("解析 Rust SDK 目录失败", err)
	}
	layout.CargoBinary = cargoBinary
	return installCargoZigbuildWithLayout(ctx, layout, nil)
}

func installCargoZigbuildWithLayout(ctx context.Context, layout rustEnvironmentLayout, hooks *RustInstallHooks) error {
	cargoZigbuildBinary := layout.CargoZigbuildBinary
	if _, err := readCargoZigbuildVersion(cargoZigbuildBinary); err == nil {
		return nil
	}
	runInstall := func(useMirror bool) error {
		restore, err := configureManagedCargoMirror(layout.CargoHome, useMirror)
		if err != nil {
			return err
		}
		defer restore()

		cmd := exec.CommandContext(ctx, layout.CargoBinary, "install", "cargo-zigbuild", "--locked")
		cmd.Env = append(os.Environ(), rustEnvironmentVars(layout, useMirror)...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			if errors.Is(ctx.Err(), context.Canceled) {
				return context.Canceled
			}
			detail := strings.TrimSpace(string(output))
			if detail == "" {
				detail = strings.TrimSpace(err.Error())
			}
			return fmt.Errorf("%s", detail)
		}
		return nil
	}
	if err := runInstall(false); err != nil {
		if mirrorErr := runInstall(true); mirrorErr != nil {
			return wrapRustInstallError("安装 cargo-zigbuild 失败", fmt.Errorf("官方源失败：%v；镜像源失败：%v", err, mirrorErr))
		}
	}
	emitRustInstallProgress(hooks, RustInstallProgress{
		Message:         "cargo-zigbuild 已补齐",
		CurrentItem:     "cargo-zigbuild",
		ProgressPercent: 100,
		Step:            1,
		TotalSteps:      1,
		Directory:       layout.RootDir,
	})
	return nil
}

func installManagedRustTargets(ctx context.Context, rustDir string, rustupBinary string, hooks *RustInstallHooks) error {
	layout, err := resolveRustEnvironmentLayout(rustDir)
	if err != nil {
		return wrapRustInstallError("解析 Rust SDK 目录失败", err)
	}
	layout.RustupBinary = rustupBinary
	return installManagedRustTargetsWithLayout(ctx, layout, hooks)
}

func installManagedRustTargetsWithLayout(ctx context.Context, layout rustEnvironmentLayout, hooks *RustInstallHooks) error {
	rustupBinary := layout.RustupBinary
	computeMissingTargets := func() ([]string, error) {
		installed, err := readInstalledRustTargets(layout)
		if err != nil {
			return nil, err
		}
		installedSet := make(map[string]struct{}, len(installed))
		for _, target := range installed {
			installedSet[target] = struct{}{}
		}
		targets := make([]string, 0, len(rustSupportedTargets()))
		for _, target := range rustSupportedTargets() {
			if target.native {
				continue
			}
			if _, ok := installedSet[target.targetTriple]; ok {
				continue
			}
			targets = append(targets, target.targetTriple)
		}
		return targets, nil
	}

	targets, err := computeMissingTargets()
	if err != nil {
		targets = nil
	}
	if len(targets) == 0 {
		return nil
	}
	requestedTargets := append([]string(nil), targets...)
	runAddTargets := func(target string, useMirror bool) error {
		args := []string{"target", "add", target}
		cmd := exec.CommandContext(ctx, rustupBinary, args...)
		cmd.Env = append(os.Environ(), rustEnvironmentVars(layout, useMirror)...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			if errors.Is(ctx.Err(), context.Canceled) {
				return context.Canceled
			}
			detail := strings.TrimSpace(string(output))
			if detail == "" {
				detail = strings.TrimSpace(err.Error())
			}
			return fmt.Errorf("%s", detail)
		}
		return nil
	}

	for index, target := range targets {
		emitRustInstallProgress(hooks, RustInstallProgress{
			Message:         "正在补齐常用 Rust targets",
			CurrentItem:     target,
			Detail:          fmt.Sprintf("第 %d/%d 个 target", index+1, len(targets)),
			ProgressPercent: roundProgressPercent(56 + float64(index)*12/float64(max(len(targets), 1))),
			Step:            1,
			TotalSteps:      1,
			Directory:       layout.RootDir,
		})
		if err := runAddTargets(target, false); err != nil {
			if mirrorErr := runAddTargets(target, true); mirrorErr != nil {
				return wrapRustInstallError("安装常用 Rust targets 失败", fmt.Errorf("%s：官方源失败：%v；镜像源失败：%v", target, err, mirrorErr))
			}
		}
	}

	if targets, err = computeMissingTargets(); err != nil {
		return wrapRustInstallError("校验已安装 Rust targets 失败", err)
	}
	if len(targets) > 0 {
		return wrapRustInstallError("常用 Rust targets 安装不完整", fmt.Errorf("仍缺少 %s", strings.Join(targets, "、")))
	}

	emitRustInstallProgress(hooks, RustInstallProgress{
		Message:         "常用 Rust targets 已补齐",
		CurrentItem:     strings.Join(requestedTargets, "、"),
		ProgressPercent: 100,
		Step:            1,
		TotalSteps:      1,
		Directory:       layout.RootDir,
	})
	return nil
}

func installManagedZig(ctx context.Context, zigVersion string, zigDir string, asset zigResolvedAsset, hooks *RustInstallHooks) error {
	zigBinary := filepath.Join(zigDir, rustToolExecutableName("zig"))
	if _, err := readZigVersion(zigBinary); err == nil {
		return nil
	}
	if err := os.RemoveAll(zigDir); err != nil {
		return wrapRustInstallError("清理旧 Zig 目录失败", err)
	}
	if err := os.MkdirAll(zigDir, 0755); err != nil {
		return wrapRustInstallError("创建 Zig 版本目录失败", err)
	}
	tempFile, err := os.CreateTemp("", "fire-salamander-zig-*"+filepath.Ext(asset.filename))
	if err != nil {
		return wrapRustInstallError("创建 Zig 下载缓存失败", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)
	defer tempFile.Close()

	sources, err := zigDownloadSources(asset.url)
	if err != nil {
		return wrapRustInstallError("准备 Zig 下载源失败", err)
	}
	if err := downloadFromSources(ctx, tempFile, sources, func(downloaded int64, total int64, speed string, source string) {
		progress := 72.0
		if total > 0 {
			progress = 72 + float64(downloaded)*20/float64(total)
		}
		emitRustInstallProgress(hooks, RustInstallProgress{
			Message:          "正在下载 Zig",
			CurrentItem:      asset.filename,
			CurrentSource:    source,
			ProgressPercent:  roundProgressPercent(progress),
			Step:             6,
			TotalSteps:       7,
			ZigVersion:       zigVersion,
			Directory:        zigDir,
			TransferredBytes: downloaded,
			TotalBytes:       total,
			TransferSpeed:    speed,
		})
	}); err != nil {
		return wrapRustInstallError("下载 Zig 失败", err)
	}
	if err := verifySHA256(tempPath, asset.shasum); err != nil {
		return wrapRustInstallError("校验 Zig 安装包失败", err)
	}

	emitRustInstallProgress(hooks, RustInstallProgress{
		Message:         "正在解压 Zig",
		CurrentItem:     asset.filename,
		ProgressPercent: 94,
		Step:            7,
		TotalSteps:      7,
		ZigVersion:      zigVersion,
		Directory:       zigDir,
	})
	switch {
	case strings.HasSuffix(strings.ToLower(asset.filename), ".zip"):
		if err := extractZip(ctx, tempPath, zigDir); err != nil {
			return wrapRustInstallError("解压 Zig 安装包失败", err)
		}
	case strings.HasSuffix(strings.ToLower(asset.filename), ".tar.xz"):
		if err := extractTarXz(ctx, tempPath, zigDir); err != nil {
			return wrapRustInstallError("解压 Zig 安装包失败", err)
		}
	default:
		return wrapRustInstallError("当前 Zig 安装包格式暂不支持自动解压", fmt.Errorf("不支持的 Zig 安装包格式: %s", asset.filename))
	}
	return nil
}

func managedRustEnvironmentVars(rustDir string, useMirror bool) []string {
	return rustEnvironmentVars(managedRustEnvironmentLayout(rustDir), useMirror)
}

func managedRustEnvironmentLayout(rustDir string) rustEnvironmentLayout {
	rustDir = normalizeRustEnvironmentRoot(rustDir)
	cargoHome := filepath.Join(rustDir, "cargo")
	return rustEnvironmentLayout{
		RootDir:             rustDir,
		CargoHome:           cargoHome,
		RustupHome:          filepath.Join(rustDir, "rustup"),
		CargoBinary:         filepath.Join(cargoHome, "bin", rustToolExecutableName("cargo")),
		RustupBinary:        filepath.Join(cargoHome, "bin", rustToolExecutableName("rustup")),
		RustcBinary:         filepath.Join(cargoHome, "bin", rustToolExecutableName("rustc")),
		CargoZigbuildBinary: filepath.Join(cargoHome, "bin", rustToolExecutableName("cargo-zigbuild")),
		Managed:             true,
	}
}

func rustEnvironmentVars(layout rustEnvironmentLayout, useMirror bool) []string {
	cargoHome := layout.CargoHome
	rustupHome := layout.RustupHome
	pathValue := prependPathEntries(os.Getenv("PATH"), filepath.Join(cargoHome, "bin"))
	env := []string{"PATH=" + pathValue, "RUSTUP_INIT_SKIP_PATH_CHECK=yes"}
	if cargoHome != "" {
		env = append(env, "CARGO_HOME="+cargoHome)
	}
	if rustupHome != "" {
		env = append(env, "RUSTUP_HOME="+rustupHome)
	}
	if useMirror {
		env = append(env,
			"RUSTUP_DIST_SERVER="+rustupMirrorDistServer,
			"RUSTUP_UPDATE_ROOT="+rustupMirrorUpdateRoot,
		)
	} else {
		env = append(env,
			"RUSTUP_DIST_SERVER="+rustupOfficialDistServer,
			"RUSTUP_UPDATE_ROOT="+rustupOfficialUpdateRoot,
		)
	}
	return env
}

func configureManagedCargoMirror(cargoHome string, useMirror bool) (func(), error) {
	configPath := filepath.Join(cargoHome, "config.toml")
	backupPath := configPath + ".bak-fire-salamander"
	restore := func() {}
	if !useMirror {
		return restore, nil
	}
	if err := os.MkdirAll(cargoHome, 0755); err != nil {
		return nil, err
	}
	if _, err := os.Stat(configPath); err == nil {
		if err := os.Rename(configPath, backupPath); err != nil {
			return nil, err
		}
		restore = func() {
			_ = os.Remove(configPath)
			_ = os.Rename(backupPath, configPath)
		}
	} else {
		restore = func() {
			_ = os.Remove(configPath)
		}
	}
	content := strings.TrimSpace(`
[source.crates-io]
replace-with = "rsproxy-sparse"

[source.rsproxy-sparse]
registry = "sparse+https://rsproxy.cn/index/"
`) + "\n"
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		restore()
		return nil, err
	}
	return restore, nil
}

type namedDownloadSource struct {
	Name string
	URL  string
}

func downloadFromSources(ctx context.Context, file *os.File, sources []namedDownloadSource, onProgress func(downloaded int64, total int64, speed string, source string)) error {
	failures := make([]string, 0, len(sources))
	for _, source := range sources {
		if err := resetDownloadFile(file); err != nil {
			return fmt.Errorf("重置下载缓存失败: %w", err)
		}
		if onProgress != nil {
			onProgress(0, 0, "", source.Name)
		}
		if err := downloadFromURL(ctx, source.URL, file, source.Name, onProgress); err == nil {
			return nil
		} else if errors.Is(err, context.Canceled) {
			return context.Canceled
		} else {
			failures = append(failures, fmt.Sprintf("%s: %s", source.Name, err.Error()))
		}
	}
	return errors.New(strings.Join(uniqueStrings(failures), "；"))
}

func rustupHostTriple() (string, error) {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "darwin/amd64":
		return "x86_64-apple-darwin", nil
	case "darwin/arm64":
		return "aarch64-apple-darwin", nil
	case "linux/amd64":
		return "x86_64-unknown-linux-gnu", nil
	case "linux/arm64":
		return "aarch64-unknown-linux-gnu", nil
	case "windows/amd64":
		return "x86_64-pc-windows-msvc", nil
	case "windows/arm64":
		return "aarch64-pc-windows-msvc", nil
	default:
		return "", fmt.Errorf("不支持的宿主平台: %s/%s", runtime.GOOS, runtime.GOARCH)
	}
}

func rustupInitFileExtension() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

func rustupInitExecutableName() string {
	return "rustup-init" + rustupInitFileExtension()
}

type zigResolvedAsset struct {
	version  string
	filename string
	url      string
	shasum   string
}

func resolveZigAsset(version string) (zigResolvedAsset, error) {
	index, err := fetchZigIndex()
	if err != nil {
		return zigResolvedAsset{}, err
	}
	raw, ok := index[version]
	if !ok {
		return zigResolvedAsset{}, fmt.Errorf("未找到 Zig 版本: %s", version)
	}
	release, ok := raw.(map[string]any)
	if !ok {
		return zigResolvedAsset{}, fmt.Errorf("解析 Zig 版本信息失败")
	}
	hostKey, err := zigHostKey()
	if err != nil {
		return zigResolvedAsset{}, err
	}
	rawAsset, ok := release[hostKey]
	if !ok {
		return zigResolvedAsset{}, fmt.Errorf("当前宿主平台缺少 Zig %s 安装包", version)
	}
	assetMap, ok := rawAsset.(map[string]any)
	if !ok {
		return zigResolvedAsset{}, fmt.Errorf("解析 Zig 安装包信息失败")
	}
	url, _ := assetMap["tarball"].(string)
	shasum, _ := assetMap["shasum"].(string)
	if strings.TrimSpace(url) == "" || strings.TrimSpace(shasum) == "" {
		return zigResolvedAsset{}, fmt.Errorf("Zig 安装包信息不完整")
	}
	return zigResolvedAsset{
		version:  version,
		filename: filepath.Base(strings.TrimSpace(url)),
		url:      strings.TrimSpace(url),
		shasum:   strings.TrimSpace(shasum),
	}, nil
}

func fetchZigIndex() (map[string]any, error) {
	req, err := http.NewRequest(http.MethodGet, zigIndexURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建 Zig 版本索引请求失败: %w", err)
	}
	resp, err := (&http.Client{Timeout: defaultNetworkTimeout}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Zig 版本索引失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("请求 Zig 版本索引失败: %s", resp.Status)
	}
	var index map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&index); err != nil {
		return nil, fmt.Errorf("解析 Zig 版本索引失败: %w", err)
	}
	return index, nil
}

func zigHostKey() (string, error) {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "darwin/amd64":
		return "x86_64-macos", nil
	case "darwin/arm64":
		return "aarch64-macos", nil
	case "linux/amd64":
		return "x86_64-linux", nil
	case "linux/arm64":
		return "aarch64-linux", nil
	case "windows/amd64":
		return "x86_64-windows", nil
	case "windows/arm64":
		return "aarch64-windows", nil
	default:
		return "", fmt.Errorf("当前宿主平台暂不支持自动安装 Zig")
	}
}

func zigReleaseSupportsHost(release map[string]any) bool {
	hostKey, err := zigHostKey()
	if err != nil {
		return false
	}
	_, ok := release[hostKey]
	return ok
}

func zigDownloadSources(officialURL string) ([]namedDownloadSource, error) {
	filename := filepath.Base(strings.TrimSpace(officialURL))
	if filename == "." || filename == "" {
		return nil, fmt.Errorf("无效的 Zig 下载地址")
	}
	req, err := http.NewRequest(http.MethodGet, zigCommunityMirrorsURL, nil)
	if err != nil {
		return []namedDownloadSource{{Name: "ziglang.org", URL: officialURL}}, nil
	}
	resp, err := (&http.Client{Timeout: defaultNetworkTimeout}).Do(req)
	if err != nil {
		return []namedDownloadSource{{Name: "ziglang.org", URL: officialURL}}, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return []namedDownloadSource{{Name: "ziglang.org", URL: officialURL}}, nil
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return []namedDownloadSource{{Name: "ziglang.org", URL: officialURL}}, nil
	}
	lines := strings.Split(string(body), "\n")
	result := make([]namedDownloadSource, 0, len(lines)+1)
	seen := make(map[string]struct{}, len(lines)+1)
	for _, line := range lines {
		baseURL := strings.TrimSpace(line)
		if baseURL == "" || strings.HasPrefix(baseURL, "#") {
			continue
		}
		url := strings.TrimRight(baseURL, "/") + "/" + filename
		if _, ok := seen[url]; ok {
			continue
		}
		seen[url] = struct{}{}
		result = append(result, namedDownloadSource{Name: strings.TrimPrefix(strings.TrimPrefix(baseURL, "https://"), "http://"), URL: url})
	}
	if _, ok := seen[officialURL]; !ok {
		result = append(result, namedDownloadSource{Name: "ziglang.org", URL: officialURL})
	}
	if len(result) == 0 {
		result = append(result, namedDownloadSource{Name: "ziglang.org", URL: officialURL})
	}
	return result, nil
}

func verifySHA256(filePath string, want string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return err
	}
	got := hex.EncodeToString(digest.Sum(nil))
	if !strings.EqualFold(strings.TrimSpace(got), strings.TrimSpace(want)) {
		return fmt.Errorf("SHA256 不匹配，期望 %s，实际 %s", want, got)
	}
	return nil
}

func runCommandWithLiveOutput(
	ctx context.Context,
	cmd *exec.Cmd,
	onLine func(line string),
	onTick func(elapsed time.Duration, lastLine string),
) (string, error) {
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}

	var (
		linesMu  sync.Mutex
		lines    []string
		lastLine string
	)
	appendLine := func(line string) {
		line = strings.TrimSpace(line)
		if line == "" {
			return
		}
		linesMu.Lock()
		lines = append(lines, line)
		lastLine = line
		linesMu.Unlock()
		if onLine != nil {
			onLine(line)
		}
	}
	streamPipe := func(reader io.ReadCloser, wg *sync.WaitGroup) {
		defer wg.Done()
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			appendLine(scanner.Text())
		}
		if err := scanner.Err(); err != nil && !errors.Is(err, os.ErrClosed) {
			appendLine(err.Error())
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go streamPipe(stdoutPipe, &wg)
	go streamPipe(stderrPipe, &wg)

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	startedAt := time.Now()
	for {
		select {
		case err := <-waitCh:
			wg.Wait()
			linesMu.Lock()
			output := strings.Join(lines, "\n")
			linesMu.Unlock()
			if err != nil {
				if errors.Is(ctx.Err(), context.Canceled) {
					return output, context.Canceled
				}
				return output, err
			}
			return output, nil
		case <-ticker.C:
			if onTick == nil {
				continue
			}
			linesMu.Lock()
			line := lastLine
			linesMu.Unlock()
			onTick(time.Since(startedAt), line)
		case <-ctx.Done():
			err := <-waitCh
			wg.Wait()
			linesMu.Lock()
			output := strings.Join(lines, "\n")
			linesMu.Unlock()
			if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
				return output, context.Canceled
			}
			if err != nil {
				return output, err
			}
			return output, context.Canceled
		}
	}
}

func summarizeCommandProgressLine(line string) string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "info: ")
	line = strings.TrimPrefix(line, "error: ")
	if line == "" {
		return ""
	}
	if len(line) > 160 {
		return line[:157] + "..."
	}
	return line
}

func formatElapsedDuration(elapsed time.Duration) string {
	if elapsed < time.Minute {
		seconds := int(elapsed.Round(time.Second) / time.Second)
		if seconds < 1 {
			seconds = 1
		}
		return fmt.Sprintf("%ds", seconds)
	}
	minutes := int(elapsed / time.Minute)
	seconds := int((elapsed % time.Minute).Round(time.Second) / time.Second)
	if seconds >= 60 {
		minutes += 1
		seconds = 0
	}
	return fmt.Sprintf("%dm%02ds", minutes, seconds)
}

func extractTarXz(ctx context.Context, archivePath string, targetDir string) error {
	args := []string{"-xJf", archivePath, "-C", targetDir, "--strip-components", "1"}
	cmd := exec.CommandContext(ctx, "tar", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return context.Canceled
		}
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("%s", detail)
	}
	return nil
}

func prependPathEntries(current string, entries ...string) string {
	values := make([]string, 0, len(entries)+4)
	seen := map[string]struct{}{}
	add := func(value string) {
		value = filepath.Clean(strings.TrimSpace(value))
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	for _, entry := range entries {
		add(entry)
	}
	for _, entry := range filepath.SplitList(current) {
		add(entry)
	}
	return strings.Join(values, string(os.PathListSeparator))
}

func emitRustInstallProgress(hooks *RustInstallHooks, progress RustInstallProgress) {
	if hooks == nil || hooks.OnProgress == nil {
		return
	}
	hooks.OnProgress(progress)
}

func compareSemverDesc(left string, right string) int {
	leftParts := parseSemverParts(left)
	rightParts := parseSemverParts(right)
	limit := minInt(len(leftParts), len(rightParts))
	for i := 0; i < limit; i++ {
		if leftParts[i] == rightParts[i] {
			continue
		}
		if leftParts[i] > rightParts[i] {
			return -1
		}
		return 1
	}
	switch {
	case len(leftParts) > len(rightParts):
		return -1
	case len(leftParts) < len(rightParts):
		return 1
	default:
		return strings.Compare(right, left)
	}
}

func parseSemverParts(version string) []int {
	fields := strings.Split(strings.TrimSpace(version), ".")
	result := make([]int, 0, len(fields))
	for _, field := range fields {
		value, err := strconv.Atoi(field)
		if err != nil {
			result = append(result, 0)
			continue
		}
		result = append(result, value)
	}
	return result
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

func filterKnownRustPaths(paths []string, removedDir string) []string {
	if strings.TrimSpace(removedDir) == "" {
		return dedupeStrings(paths)
	}
	filtered := make([]string, 0, len(paths))
	for _, path := range paths {
		if isPathWithinDir(path, removedDir) {
			continue
		}
		filtered = append(filtered, path)
	}
	return dedupeStrings(filtered)
}

func isPathWithinDir(path string, dir string) bool {
	path = filepath.Clean(strings.TrimSpace(path))
	dir = filepath.Clean(strings.TrimSpace(dir))
	if path == "" || dir == "" {
		return false
	}
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != "." && !strings.HasPrefix(rel, "..")
}
