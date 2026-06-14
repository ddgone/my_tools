package builder

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"fire-salamander-desktop/internal/toolchain"
	"my_tools/libs/core/procutil"
)

const (
	envCargoBinary         = "FIRE_SALAMANDER_CARGO_BIN"
	envRustupBinary        = "FIRE_SALAMANDER_RUSTUP_BIN"
	envZigBinary           = "FIRE_SALAMANDER_ZIG_BIN"
	envCargoZigbuildBinary = "FIRE_SALAMANDER_CARGO_ZIGBUILD_BIN"
)

func buildRustPackage(req BuildRequest) (BuildResult, error) {
	if req.SourceEntry == "" {
		return BuildResult{}, fmt.Errorf("工具 %s 缺少 Rust 源码入口", req.ToolID)
	}
	if req.RepoRoot == "" {
		return BuildResult{}, fmt.Errorf("工具 %s 缺少仓库根目录，无法生成 Rust 产物", req.ToolID)
	}

	targetOS := req.TargetOS
	if targetOS == "" {
		targetOS = runtime.GOOS
	}
	targetArch := req.TargetArch
	if targetArch == "" {
		targetArch = runtime.GOARCH
	}

	outputName := strings.TrimSpace(req.OutputName)
	if outputName == "" {
		outputName = req.ToolID + "_" + targetOS + "_" + targetArch
		if targetOS == "windows" {
			outputName += ".exe"
		}
	}
	outFile := filepath.Join(req.OutputDir, filepath.Base(outputName))

	sourcePath, err := resolveSourceEntryPath(req)
	if err != nil {
		return BuildResult{}, err
	}
	crateRoot, err := resolveRustCrateRoot(sourcePath)
	if err != nil {
		return BuildResult{}, err
	}
	targetTriple, nativeBuild, err := resolveRustBuildTarget(targetOS, targetArch)
	if err != nil {
		return BuildResult{}, err
	}
	cacheKey, err := computeRustCacheKey(req, crateRoot, targetOS, targetArch, targetTriple, nativeBuild)
	if err != nil {
		return BuildResult{}, err
	}
	cachePath, sourceCachePath, cacheKeyPath, err := resolveCachePaths(req, targetOS+"_"+targetArch, outputName, sourcePath)
	if err != nil {
		return BuildResult{}, err
	}
	if !req.ForceRebuild && cacheEntryMatches(cachePath, cacheKeyPath, cacheKey) {
		logBuildProgress(req, "命中构建缓存")
		return finalizeBuildOutput(req, outFile, cachePath, 0755, cacheKey, true)
	}

	cargoBinary, err := resolveCargoBinary()
	if err != nil {
		return BuildResult{}, err
	}

	targetDir, err := os.MkdirTemp("", req.ToolID+"_rust_target_")
	if err != nil {
		return BuildResult{}, fmt.Errorf("创建 Rust 构建目录失败: %w", err)
	}
	defer os.RemoveAll(targetDir)

	if !nativeBuild {
		rustupBinary, rustupErr := resolveRustupBinary()
		if rustupErr != nil {
			return BuildResult{}, rustupErr
		}
		logBuildProgress(req, "准备 Rust 交叉编译目标")
		targetCmd := procutil.Command(rustupBinary, "target", "add", targetTriple)
		targetCmd.Dir = crateRoot
		targetCmd.Env = os.Environ()
		output, targetErr := targetCmd.CombinedOutput()
		if targetErr != nil {
			return BuildResult{}, fmt.Errorf("安装 Rust 目标失败: %w\n%s", targetErr, strings.TrimSpace(string(output)))
		}
	}

	args := []string{"build", "--release"}
	progressLabel := "正在构建 Rust 工具产物"
	zigBinary := ""
	cargoZigbuildBinary := ""
	if !nativeBuild {
		zigBinary, err = resolveZigBinary()
		if err != nil {
			return BuildResult{}, err
		}
		cargoZigbuildBinary, err = resolveCargoZigbuildBinary()
		if err != nil {
			return BuildResult{}, err
		}
		args = []string{"zigbuild", "--release", "--target", targetTriple}
		progressLabel = "正在交叉编译 Rust 工具产物"
	}
	logBuildProgress(req, progressLabel)

	buildCmd := procutil.Command(cargoBinary, args...)
	buildCmd.Dir = crateRoot
	buildCmd.Env = rustCommandEnv(cargoBinary, zigBinary, cargoZigbuildBinary, targetDir)
	output, err := buildCmd.CombinedOutput()
	if err != nil {
		return BuildResult{}, fmt.Errorf("构建 Rust 产物失败: %w\n%s", err, strings.TrimSpace(string(output)))
	}

	builtBinaryPath := filepath.Join(targetDir, "release", rustBinaryBaseName(req.ToolID, targetOS))
	if !nativeBuild {
		builtBinaryPath = filepath.Join(targetDir, targetTriple, "release", rustBinaryBaseName(req.ToolID, targetOS))
	}
	if !fileExists(builtBinaryPath) {
		return BuildResult{}, fmt.Errorf("未找到 Rust 构建产物: %s", builtBinaryPath)
	}
	if err := copyFile(builtBinaryPath, cachePath, 0755); err != nil {
		return BuildResult{}, fmt.Errorf("写入 Rust 产物缓存失败: %w", err)
	}
	if err := copySourceSnapshot(sourcePath, sourceCachePath); err != nil {
		return BuildResult{}, err
	}
	if err := writeCacheKeyFile(cacheKeyPath, cacheKey); err != nil {
		return BuildResult{}, err
	}

	logBuildProgress(req, "构建完成，已写入缓存")
	return finalizeBuildOutput(req, outFile, cachePath, 0755, cacheKey, false)
}

func probeRustCache(req BuildRequest) (BuildResult, error) {
	if req.SourceEntry == "" {
		return BuildResult{}, fmt.Errorf("工具 %s 缺少 Rust 源码入口", req.ToolID)
	}
	if req.RepoRoot == "" {
		return BuildResult{}, fmt.Errorf("工具 %s 缺少仓库根目录，无法生成 Rust 产物", req.ToolID)
	}

	targetOS := req.TargetOS
	if targetOS == "" {
		targetOS = runtime.GOOS
	}
	targetArch := req.TargetArch
	if targetArch == "" {
		targetArch = runtime.GOARCH
	}

	outputName := strings.TrimSpace(req.OutputName)
	if outputName == "" {
		outputName = req.ToolID + "_" + targetOS + "_" + targetArch
		if targetOS == "windows" {
			outputName += ".exe"
		}
	}
	sourcePath, err := resolveSourceEntryPath(req)
	if err != nil {
		return BuildResult{}, err
	}
	crateRoot, err := resolveRustCrateRoot(sourcePath)
	if err != nil {
		return BuildResult{}, err
	}
	targetTriple, nativeBuild, err := resolveRustBuildTarget(targetOS, targetArch)
	if err != nil {
		return BuildResult{}, err
	}
	cacheKey, err := computeRustCacheKey(req, crateRoot, targetOS, targetArch, targetTriple, nativeBuild)
	if err != nil {
		return BuildResult{}, err
	}
	cachePath, _, cacheKeyPath, err := resolveCachePaths(req, targetOS+"_"+targetArch, outputName, sourcePath)
	if err != nil {
		return BuildResult{}, err
	}
	return BuildResult{
		Path:      cachePath,
		CachePath: cachePath,
		CacheKey:  cacheKey,
		CacheHit:  cacheEntryMatches(cachePath, cacheKeyPath, cacheKey),
	}, nil
}

func computeRustCacheKey(req BuildRequest, crateRoot string, targetOS string, targetArch string, targetTriple string, nativeBuild bool) (string, error) {
	digest := sha256.New()
	writeCacheToken(digest, "rust-cache-v1")
	writeCacheToken(digest, req.ToolID)
	writeCacheToken(digest, targetOS)
	writeCacheToken(digest, targetArch)
	writeCacheToken(digest, targetTriple)
	if nativeBuild {
		writeCacheToken(digest, "native")
	} else {
		writeCacheToken(digest, "cross")
	}

	files, err := collectRustRelevantFiles(crateRoot)
	if err != nil {
		return "", fmt.Errorf("扫描 Rust 构建输入失败: %w", err)
	}
	for _, relPath := range files {
		if err := hashSingleFile(digest, filepath.Join(crateRoot, filepath.FromSlash(relPath)), relPath); err != nil {
			return "", fmt.Errorf("计算 Rust 产物缓存失败: %w", err)
		}
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func collectRustRelevantFiles(crateRoot string) ([]string, error) {
	files := make([]string, 0, 16)
	err := filepath.WalkDir(crateRoot, func(currentPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relPath, err := filepath.Rel(crateRoot, currentPath)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)
		if entry.IsDir() {
			switch relPath {
			case ".", "":
				return nil
			case "target", ".git":
				return filepath.SkipDir
			default:
				return nil
			}
		}
		if isRustCacheInput(relPath) {
			files = append(files, relPath)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func isRustCacheInput(relPath string) bool {
	switch filepath.Base(relPath) {
	case "Cargo.toml", "Cargo.lock":
		return true
	}
	if strings.HasPrefix(relPath, ".cargo/") {
		return strings.EqualFold(filepath.Base(relPath), "config.toml")
	}
	return strings.EqualFold(filepath.Ext(relPath), ".rs")
}

func resolveRustCrateRoot(sourcePath string) (string, error) {
	dir := filepath.Dir(sourcePath)
	for {
		cargoToml := filepath.Join(dir, "Cargo.toml")
		if fileExists(cargoToml) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("未找到 Rust crate 根目录: %s", sourcePath)
		}
		dir = parent
	}
}

func resolveRustBuildTarget(targetOS string, targetArch string) (string, bool, error) {
	if targetOS == runtime.GOOS && targetArch == runtime.GOARCH {
		return "", true, nil
	}
	switch targetOS + "/" + targetArch {
	case "linux/amd64":
		return "x86_64-unknown-linux-musl", false, nil
	case "linux/arm64":
		return "aarch64-unknown-linux-musl", false, nil
	case "darwin/amd64":
		return "x86_64-apple-darwin", false, nil
	case "darwin/arm64":
		return "aarch64-apple-darwin", false, nil
	case "windows/amd64":
		return "x86_64-pc-windows-gnu", false, nil
	case "windows/arm64":
		return "aarch64-pc-windows-gnullvm", false, nil
	default:
		return "", false, fmt.Errorf("暂不支持为 %s/%s 构建 Rust 工具产物", targetOS, targetArch)
	}
}

func rustBinaryBaseName(toolID string, targetOS string) string {
	if targetOS == "windows" {
		return toolID + ".exe"
	}
	return toolID
}

func resolveCargoBinary() (string, error) {
	if override := strings.TrimSpace(os.Getenv(envCargoBinary)); override != "" {
		if isExecutableFile(override) {
			return override, nil
		}
		return "", fmt.Errorf("指定的 Cargo 不存在: %s", override)
	}
	path, err := toolchain.ResolveCargoBinary()
	if err != nil {
		return "", err
	}
	return path, nil
}

func resolveRustupBinary() (string, error) {
	if override := strings.TrimSpace(os.Getenv(envRustupBinary)); override != "" {
		if isExecutableFile(override) {
			return override, nil
		}
		return "", fmt.Errorf("指定的 rustup 不存在: %s", override)
	}
	path, err := toolchain.ResolveRustupBinary()
	if err != nil {
		return "", err
	}
	return path, nil
}

func resolveZigBinary() (string, error) {
	if override := strings.TrimSpace(os.Getenv(envZigBinary)); override != "" {
		if isExecutableFile(override) {
			return override, nil
		}
		return "", fmt.Errorf("指定的 zig 不存在: %s", override)
	}
	path, err := toolchain.ResolveZigBinary()
	if err != nil {
		return "", err
	}
	return path, nil
}

func resolveCargoZigbuildBinary() (string, error) {
	if override := strings.TrimSpace(os.Getenv(envCargoZigbuildBinary)); override != "" {
		if isExecutableFile(override) {
			return override, nil
		}
		return "", fmt.Errorf("指定的 cargo-zigbuild 不存在: %s", override)
	}
	path, err := toolchain.ResolveCargoZigbuildBinary()
	if err != nil {
		return "", err
	}
	return path, nil
}

func rustCommandEnv(cargoBinary string, zigBinary string, cargoZigbuildBinary string, targetDir string) []string {
	env := append([]string{}, os.Environ()...)
	env = appendOrReplaceEnv(env, "CARGO_TARGET_DIR", targetDir)
	currentPath := os.Getenv("PATH")
	for _, binPath := range []string{cargoBinary, zigBinary, cargoZigbuildBinary} {
		binDir := strings.TrimSpace(filepath.Dir(binPath))
		if binDir == "" {
			continue
		}
		if currentPath == "" {
			currentPath = binDir
			continue
		}
		if !pathListContains(currentPath, binDir) {
			currentPath = binDir + string(os.PathListSeparator) + currentPath
		}
	}
	if currentPath != "" {
		env = appendOrReplaceEnv(env, "PATH", currentPath)
	}
	return env
}

func fallbackCargoTool(name string) (string, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	candidate := filepath.Join(home, ".cargo", "bin", executableName(name))
	if isExecutableFile(candidate) {
		return candidate, true
	}
	return "", false
}

func executableName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func appendOrReplaceEnv(env []string, key string, value string) []string {
	prefix := key + "="
	for i, item := range env {
		if strings.HasPrefix(item, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

func pathListContains(currentPath string, candidate string) bool {
	for _, item := range filepath.SplitList(currentPath) {
		if filepath.Clean(item) == filepath.Clean(candidate) {
			return true
		}
	}
	return false
}
