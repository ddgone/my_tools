package toolchain

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
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
	"strings"
	"time"

	"fire-salamander-desktop/internal/appconfig"
	"fire-salamander-desktop/internal/runtimeenv"
)

const (
	configKeyGo        = "go"
	sourceConfigured   = "configured"
	sourceRemembered   = "remembered"
	sourcePath         = "path"
	sourceDetected     = "detected"
	sourceManaged      = "managed"
	officialReleasesEP = "https://go.dev/dl/?mode=json&include=all"
)

var goVersionDirectoryPattern = regexp.MustCompile(`(?i)^go\d+(?:\.\d+)+(?:[-._a-z0-9]+)?$`)

type Config struct {
	SelectedBinary       string   `json:"selectedBinary"`
	KnownBinaries        []string `json:"knownBinaries"`
	LastInstallDirectory string   `json:"lastInstallDirectory"`
	Disabled             bool     `json:"disabled"`
}

type Candidate struct {
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

type State struct {
	Config                    Config         `json:"config"`
	Candidates                []Candidate    `json:"candidates"`
	HasUsableBinary           bool           `json:"hasUsableBinary"`
	ActiveBinary              string         `json:"activeBinary"`
	ActiveVersion             string         `json:"activeVersion"`
	ActiveSource              string         `json:"activeSource"`
	RuntimeDetails            RuntimeDetails `json:"runtimeDetails"`
	StatusMessage             string         `json:"statusMessage"`
	SuggestedInstallDirectory string         `json:"suggestedInstallDirectory"`
}

type RuntimeDetails struct {
	GOROOT    string `json:"goroot"`
	GOPATH    string `json:"gopath"`
	GOOS      string `json:"goos"`
	GOARCH    string `json:"goarch"`
	GOVERSION string `json:"goversion"`
}

type GoInstallProgress struct {
	Message          string  `json:"message"`
	Detail           string  `json:"detail,omitempty"`
	CurrentItem      string  `json:"currentItem,omitempty"`
	ProgressPercent  float64 `json:"progressPercent"`
	Step             int     `json:"step"`
	TotalSteps       int     `json:"totalSteps"`
	Version          string  `json:"version,omitempty"`
	Directory        string  `json:"directory,omitempty"`
	TransferredBytes int64   `json:"transferredBytes,omitempty"`
	TotalBytes       int64   `json:"totalBytes,omitempty"`
	TransferSpeed    string  `json:"transferSpeed,omitempty"`
}

type GoInstallHooks struct {
	OnProgress func(progress GoInstallProgress)
}

type Release struct {
	Version string `json:"version"`
	Stable  bool   `json:"stable"`
}

type InstallResult struct {
	Version    string `json:"version"`
	Directory  string `json:"directory"`
	BinaryPath string `json:"binaryPath"`
}

type officialRelease struct {
	Version string         `json:"version"`
	Stable  bool           `json:"stable"`
	Files   []officialFile `json:"files"`
}

type officialFile struct {
	Filename string `json:"filename"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Kind     string `json:"kind"`
}

type goInstallError struct {
	Message string
	Detail  string
	Err     error
}

func (e *goInstallError) Error() string {
	if text := strings.TrimSpace(e.Detail); text != "" {
		return text
	}
	if text := strings.TrimSpace(e.Message); text != "" {
		return text
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "Go SDK 安装失败"
}

func (e *goInstallError) Unwrap() error {
	return e.Err
}

func wrapGoInstallError(message string, err error) error {
	if err == nil {
		return &goInstallError{
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
	return &goInstallError{
		Message: strings.TrimSpace(message),
		Detail:  detail,
		Err:     err,
	}
}

func DescribeGoInstallError(err error) (string, string) {
	if err == nil {
		return "", ""
	}
	if errors.Is(err, context.Canceled) {
		return "Go SDK 下载任务已停止", ""
	}
	var installErr *goInstallError
	if errors.As(err, &installErr) {
		message := strings.TrimSpace(installErr.Message)
		detail := strings.TrimSpace(installErr.Detail)
		if message == "" {
			message = "Go SDK 下载任务失败"
		}
		if detail == message {
			detail = ""
		}
		return message, detail
	}
	detail := strings.TrimSpace(err.Error())
	if detail == "" {
		detail = "Go SDK 下载任务失败"
	}
	return "Go SDK 下载任务失败", detail
}

func LoadConfig() (Config, error) {
	configPath, err := appconfig.ResolveConfigPath()
	if err != nil {
		return Config{}, err
	}
	doc, err := appconfig.LoadDocument(configPath)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if raw, ok := doc[configKeyGo]; ok && len(raw) > 0 {
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return Config{}, fmt.Errorf("解析 Go 配置失败: %w", err)
		}
	}
	return normalizeConfig(cfg), nil
}

func SaveConfig(cfg Config) error {
	configPath, err := appconfig.ResolveConfigPath()
	if err != nil {
		return err
	}
	doc, err := appconfig.LoadDocument(configPath)
	if err != nil {
		return err
	}
	cfg = normalizeConfig(cfg)
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("序列化 Go 配置失败: %w", err)
	}
	doc[configKeyGo] = data
	return appconfig.WriteDocument(configPath, doc)
}

func GetState() (State, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return State{}, err
	}
	return InspectConfig(cfg)
}

func InspectConfig(cfg Config) (State, error) {
	cfg = normalizeConfig(cfg)
	suggestedInstallDir, _ := defaultInstallDirectory()
	paths := collectCandidatePaths(cfg)
	candidates := make([]Candidate, 0, len(paths))

	activePath := ""
	activeVersion := ""
	activeSource := ""
	selectedError := ""

	for _, entry := range paths {
		candidate := inspectCandidate(entry.path, entry.source, cfg.SelectedBinary)
		if candidate.Selected && !candidate.Valid && candidate.Error != "" {
			selectedError = candidate.Error
		}
		if !cfg.Disabled && activePath == "" && candidate.Valid {
			activePath = candidate.Path
			activeVersion = candidate.Version
			activeSource = candidate.Source
			candidate.Active = true
		}
		candidates = append(candidates, candidate)
	}

	statusMessage := ""
	if cfg.Disabled {
		statusMessage = "当前未启用 Go 环境，可重新选择本地 Go 或下载 SDK"
	} else if activePath == "" {
		statusMessage = "未检测到可用的 Go 环境，请先选择本地 Go 或下载 SDK"
	} else if selectedError != "" {
		statusMessage = fmt.Sprintf("已回退到自动检测的 Go 环境；之前配置的路径不可用：%s", selectedError)
	}

	runtimeDetails := RuntimeDetails{}
	if activePath != "" {
		runtimeDetails, _ = readGoRuntimeDetails(activePath)
	}

	return State{
		Config:                    cfg,
		Candidates:                candidates,
		HasUsableBinary:           activePath != "",
		ActiveBinary:              activePath,
		ActiveVersion:             activeVersion,
		ActiveSource:              activeSource,
		RuntimeDetails:            runtimeDetails,
		StatusMessage:             statusMessage,
		SuggestedInstallDirectory: suggestedInstallDir,
	}, nil
}

func ResolveGoBinary() (string, error) {
	state, err := GetState()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(state.ActiveBinary) != "" {
		return state.ActiveBinary, nil
	}
	if strings.TrimSpace(state.StatusMessage) != "" {
		return "", fmt.Errorf("%s", state.StatusMessage)
	}
	return "", fmt.Errorf("未检测到可用的 Go 环境，请先在系统设置 > Go 中选择本地 Go 或下载 SDK")
}

func ListOfficialReleases() ([]Release, error) {
	releases, err := fetchOfficialReleases()
	if err != nil {
		return nil, err
	}
	result := make([]Release, 0, 16)
	for _, release := range releases {
		if !release.Stable {
			continue
		}
		if _, ok := selectArchiveForCurrentPlatform(release); !ok {
			continue
		}
		result = append(result, Release{
			Version: release.Version,
			Stable:  release.Stable,
		})
		if len(result) >= 20 {
			break
		}
	}
	return result, nil
}

func InstallOfficialRelease(version string, directory string) (InstallResult, error) {
	return InstallOfficialReleaseWithOptions(context.Background(), version, directory, nil)
}

func InstallOfficialReleaseWithOptions(ctx context.Context, version string, directory string, hooks *GoInstallHooks) (InstallResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	version = strings.TrimSpace(version)
	directory = filepath.Clean(strings.TrimSpace(directory))
	if version == "" {
		return InstallResult{}, fmt.Errorf("Go 版本不能为空")
	}
	if directory == "" || directory == "." {
		return InstallResult{}, fmt.Errorf("安装位置不能为空")
	}

	releases, err := fetchOfficialReleases()
	if err != nil {
		return InstallResult{}, wrapGoInstallError("获取 Go 版本列表失败", err)
	}
	var matched officialRelease
	found := false
	for _, release := range releases {
		if strings.EqualFold(strings.TrimSpace(release.Version), version) {
			matched = release
			found = true
			break
		}
	}
	if !found {
		return InstallResult{}, wrapGoInstallError("未找到可下载的 Go 版本", fmt.Errorf("未找到可下载的 Go 版本: %s", version))
	}
	file, ok := selectArchiveForCurrentPlatform(matched)
	if !ok {
		return InstallResult{}, wrapGoInstallError("当前平台缺少对应的 Go SDK 安装包", fmt.Errorf("当前平台暂不支持自动安装 %s", version))
	}
	targetDirectory := resolveInstallTargetDirectory(version, directory)
	emitGoInstallProgress(hooks, GoInstallProgress{
		Message:         "准备下载安装 Go SDK",
		CurrentItem:     version,
		ProgressPercent: 0,
		Step:            1,
		TotalSteps:      4,
		Version:         version,
		Directory:       targetDirectory,
	})
	if err := ensureInstallDirectoryReady(targetDirectory); err != nil {
		return InstallResult{}, wrapGoInstallError("安装目录不可用", err)
	}
	if existingBinary, ok := resolveInstalledGoBinary(targetDirectory); ok {
		emitGoInstallProgress(hooks, GoInstallProgress{
			Message:         "已复用现有 Go SDK",
			Detail:          targetDirectory,
			CurrentItem:     version,
			ProgressPercent: 100,
			Step:            4,
			TotalSteps:      4,
			Version:         version,
			Directory:       targetDirectory,
		})
		return InstallResult{
			Version:    matched.Version,
			Directory:  targetDirectory,
			BinaryPath: existingBinary,
		}, nil
	}

	tempFile, err := os.CreateTemp("", "fire-salamander-go-sdk-*"+filepath.Ext(file.Filename))
	if err != nil {
		return InstallResult{}, wrapGoInstallError("创建下载缓存失败", fmt.Errorf("创建下载缓存失败: %w", err))
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)
	defer tempFile.Close()

	defer func() {
		if ctx.Err() != nil {
			_ = os.RemoveAll(targetDirectory)
		}
	}()

	emitGoInstallProgress(hooks, GoInstallProgress{
		Message:         "正在下载 Go SDK",
		CurrentItem:     file.Filename,
		ProgressPercent: 8,
		Step:            2,
		TotalSteps:      4,
		Version:         version,
		Directory:       targetDirectory,
	})
	if err := downloadToFile(ctx, "https://go.dev/dl/"+file.Filename, tempFile, func(downloaded int64, total int64, speed string) {
		progress := 45.0
		if total > 0 {
			progress = 8 + float64(downloaded)*57/float64(total)
		}
		emitGoInstallProgress(hooks, GoInstallProgress{
			Message:          "正在下载 Go SDK",
			CurrentItem:      file.Filename,
			ProgressPercent:  roundProgressPercent(progress),
			Step:             2,
			TotalSteps:       4,
			Version:          version,
			Directory:        targetDirectory,
			TransferredBytes: downloaded,
			TotalBytes:       total,
			TransferSpeed:    speed,
		})
	}); err != nil {
		return InstallResult{}, wrapGoInstallError("下载 Go SDK 失败", err)
	}
	if err := tempFile.Close(); err != nil {
		return InstallResult{}, wrapGoInstallError("写入下载缓存失败", fmt.Errorf("关闭下载缓存失败: %w", err))
	}

	emitGoInstallProgress(hooks, GoInstallProgress{
		Message:         "正在解压 Go SDK",
		Detail:          targetDirectory,
		CurrentItem:     matched.Version,
		ProgressPercent: 70,
		Step:            3,
		TotalSteps:      4,
		Version:         version,
		Directory:       targetDirectory,
	})
	if strings.HasSuffix(file.Filename, ".zip") {
		if err := extractZip(ctx, tempPath, targetDirectory); err != nil {
			return InstallResult{}, wrapGoInstallError("解压 Go SDK 失败", err)
		}
	} else if strings.HasSuffix(file.Filename, ".tar.gz") {
		if err := extractTarGz(ctx, tempPath, targetDirectory); err != nil {
			return InstallResult{}, wrapGoInstallError("解压 Go SDK 失败", err)
		}
	} else {
		return InstallResult{}, wrapGoInstallError("当前安装包格式暂不支持自动解压", fmt.Errorf("暂不支持自动解压 %s", file.Filename))
	}

	binaryPath := filepath.Join(targetDirectory, "bin", goExecutableName())
	if _, err := os.Stat(binaryPath); err != nil {
		return InstallResult{}, wrapGoInstallError("Go SDK 安装不完整", fmt.Errorf("安装完成后未找到 Go 可执行文件: %w", err))
	}
	emitGoInstallProgress(hooks, GoInstallProgress{
		Message:         "Go SDK 安装完成",
		Detail:          targetDirectory,
		CurrentItem:     matched.Version,
		ProgressPercent: 100,
		Step:            4,
		TotalSteps:      4,
		Version:         matched.Version,
		Directory:       targetDirectory,
	})
	return InstallResult{
		Version:    matched.Version,
		Directory:  targetDirectory,
		BinaryPath: binaryPath,
	}, nil
}

type candidatePath struct {
	path   string
	source string
}

func collectCandidatePaths(cfg Config) []candidatePath {
	seen := map[string]int{}
	result := make([]candidatePath, 0, 16)
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
		result = append(result, candidatePath{path: path, source: source})
	}

	if cfg.SelectedBinary != "" {
		add(cfg.SelectedBinary, sourceConfigured)
	}
	for _, path := range cfg.KnownBinaries {
		add(path, sourceRemembered)
	}
	if path, err := exec.LookPath(goExecutableName()); err == nil && path != "" {
		add(path, sourcePath)
	}
	for _, path := range candidateGoBinaryPaths() {
		add(path, sourceDetected)
	}
	for _, path := range discoverManagedBinaries() {
		add(path, sourceManaged)
	}
	return result
}

func inspectCandidate(path string, source string, selectedBinary string) Candidate {
	candidate := Candidate{
		Path:     path,
		Source:   source,
		Selected: sameFilePath(path, selectedBinary),
		Detail:   path,
	}
	version, err := readGoVersion(path)
	if err != nil {
		candidate.Label = filepath.Base(path)
		candidate.Error = err.Error()
		return candidate
	}
	candidate.Valid = true
	candidate.Version = version
	candidate.Label = version
	return candidate
}

func readGoVersion(binaryPath string) (string, error) {
	if _, err := os.Stat(binaryPath); err != nil {
		return "", fmt.Errorf("路径不存在")
	}
	output, err := exec.Command(binaryPath, "version").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("无法读取版本")
	}
	line := strings.TrimSpace(string(output))
	if line == "" {
		return "", fmt.Errorf("版本输出为空")
	}
	fields := strings.Fields(line)
	if len(fields) >= 3 {
		return strings.ToUpper(fields[2][:1]) + fields[2][1:], nil
	}
	return line, nil
}

func candidateGoBinaryPaths() []string {
	paths := make([]string, 0, 8)

	addGOROOTCandidate := func(root string) {
		root = strings.TrimSpace(root)
		if root == "" {
			return
		}
		paths = append(paths, filepath.Join(root, "bin", goExecutableName()))
	}

	addGOROOTCandidate(os.Getenv("GOROOT"))

	if runtime.GOOS == "windows" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			matches, _ := filepath.Glob(filepath.Join(home, "sdk", "go*", "bin", "go.exe"))
			sort.Strings(matches)
			for i := len(matches) - 1; i >= 0; i-- {
				paths = append(paths, matches[i])
			}
		}
		for _, envKey := range []string{"ProgramFiles", "ProgramFiles(x86)", "LocalAppData"} {
			base := strings.TrimSpace(os.Getenv(envKey))
			if base == "" {
				continue
			}
			switch envKey {
			case "LocalAppData":
				paths = append(paths, filepath.Join(base, "Programs", "Go", "bin", "go.exe"))
			default:
				paths = append(paths, filepath.Join(base, "Go", "bin", "go.exe"))
			}
		}
		return dedupeStrings(paths)
	}

	paths = append(paths, "/usr/local/go/bin/go", "/opt/homebrew/bin/go", "/usr/bin/go")
	return dedupeStrings(paths)
}

func discoverManagedBinaries() []string {
	layout, err := runtimeenv.ResolveLayout()
	if err != nil {
		return nil
	}
	root := filepath.Join(layout.Root, "toolchains")
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		binaryPath := filepath.Join(root, entry.Name(), "bin", goExecutableName())
		if _, err := os.Stat(binaryPath); err == nil {
			result = append(result, binaryPath)
		}
	}
	sort.Strings(result)
	return result
}

func defaultInstallDirectory() (string, error) {
	layout, err := runtimeenv.ResolveLayout()
	if err != nil {
		return "", err
	}
	return filepath.Join(layout.Root, "toolchains"), nil
}

func normalizeConfig(cfg Config) Config {
	cfg.SelectedBinary = strings.TrimSpace(cfg.SelectedBinary)
	cfg.LastInstallDirectory = strings.TrimSpace(cfg.LastInstallDirectory)
	cfg.KnownBinaries = dedupeStrings(cfg.KnownBinaries)
	if cfg.SelectedBinary != "" {
		cfg.KnownBinaries = dedupeStrings(append([]string{cfg.SelectedBinary}, cfg.KnownBinaries...))
	}
	return cfg
}

func sameFilePath(left string, right string) bool {
	left = filepath.Clean(strings.TrimSpace(left))
	right = filepath.Clean(strings.TrimSpace(right))
	if left == "" || right == "" {
		return false
	}
	return fileIdentity(left) == fileIdentity(right)
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = filepath.Clean(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		identity := fileIdentity(value)
		if _, ok := seen[identity]; ok {
			continue
		}
		seen[identity] = struct{}{}
		result = append(result, value)
	}
	return result
}

func fileIdentity(path string) string {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" {
		return ""
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	resolved = filepath.Clean(strings.TrimSpace(resolved))
	if resolved == "" {
		return path
	}
	return resolved
}

func fetchOfficialReleases() ([]officialRelease, error) {
	resp, err := http.Get(officialReleasesEP)
	if err != nil {
		return nil, fmt.Errorf("获取 Go 版本列表失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("获取 Go 版本列表失败: %s", resp.Status)
	}
	var releases []officialRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("解析 Go 版本列表失败: %w", err)
	}
	return releases, nil
}

func selectArchiveForCurrentPlatform(release officialRelease) (officialFile, bool) {
	wantExt := ".tar.gz"
	if runtime.GOOS == "windows" {
		wantExt = ".zip"
	}
	for _, file := range release.Files {
		if file.OS != runtime.GOOS || file.Arch != runtime.GOARCH || file.Kind != "archive" {
			continue
		}
		if strings.HasSuffix(file.Filename, wantExt) {
			return file, true
		}
	}
	return officialFile{}, false
}

func ensureInstallDirectoryReady(directory string) error {
	info, err := os.Stat(directory)
	if err != nil {
		if os.IsNotExist(err) {
			return os.MkdirAll(directory, 0755)
		}
		return fmt.Errorf("检查安装目录失败: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("安装位置不是目录: %s", directory)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("读取安装目录失败: %w", err)
	}
	if len(entries) > 0 {
		binaryPath := filepath.Join(directory, "bin", goExecutableName())
		if version, versionErr := readGoVersion(binaryPath); versionErr == nil && version != "" {
			return nil
		}
		return fmt.Errorf("目标版本目录已存在且内容不可复用，请选择新的位置")
	}
	return nil
}

func resolveInstalledGoBinary(directory string) (string, bool) {
	binaryPath := filepath.Join(directory, "bin", goExecutableName())
	version, err := readGoVersion(binaryPath)
	return binaryPath, err == nil && strings.TrimSpace(version) != ""
}

func resolveInstallTargetDirectory(version string, directory string) string {
	version = strings.ToLower(strings.TrimSpace(version))
	baseDirectory := NormalizeInstallBaseDirectory(directory)
	if baseDirectory == "" {
		return ""
	}
	if version != "" && strings.EqualFold(filepath.Base(baseDirectory), version) {
		return baseDirectory
	}
	if version == "" {
		return baseDirectory
	}
	return filepath.Join(baseDirectory, version)
}

func NormalizeInstallBaseDirectory(directory string) string {
	directory = filepath.Clean(strings.TrimSpace(directory))
	if directory == "" || directory == "." {
		return ""
	}
	base := filepath.Base(directory)
	if goVersionDirectoryPattern.MatchString(base) {
		parent := filepath.Dir(directory)
		if parent != "" && parent != "." {
			return parent
		}
	}
	return directory
}

func preferCandidateSource(current string, incoming string) string {
	sourceRank := map[string]int{
		sourceConfigured: 0,
		sourceRemembered: 1,
		sourceDetected:   2,
		sourcePath:       3,
		sourceManaged:    4,
	}
	if sourceRank[incoming] > sourceRank[current] {
		return incoming
	}
	return current
}

func readGoRuntimeDetails(binaryPath string) (RuntimeDetails, error) {
	output, err := exec.Command(binaryPath, "env", "GOROOT", "GOPATH", "GOOS", "GOARCH", "GOVERSION").CombinedOutput()
	if err != nil {
		return RuntimeDetails{}, fmt.Errorf("读取 Go 环境详情失败: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	values := make([]string, 0, len(lines))
	for _, line := range lines {
		values = append(values, strings.Trim(strings.TrimSpace(line), `"`))
	}
	if len(values) < 5 {
		return RuntimeDetails{}, fmt.Errorf("Go 环境详情输出不完整")
	}
	return RuntimeDetails{
		GOROOT:    values[0],
		GOPATH:    values[1],
		GOOS:      values[2],
		GOARCH:    values[3],
		GOVERSION: values[4],
	}, nil
}

func DeleteManagedGoEnvironment() (State, error) {
	state, err := GetState()
	if err != nil {
		return State{}, err
	}
	activeBinary := strings.TrimSpace(state.ActiveBinary)
	if activeBinary == "" {
		return state, fmt.Errorf("当前没有可删除的 Go 环境")
	}
	managedRoot, err := defaultInstallDirectory()
	if err != nil {
		return state, err
	}
	targetDir, ok := resolveManagedGoDirectory(activeBinary, managedRoot)
	if !ok {
		return state, fmt.Errorf("当前 Go 环境不是托管 SDK，无法在这里删除")
	}
	if err := os.RemoveAll(targetDir); err != nil {
		return state, fmt.Errorf("删除当前 Go 环境失败: %w", err)
	}
	cfg, err := LoadConfig()
	if err != nil {
		return State{}, err
	}
	filtered := make([]string, 0, len(cfg.KnownBinaries))
	for _, known := range cfg.KnownBinaries {
		if sameFilePath(known, activeBinary) {
			continue
		}
		filtered = append(filtered, known)
	}
	cfg.KnownBinaries = filtered
	if sameFilePath(cfg.SelectedBinary, activeBinary) {
		cfg.SelectedBinary = ""
	}
	if sameFilePath(cfg.LastInstallDirectory, targetDir) {
		cfg.LastInstallDirectory = managedRoot
	}
	if err := SaveConfig(cfg); err != nil {
		return State{}, err
	}
	return GetState()
}

func resolveManagedGoDirectory(binaryPath string, managedRoot string) (string, bool) {
	binaryPath = filepath.Clean(strings.TrimSpace(binaryPath))
	managedRoot = filepath.Clean(strings.TrimSpace(managedRoot))
	if binaryPath == "" || managedRoot == "" {
		return "", false
	}
	envDir := filepath.Dir(filepath.Dir(binaryPath))
	rel, err := filepath.Rel(managedRoot, envDir)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return "", false
	}
	return envDir, true
}

func downloadToFile(ctx context.Context, url string, file *os.File, onProgress func(downloaded int64, total int64, speed string)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("下载 Go SDK 失败: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return context.Canceled
		}
		return fmt.Errorf("下载 Go SDK 失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载 Go SDK 失败: %s", resp.Status)
	}
	total := resp.ContentLength
	buffer := make([]byte, 128*1024)
	start := time.Now()
	var downloaded int64
	for {
		if ctx.Err() != nil {
			return context.Canceled
		}
		n, readErr := resp.Body.Read(buffer)
		if n > 0 {
			if _, err := file.Write(buffer[:n]); err != nil {
				return fmt.Errorf("写入 Go SDK 失败: %w", err)
			}
			downloaded += int64(n)
			if onProgress != nil {
				elapsed := time.Since(start)
				if elapsed <= 0 {
					elapsed = time.Millisecond
				}
				speed := float64(downloaded) / elapsed.Seconds()
				onProgress(downloaded, total, formatTransferRate(speed))
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			if errors.Is(readErr, context.Canceled) || ctx.Err() != nil {
				return context.Canceled
			}
			return fmt.Errorf("下载 Go SDK 失败: %w", readErr)
		}
	}
	return nil
}

func extractTarGz(ctx context.Context, archivePath string, targetDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("打开压缩包失败: %w", err)
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("读取压缩包失败: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		if ctx.Err() != nil {
			return context.Canceled
		}
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("解压 Go SDK 失败: %w", err)
		}
		relativePath, ok := stripArchiveRoot(header.Name)
		if !ok {
			continue
		}
		destPath := filepath.Join(targetDir, relativePath)
		if !strings.HasPrefix(destPath, filepath.Clean(targetDir)+string(os.PathSeparator)) && filepath.Clean(destPath) != filepath.Clean(targetDir) {
			return fmt.Errorf("压缩包内容非法: %s", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destPath, 0755); err != nil {
				return fmt.Errorf("创建目录失败: %w", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				return fmt.Errorf("创建目录失败: %w", err)
			}
			out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("写入文件失败: %w", err)
			}
			if _, err := io.Copy(out, &contextAwareReader{ctx: ctx, reader: tarReader}); err != nil {
				out.Close()
				if errors.Is(err, context.Canceled) {
					return context.Canceled
				}
				return fmt.Errorf("解压文件失败: %w", err)
			}
			if err := out.Close(); err != nil {
				return fmt.Errorf("关闭文件失败: %w", err)
			}
		}
	}
	return nil
}

func extractZip(ctx context.Context, archivePath string, targetDir string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("打开压缩包失败: %w", err)
	}
	defer reader.Close()

	for _, file := range reader.File {
		if ctx.Err() != nil {
			return context.Canceled
		}
		relativePath, ok := stripArchiveRoot(file.Name)
		if !ok {
			continue
		}
		destPath := filepath.Join(targetDir, relativePath)
		if !strings.HasPrefix(destPath, filepath.Clean(targetDir)+string(os.PathSeparator)) && filepath.Clean(destPath) != filepath.Clean(targetDir) {
			return fmt.Errorf("压缩包内容非法: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(destPath, 0755); err != nil {
				return fmt.Errorf("创建目录失败: %w", err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("创建目录失败: %w", err)
		}
		input, err := file.Open()
		if err != nil {
			return fmt.Errorf("读取压缩包内容失败: %w", err)
		}
		output, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, file.Mode())
		if err != nil {
			input.Close()
			return fmt.Errorf("写入文件失败: %w", err)
		}
		if _, err := io.Copy(output, &contextAwareReader{ctx: ctx, reader: input}); err != nil {
			input.Close()
			output.Close()
			if errors.Is(err, context.Canceled) {
				return context.Canceled
			}
			return fmt.Errorf("解压文件失败: %w", err)
		}
		input.Close()
		if err := output.Close(); err != nil {
			return fmt.Errorf("关闭文件失败: %w", err)
		}
	}
	return nil
}

type contextAwareReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextAwareReader) Read(p []byte) (int, error) {
	if r.ctx != nil && r.ctx.Err() != nil {
		return 0, context.Canceled
	}
	return r.reader.Read(p)
}

func emitGoInstallProgress(hooks *GoInstallHooks, progress GoInstallProgress) {
	if hooks == nil || hooks.OnProgress == nil {
		return
	}
	hooks.OnProgress(progress)
}

func formatByteCount(value int64) string {
	if value < 1024 {
		return fmt.Sprintf("%d B", value)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	size := float64(value)
	unitIndex := -1
	for size >= 1024 && unitIndex < len(units)-1 {
		size /= 1024
		unitIndex++
	}
	return fmt.Sprintf("%.1f %s", size, units[unitIndex])
}

func formatTransferRate(bytesPerSecond float64) string {
	if bytesPerSecond <= 0 {
		return "0 B/s"
	}
	return fmt.Sprintf("%s/s", formatByteCount(int64(bytesPerSecond)))
}

func stripArchiveRoot(name string) (string, bool) {
	cleaned := filepath.Clean(strings.TrimSpace(name))
	if cleaned == "." || cleaned == "" {
		return "", false
	}
	parts := strings.Split(cleaned, string(os.PathSeparator))
	if len(parts) <= 1 {
		return "", false
	}
	relative := filepath.Join(parts[1:]...)
	if relative == "." || relative == "" {
		return "", false
	}
	return relative, true
}

func goExecutableName() string {
	if runtime.GOOS == "windows" {
		return "go.exe"
	}
	return "go"
}
