package builder

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"fire-salamander-desktop/internal/toolchain"
	"my_tools/libs/core/procutil"
)

type ToolKind string

const (
	KindGo      ToolKind = "go"
	KindPython  ToolKind = "python"
	envGoBinary          = "FIRE_SALAMANDER_GO_BIN"
)

type BuildRequest struct {
	ToolID           string
	ToolName         string
	Kind             ToolKind
	OutputDir        string
	CacheDir         string
	OutputName       string
	RepoRoot         string
	SourceEntry      string
	TargetOS         string
	TargetArch       string
	ForceRebuild     bool
	UseCacheAsOutput bool
	Progress         io.Writer
}

type BuildResult struct {
	Path      string
	CachePath string
	CacheKey  string
	CacheHit  bool
}

func ProbeBuildCache(req BuildRequest) (BuildResult, error) {
	req.CacheDir = strings.TrimSpace(req.CacheDir)
	if req.CacheDir == "" {
		req.CacheDir = req.OutputDir
	}
	if strings.TrimSpace(req.CacheDir) == "" {
		return BuildResult{}, fmt.Errorf("缺少构建缓存目录")
	}

	switch req.Kind {
	case KindPython:
		return probePythonCache(req)
	case KindGo:
		return probeGoCache(req)
	default:
		return BuildResult{}, fmt.Errorf("不支持的工具类型: %s", req.Kind)
	}
}

func BuildPackage(req BuildRequest) (BuildResult, error) {
	if strings.TrimSpace(req.OutputDir) == "" {
		return BuildResult{}, fmt.Errorf("缺少输出目录")
	}
	if err := os.MkdirAll(req.OutputDir, 0755); err != nil {
		return BuildResult{}, fmt.Errorf("创建输出目录失败: %w", err)
	}

	req.CacheDir = strings.TrimSpace(req.CacheDir)
	if req.CacheDir == "" {
		req.CacheDir = req.OutputDir
	}
	if err := os.MkdirAll(req.CacheDir, 0755); err != nil {
		return BuildResult{}, fmt.Errorf("创建构建缓存目录失败: %w", err)
	}

	switch req.Kind {
	case KindPython:
		return buildPythonPackage(req)
	case KindGo:
		return buildGoPackage(req)
	default:
		return BuildResult{}, fmt.Errorf("不支持的工具类型: %s", req.Kind)
	}
}

func buildPythonPackage(req BuildRequest) (BuildResult, error) {
	scriptPath := req.SourceEntry
	if scriptPath == "" {
		return BuildResult{}, fmt.Errorf("工具 %s 缺少 Python 脚本入口", req.ToolID)
	}
	if !filepath.IsAbs(scriptPath) {
		if req.RepoRoot == "" {
			return BuildResult{}, fmt.Errorf("工具 %s 缺少仓库根目录，无法解析脚本入口", req.ToolID)
		}
		scriptPath = filepath.Join(req.RepoRoot, filepath.FromSlash(scriptPath))
	}
	if _, err := os.Stat(scriptPath); err != nil {
		return BuildResult{}, fmt.Errorf("Python 脚本不存在: %w", err)
	}

	outputName := strings.TrimSpace(req.OutputName)
	if outputName == "" {
		outputName = req.ToolID + ".py"
	}
	outFile := filepath.Join(req.OutputDir, filepath.Base(outputName))

	cacheKey, err := computePythonCacheKey(req, scriptPath)
	if err != nil {
		return BuildResult{}, err
	}
	cachePath, sourceCachePath, cacheKeyPath, err := resolveCachePaths(req, "script", outputName, scriptPath)
	if err != nil {
		return BuildResult{}, err
	}
	if !req.ForceRebuild && cacheEntryMatches(cachePath, cacheKeyPath, cacheKey) {
		logBuildProgress(req, "命中构建缓存")
		return finalizeBuildOutput(req, outFile, cachePath, 0644, cacheKey, true)
	}

	logBuildProgress(req, "写入 Python 脚本缓存")
	if err := copyFile(scriptPath, cachePath, 0644); err != nil {
		return BuildResult{}, fmt.Errorf("复制 Python 脚本失败: %w", err)
	}
	if err := copySourceSnapshot(scriptPath, sourceCachePath); err != nil {
		return BuildResult{}, err
	}
	if err := writeCacheKeyFile(cacheKeyPath, cacheKey); err != nil {
		return BuildResult{}, err
	}

	return finalizeBuildOutput(req, outFile, cachePath, 0644, cacheKey, false)
}

func probePythonCache(req BuildRequest) (BuildResult, error) {
	scriptPath := req.SourceEntry
	if scriptPath == "" {
		return BuildResult{}, fmt.Errorf("工具 %s 缺少 Python 脚本入口", req.ToolID)
	}
	if !filepath.IsAbs(scriptPath) {
		if req.RepoRoot == "" {
			return BuildResult{}, fmt.Errorf("工具 %s 缺少仓库根目录，无法解析脚本入口", req.ToolID)
		}
		scriptPath = filepath.Join(req.RepoRoot, filepath.FromSlash(scriptPath))
	}
	if _, err := os.Stat(scriptPath); err != nil {
		return BuildResult{}, fmt.Errorf("Python 脚本不存在: %w", err)
	}

	outputName := strings.TrimSpace(req.OutputName)
	if outputName == "" {
		outputName = req.ToolID + ".py"
	}
	cacheKey, err := computePythonCacheKey(req, scriptPath)
	if err != nil {
		return BuildResult{}, err
	}
	cachePath, _, cacheKeyPath, err := resolveCachePaths(req, "script", outputName, scriptPath)
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

func buildGoPackage(req BuildRequest) (BuildResult, error) {
	if req.SourceEntry == "" {
		return BuildResult{}, fmt.Errorf("工具 %s 缺少 Go 源码入口", req.ToolID)
	}
	if req.RepoRoot == "" {
		return BuildResult{}, fmt.Errorf("工具 %s 缺少仓库根目录，无法生成单工具产物", req.ToolID)
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

	importPath := buildImportPath(req.SourceEntry)
	cacheKey, err := computeGoCacheKey(req, importPath, targetOS, targetArch)
	if err != nil {
		return BuildResult{}, err
	}
	sourcePath, err := resolveSourceEntryPath(req)
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
	buildDir, err := os.MkdirTemp("", req.ToolID+"_"+targetOS+"_"+targetArch+"_src_")
	if err != nil {
		return BuildResult{}, fmt.Errorf("创建构建目录失败: %w", err)
	}
	defer os.RemoveAll(buildDir)

	wrapperPath := filepath.Join(buildDir, "main.go")
	if err := os.WriteFile(wrapperPath, []byte(renderGoWrapper(req.ToolID, importPath)), 0644); err != nil {
		return BuildResult{}, fmt.Errorf("写入构建入口失败: %w", err)
	}

	goBinary, err := resolveGoBinary()
	if err != nil {
		return BuildResult{}, err
	}

	logBuildProgress(req, "正在构建工具产物")
	cmd := procutil.Command(goBinary, "build", "-o", cachePath, wrapperPath)
	cmd.Dir = req.RepoRoot
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0",
		"GOOS="+targetOS,
		"GOARCH="+targetArch,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return BuildResult{}, fmt.Errorf("构建单工具产物失败: %w\n%s", err, strings.TrimSpace(string(output)))
	}

	if err := os.Chmod(cachePath, 0755); err != nil {
		return BuildResult{}, fmt.Errorf("设置可执行权限失败: %w", err)
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

func probeGoCache(req BuildRequest) (BuildResult, error) {
	if req.SourceEntry == "" {
		return BuildResult{}, fmt.Errorf("工具 %s 缺少 Go 源码入口", req.ToolID)
	}
	if req.RepoRoot == "" {
		return BuildResult{}, fmt.Errorf("工具 %s 缺少仓库根目录，无法生成单工具产物", req.ToolID)
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
	importPath := buildImportPath(req.SourceEntry)
	cacheKey, err := computeGoCacheKey(req, importPath, targetOS, targetArch)
	if err != nil {
		return BuildResult{}, err
	}
	sourcePath, err := resolveSourceEntryPath(req)
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

func buildImportPath(sourceEntry string) string {
	normalized := strings.TrimSpace(sourceEntry)
	normalized = strings.ReplaceAll(normalized, "\\", "/")
	normalized = filepath.ToSlash(normalized)
	normalized = strings.TrimPrefix(normalized, "./")
	importDir := path.Dir(normalized)
	importDir = strings.TrimPrefix(importDir, "./")
	importDir = strings.TrimPrefix(importDir, "/")
	return path.Clean("my_tools/" + importDir)
}

func resolveGoBinary() (string, error) {
	if override := strings.TrimSpace(os.Getenv(envGoBinary)); override != "" {
		if isExecutableFile(override) {
			return override, nil
		}
		return "", fmt.Errorf("指定的 Go 工具链不存在: %s", override)
	}

	path, err := toolchain.ResolveGoBinary()
	if err != nil {
		return "", fmt.Errorf("构建单工具产物失败: %w", err)
	}
	return path, nil
}

func goExecutableName() string {
	if runtime.GOOS == "windows" {
		return "go.exe"
	}
	return "go"
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
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

func finalizeBuildOutput(req BuildRequest, outFile string, cachePath string, mode os.FileMode, cacheKey string, cacheHit bool) (BuildResult, error) {
	if req.UseCacheAsOutput || sameCleanPath(outFile, cachePath) {
		return BuildResult{
			Path:      cachePath,
			CachePath: cachePath,
			CacheKey:  cacheKey,
			CacheHit:  cacheHit,
		}, nil
	}
	logBuildProgress(req, "写入目标产物")
	if err := copyFile(cachePath, outFile, mode); err != nil {
		return BuildResult{}, fmt.Errorf("写入目标产物失败: %w", err)
	}
	return BuildResult{
		Path:      outFile,
		CachePath: cachePath,
		CacheKey:  cacheKey,
		CacheHit:  cacheHit,
	}, nil
}

func computePythonCacheKey(req BuildRequest, scriptPath string) (string, error) {
	digest := sha256.New()
	writeCacheToken(digest, "python-cache-v1")
	writeCacheToken(digest, req.ToolID)
	writeCacheToken(digest, filepath.Base(scriptPath))
	if err := hashSingleFile(digest, scriptPath, filepath.Base(scriptPath)); err != nil {
		return "", fmt.Errorf("计算 Python 产物缓存失败: %w", err)
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func computeGoCacheKey(req BuildRequest, importPath string, targetOS string, targetArch string) (string, error) {
	digest := sha256.New()
	writeCacheToken(digest, "go-cache-v1")
	writeCacheToken(digest, req.ToolID)
	writeCacheToken(digest, importPath)
	writeCacheToken(digest, targetOS)
	writeCacheToken(digest, targetArch)
	writeCacheToken(digest, renderGoWrapper(req.ToolID, importPath))

	files, err := collectGoRelevantFiles(req.RepoRoot)
	if err != nil {
		return "", fmt.Errorf("扫描 Go 构建输入失败: %w", err)
	}
	for _, relPath := range files {
		if err := hashSingleFile(digest, filepath.Join(req.RepoRoot, filepath.FromSlash(relPath)), relPath); err != nil {
			return "", fmt.Errorf("计算 Go 产物缓存失败: %w", err)
		}
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func collectGoRelevantFiles(repoRoot string) ([]string, error) {
	files := make([]string, 0, 64)
	err := filepath.WalkDir(repoRoot, func(currentPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relPath, err := filepath.Rel(repoRoot, currentPath)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)
		if entry.IsDir() {
			if shouldSkipGoCacheDir(relPath, entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if isGoCacheInput(relPath) {
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

func shouldSkipGoCacheDir(relPath string, base string) bool {
	switch relPath {
	case ".", "":
		return false
	case "app/frontend":
		return true
	}
	switch base {
	case ".git", ".trae", "build", "node_modules", "dist":
		return true
	default:
		return false
	}
}

func isGoCacheInput(relPath string) bool {
	ext := strings.ToLower(filepath.Ext(relPath))
	switch ext {
	case ".go", ".mod", ".sum", ".work":
		return true
	default:
		return false
	}
}

func hashSingleFile(digest hash.Hash, filePath string, label string) error {
	info, err := os.Stat(filePath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("路径不是文件: %s", filePath)
	}
	writeCacheToken(digest, label)
	writeCacheToken(digest, fmt.Sprintf("%d", info.Size()))
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := io.Copy(digest, file); err != nil {
		return err
	}
	writeCacheToken(digest, "")
	return nil
}

func writeCacheToken(digest hash.Hash, value string) {
	_, _ = io.WriteString(digest, value)
	_, _ = io.WriteString(digest, "\n")
}

func resolveCachePaths(req BuildRequest, platformKey string, outputName string, sourcePath string) (artifactPath string, sourceCachePath string, cacheKeyPath string, err error) {
	toolDir := filepath.Join(req.CacheDir, cacheToolDirName(req))
	platformDir := filepath.Join(toolDir, sanitizeCachePathSegment(platformKey))
	artifactDir := filepath.Join(platformDir, "artifact")
	sourceDir := filepath.Join(platformDir, "source")
	for _, dir := range []string{artifactDir, sourceDir} {
		if mkErr := os.MkdirAll(dir, 0755); mkErr != nil {
			return "", "", "", fmt.Errorf("创建结构化缓存目录失败: %w", mkErr)
		}
	}
	artifactPath = filepath.Join(artifactDir, cacheArtifactFileName(req, outputName))
	sourceCachePath = filepath.Join(sourceDir, filepath.Base(sourcePath))
	cacheKeyPath = filepath.Join(platformDir, ".cachekey")
	return artifactPath, sourceCachePath, cacheKeyPath, nil
}

func cacheToolDirName(req BuildRequest) string {
	toolID := sanitizeCachePathSegment(req.ToolID)
	if toolID != "" && toolID != "artifact" {
		return toolID
	}
	return sanitizeCachePathSegment(cacheToolName(req))
}

func cacheArtifactFileName(req BuildRequest, outputName string) string {
	ext := filepath.Ext(outputName)
	base := sanitizeCachePathSegment(req.ToolID)
	if base == "" || base == "artifact" {
		base = sanitizeCachePathSegment(cacheToolName(req))
	}
	if req.Kind == KindPython {
		if ext == "" {
			ext = ".py"
		}
		return base + ext
	}
	if req.TargetOS != "" {
		base += "_" + sanitizeCachePathSegment(req.TargetOS)
	}
	if req.TargetArch != "" {
		base += "_" + sanitizeCachePathSegment(req.TargetArch)
	}
	if req.TargetOS == "windows" && ext == "" {
		ext = ".exe"
	}
	return base + ext
}

func cacheEntryMatches(artifactPath string, cacheKeyPath string, cacheKey string) bool {
	if !fileExists(artifactPath) || !fileExists(cacheKeyPath) {
		return false
	}
	storedKey, err := os.ReadFile(cacheKeyPath)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(storedKey)) == strings.TrimSpace(cacheKey)
}

func writeCacheKeyFile(cacheKeyPath string, cacheKey string) error {
	if err := os.MkdirAll(filepath.Dir(cacheKeyPath), 0755); err != nil {
		return fmt.Errorf("创建缓存元数据目录失败: %w", err)
	}
	if err := os.WriteFile(cacheKeyPath, []byte(cacheKey+"\n"), 0644); err != nil {
		return fmt.Errorf("写入缓存元数据失败: %w", err)
	}
	return nil
}

func copySourceSnapshot(sourcePath string, snapshotPath string) error {
	if err := copyFile(sourcePath, snapshotPath, 0644); err != nil {
		return fmt.Errorf("写入源码缓存失败: %w", err)
	}
	return nil
}

func resolveSourceEntryPath(req BuildRequest) (string, error) {
	entry := strings.TrimSpace(req.SourceEntry)
	if entry == "" {
		return "", fmt.Errorf("缺少源码入口")
	}
	if filepath.IsAbs(entry) {
		return entry, nil
	}
	if strings.TrimSpace(req.RepoRoot) == "" {
		return "", fmt.Errorf("缺少仓库根目录，无法解析源码入口")
	}
	return filepath.Join(req.RepoRoot, filepath.FromSlash(entry)), nil
}

func cacheToolName(req BuildRequest) string {
	if strings.TrimSpace(req.ToolName) != "" {
		return req.ToolName
	}
	return req.ToolID
}

func sanitizeCachePathSegment(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "artifact"
	}
	safe := strings.Map(func(r rune) rune {
		switch {
		case r < 32:
			return -1
		case strings.ContainsRune(`<>:"/\|?*`, r):
			return '_'
		default:
			return r
		}
	}, trimmed)
	safe = strings.Trim(safe, ". ")
	if safe == "" {
		return "artifact"
	}
	return safe
}

func sameCleanPath(left string, right string) bool {
	return filepath.Clean(left) == filepath.Clean(right)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func logBuildProgress(req BuildRequest, message string) {
	if req.Progress == nil {
		return
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}
	_, _ = io.WriteString(req.Progress, message+"\n")
}

func copyFile(srcPath, dstPath string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return err
	}
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}

	return os.Chmod(dstPath, mode)
}

func renderGoWrapper(toolID, importPath string) string {
	return fmt.Sprintf("package main\n\n"+
		"import (\n"+
		"\t\"context\"\n"+
		"\t\"fmt\"\n"+
		"\t\"io\"\n"+
		"\t\"os\"\n"+
		"\t\"strings\"\n\n"+
		"\t\"my_tools/libs/framework\"\n"+
		"\t_ %q\n"+
		")\n\n"+
		"type captureAppContext struct {\n"+
		"\trun func(ctx context.Context, args string, out io.Writer) error\n"+
		"}\n\n"+
		"func (c *captureAppContext) ShowModal(title, message string) {}\n"+
		"func (c *captureAppContext) PromptInput(title, prompt, defaultValue string, callback func(string)) {}\n"+
		"func (c *captureAppContext) PromptChoice(title, prompt string, options []string, callback func(string)) {}\n"+
		"func (c *captureAppContext) ShowTerminal(title string, usage string, run func(ctx context.Context, args string, out io.Writer) error) {\n"+
		"\tc.run = run\n"+
		"}\n"+
		"func (c *captureAppContext) ShowPythonTerminal(title string, usage string, run func(ctx context.Context, env string, args string, out io.Writer) error) {}\n"+
		"func (c *captureAppContext) GetLastParam(key string) string { return \"\" }\n"+
		"func (c *captureAppContext) RecordUsage(params map[string]string) {}\n\n"+
		"func joinArgs(args []string) string {\n"+
		"\tif len(args) == 0 {\n"+
		"\t\treturn \"\"\n"+
		"\t}\n"+
		"\tparts := make([]string, 0, len(args))\n"+
		"\tfor _, arg := range args {\n"+
		"\t\tif arg == \"\" {\n"+
		"\t\t\tparts = append(parts, \"\\\"\\\"\")\n"+
		"\t\t\tcontinue\n"+
		"\t\t}\n"+
		"\t\tif strings.ContainsAny(arg, \" \\t\\r\\n\") {\n"+
		"\t\t\tparts = append(parts, \"\\\"\"+strings.ReplaceAll(arg, \"\\\"\", \"\\\\\\\"\")+\"\\\"\")\n"+
		"\t\t\tcontinue\n"+
		"\t\t}\n"+
		"\t\tparts = append(parts, arg)\n"+
		"\t}\n"+
		"\treturn strings.Join(parts, \" \")\n"+
		"}\n\n"+
		"func main() {\n"+
		"\tvar selected framework.Tool\n"+
		"\tfor _, tool := range framework.Registry {\n"+
		"\t\tif tool.ID() == %q {\n"+
		"\t\t\tselected = tool\n"+
		"\t\t\tbreak\n"+
		"\t\t}\n"+
		"\t}\n"+
		"\tif selected == nil {\n"+
		"\t\tfmt.Fprintf(os.Stderr, \"未找到工具: %%s\\n\", %q)\n"+
		"\t\tos.Exit(1)\n"+
		"\t}\n\n"+
		"\tcapture := &captureAppContext{}\n"+
		"\tselected.Execute(capture)\n"+
		"\tif capture.run == nil {\n"+
		"\t\tfmt.Fprintf(os.Stderr, \"工具 %%s 缺少 Go 执行入口\\n\", %q)\n"+
		"\t\tos.Exit(1)\n"+
		"\t}\n\n"+
		"\tif err := capture.run(context.Background(), joinArgs(os.Args[1:]), os.Stdout); err != nil {\n"+
		"\t\tfmt.Fprintln(os.Stderr, err)\n"+
		"\t\tos.Exit(1)\n"+
		"\t}\n"+
		"}\n",
		importPath, toolID, toolID, toolID)
}
