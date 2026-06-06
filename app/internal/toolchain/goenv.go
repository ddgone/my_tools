package toolchain

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

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
	Config                    Config      `json:"config"`
	Candidates                []Candidate `json:"candidates"`
	HasUsableBinary           bool        `json:"hasUsableBinary"`
	ActiveBinary              string      `json:"activeBinary"`
	ActiveVersion             string      `json:"activeVersion"`
	ActiveSource              string      `json:"activeSource"`
	StatusMessage             string      `json:"statusMessage"`
	SuggestedInstallDirectory string      `json:"suggestedInstallDirectory"`
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

	return State{
		Config:                    cfg,
		Candidates:                candidates,
		HasUsableBinary:           activePath != "",
		ActiveBinary:              activePath,
		ActiveVersion:             activeVersion,
		ActiveSource:              activeSource,
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
		return InstallResult{}, err
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
		return InstallResult{}, fmt.Errorf("未找到可下载的 Go 版本: %s", version)
	}
	file, ok := selectArchiveForCurrentPlatform(matched)
	if !ok {
		return InstallResult{}, fmt.Errorf("当前平台暂不支持自动安装 %s", version)
	}
	if err := ensureInstallDirectoryReady(directory); err != nil {
		return InstallResult{}, err
	}

	tempFile, err := os.CreateTemp("", "fire-salamander-go-sdk-*"+filepath.Ext(file.Filename))
	if err != nil {
		return InstallResult{}, fmt.Errorf("创建下载缓存失败: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)
	defer tempFile.Close()

	if err := downloadToFile("https://go.dev/dl/"+file.Filename, tempFile); err != nil {
		return InstallResult{}, err
	}
	if err := tempFile.Close(); err != nil {
		return InstallResult{}, fmt.Errorf("关闭下载缓存失败: %w", err)
	}

	if strings.HasSuffix(file.Filename, ".zip") {
		if err := extractZip(tempPath, directory); err != nil {
			return InstallResult{}, err
		}
	} else if strings.HasSuffix(file.Filename, ".tar.gz") {
		if err := extractTarGz(tempPath, directory); err != nil {
			return InstallResult{}, err
		}
	} else {
		return InstallResult{}, fmt.Errorf("暂不支持自动解压 %s", file.Filename)
	}

	binaryPath := filepath.Join(directory, "bin", goExecutableName())
	if _, err := os.Stat(binaryPath); err != nil {
		return InstallResult{}, fmt.Errorf("安装完成后未找到 Go 可执行文件: %w", err)
	}
	return InstallResult{
		Version:    matched.Version,
		Directory:  directory,
		BinaryPath: binaryPath,
	}, nil
}

type candidatePath struct {
	path   string
	source string
}

func collectCandidatePaths(cfg Config) []candidatePath {
	seen := map[string]struct{}{}
	result := make([]candidatePath, 0, 16)
	add := func(path string, source string) {
		path = filepath.Clean(strings.TrimSpace(path))
		if path == "" {
			return
		}
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
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
	return left == right
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = filepath.Clean(strings.TrimSpace(value))
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
		return fmt.Errorf("安装位置不是空目录，请选择新的目录")
	}
	return nil
}

func downloadToFile(url string, file *os.File) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("下载 Go SDK 失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载 Go SDK 失败: %s", resp.Status)
	}
	if _, err := io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("写入 Go SDK 失败: %w", err)
	}
	return nil
}

func extractTarGz(archivePath string, targetDir string) error {
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
			if _, err := io.Copy(out, tarReader); err != nil {
				out.Close()
				return fmt.Errorf("解压文件失败: %w", err)
			}
			if err := out.Close(); err != nil {
				return fmt.Errorf("关闭文件失败: %w", err)
			}
		}
	}
	return nil
}

func extractZip(archivePath string, targetDir string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("打开压缩包失败: %w", err)
	}
	defer reader.Close()

	for _, file := range reader.File {
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
		if _, err := io.Copy(output, input); err != nil {
			input.Close()
			output.Close()
			return fmt.Errorf("解压文件失败: %w", err)
		}
		input.Close()
		if err := output.Close(); err != nil {
			return fmt.Errorf("关闭文件失败: %w", err)
		}
	}
	return nil
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
