package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"my_tools/libs/catalog/builtin"
)

type Target struct {
	OS   string
	Arch string
}

var allTargets = []Target{
	{"darwin", "amd64"},
	{"darwin", "arm64"},
	{"linux", "amd64"},
	{"linux", "arm64"},
	{"windows", "amd64"},
	{"windows", "arm64"},
}

func wailsBin() string {
	p := os.Getenv("GOPATH")
	if p == "" {
		home, _ := os.UserHomeDir()
		p = filepath.Join(home, "go")
	}
	bin := filepath.Join(p, "bin", "wails")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	return bin
}

func main() {
	rootDir, _ := os.Getwd()
	appDir := filepath.Join(rootDir, "app")
	hostDir := filepath.Join(rootDir, "build") // 单层交付根
	programToolsDir := filepath.Join(hostDir, "program", "tools")
	dataDir := filepath.Join(hostDir, "data")

	buildAll := false
	targetOS := ""
	targetArch := ""
	for i := 1; i < len(os.Args); i++ {
		a := os.Args[i]
		if a == "-all" || a == "--all" {
			buildAll = true
		}
		if (a == "--target" || a == "-target") && i+1 < len(os.Args) {
			parts := strings.SplitN(os.Args[i+1], "/", 2)
			if len(parts) == 2 {
				targetOS = parts[0]
				targetArch = parts[1]
			}
		}
	}

	fmt.Println("====================================")
	fmt.Println("   火蜥蜴工具箱 Desktop - 构建脚本")
	fmt.Println("====================================")

	validateBuiltinManifests()
	if err := buildFrontend(appDir); err != nil {
		fmt.Printf("❌ 前端构建失败: %v\n", err)
		os.Exit(1)
	}
	runGoVet(rootDir, appDir)

	// 清理可重建的程序区（exe/assets），保留 data/ 用户数据与 program/ 工具预编译缓存
	os.RemoveAll(filepath.Join(appDir, "build"))
	os.MkdirAll(hostDir, 0755)
	os.MkdirAll(programToolsDir, 0755)

	cleanBuildRootStale(rootDir)

	initRuntimeDirs(dataDir)

	if buildAll {
		platforms := make([]string, len(allTargets))
		for i, t := range allTargets {
			platforms[i] = t.OS + "/" + t.Arch
		}
		platStr := strings.Join(platforms, ",")

		fmt.Printf("\n全平台编译: %s\n", platStr)
		cmd := exec.Command(wailsBin(), "build", "-clean", "-platform", platStr)
		cmd.Dir = appDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()

		srcBin := filepath.Join(appDir, "build", "bin")
		if entries, readErr := os.ReadDir(srcBin); readErr == nil {
			for _, e := range entries {
				os.Rename(filepath.Join(srcBin, e.Name()), filepath.Join(hostDir, e.Name()))
			}
		}
		os.RemoveAll(filepath.Join(appDir, "build"))

		if err != nil {
			fmt.Println("\n⚠ 部分平台编译失败（见上方错误）")
		}
	} else {
		curr := runtime.GOOS + "/" + runtime.GOARCH
		fmt.Printf("\n编译: %s\n", curr)

		cmd := exec.Command(wailsBin(), "build", "-clean")
		cmd.Dir = appDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Println("\n❌ 编译失败")
			os.Exit(1)
		}

		srcBin := filepath.Join(appDir, "build", "bin")
		if entries, readErr := os.ReadDir(srcBin); readErr == nil {
			for _, e := range entries {
				os.Rename(filepath.Join(srcBin, e.Name()), filepath.Join(hostDir, e.Name()))
			}
		}
		os.RemoveAll(filepath.Join(appDir, "build"))
	}

	if runtime.GOOS == "darwin" {
		if entries, err := os.ReadDir(hostDir); err == nil {
			for _, e := range entries {
				if strings.HasSuffix(e.Name(), ".app") {
					appPath := filepath.Join(hostDir, e.Name())
					exec.Command("xattr", "-d", "com.apple.quarantine", appPath).Run()
					exec.Command("xattr", "-cr", appPath).Run()
				}
			}
		}
	}

	writeDefaultConfig(dataDir)

	copyAssets(filepath.Join(rootDir, "app", "assets"), filepath.Join(hostDir, "assets"))

	// 工具产物统一预热到 build/program/tools（交付只读程序区）。
	// 预编译不随包携带工具链：Windows 宿主编译 windows + linux/amd64，
	// macOS 宿主编译 darwin(宿主架构) + linux/amd64，供远程 Linux 执行使用。
	targets := resolveToolTargets(targetOS, targetArch, buildAll)
	labels := make([]string, len(targets))
	for i, t := range targets {
		labels[i] = t.OS + "/" + t.Arch
	}
	fmt.Printf("\n预热工具缓存 (目标: %s)...\n", strings.Join(labels, ", "))
	if err := buildToolCache(rootDir, programToolsDir, targets); err != nil {
		fmt.Printf("\n❌ 工具缓存预热失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n====================================")
	fmt.Printf("✅ 产物: %s\n", hostDir)
	entries, _ := os.ReadDir(hostDir)
	for _, e := range entries {
		fmt.Printf("   %s\n", e.Name())
	}
	fmt.Println("====================================")
}

func validateBuiltinManifests() {
	fmt.Println("\n校验内置工具清单...")
	manifests, err := builtin.Load()
	if err != nil {
		fmt.Printf("❌ 内置工具清单校验失败: %v\n", err)
		os.Exit(1)
	}
	seen := make(map[string]struct{}, len(manifests))
	for _, manifest := range manifests {
		if manifest.ID == "" {
			fmt.Println("❌ 内置工具清单校验失败: 存在空 id 的 manifest")
			os.Exit(1)
		}
		if manifest.Name == "" {
			fmt.Printf("❌ 内置工具清单校验失败: manifest %q 缺少 name\n", manifest.ID)
			os.Exit(1)
		}
		if manifest.Source.Entry == "" {
			fmt.Printf("❌ 内置工具清单校验失败: manifest %q 缺少 source.entry\n", manifest.ID)
			os.Exit(1)
		}
		if _, exists := seen[manifest.ID]; exists {
			fmt.Printf("❌ 内置工具清单校验失败: manifest id 重复 %q\n", manifest.ID)
			os.Exit(1)
		}
		seen[manifest.ID] = struct{}{}
	}
	fmt.Printf("内置工具清单校验通过: %d 个 manifest\n", len(manifests))
}

func runGoVet(rootDir, appDir string) {
	fmt.Println("\ngo vet...")

	rootPackages := []string{"./libs/...", "./tools/...", "./scripts/..."}
	runVetCommand(rootDir, rootPackages)
	runVetCommand(appDir, []string{"./..."})
}

func buildFrontend(appDir string) error {
	fmt.Println("\n构建前端...")
	frontendDir := filepath.Join(appDir, "frontend")
	npmBin, err := exec.LookPath("npm")
	if err != nil {
		return fmt.Errorf("未找到 npm: %w", err)
	}
	if _, err := os.Stat(filepath.Join(frontendDir, "node_modules")); err != nil {
		fmt.Println("  未检测到 node_modules，执行 npm install...")
		install := exec.Command(npmBin, "install")
		install.Dir = frontendDir
		install.Stdout = os.Stdout
		install.Stderr = os.Stderr
		if err := install.Run(); err != nil {
			return fmt.Errorf("npm install 失败: %w", err)
		}
	}
	build := exec.Command(npmBin, "run", "build")
	build.Dir = frontendDir
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(frontendDir, "dist")); err != nil {
		return fmt.Errorf("前端构建未生成 dist 目录: %w", err)
	}
	return nil
}

func runVetCommand(dir string, packages []string) {
	args := append([]string{"vet"}, packages...)
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("⚠ go vet 失败: %v\n", err)
	}
}

// cleanBuildRootStale 清理 build/ 根目录下可重建的构建产物（exe/assets 等），
// 保留 data/（用户可变数据）与 program/（工具预编译缓存，源码未变时命中跳过重编）。
func cleanBuildRootStale(rootDir string) {
	buildDir := filepath.Join(rootDir, "build")
	entries, err := os.ReadDir(buildDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.Name() == "data" || e.Name() == "program" {
			continue
		}
		os.RemoveAll(filepath.Join(buildDir, e.Name()))
	}
}

var runtimeDirs = []string{
	"cache",
	"cache/builds",
	"cache/scripts",
	"config",
	"logs",
	"exports",
}

func initRuntimeDirs(runtimeDir string) {
	for _, sub := range runtimeDirs {
		os.MkdirAll(filepath.Join(runtimeDir, sub), 0755)
	}
}

const defaultConfigJSON = `{
  "app": {
    "version": "1.0.0",
    "language": "zh-CN"
  },
  "execution": {
    "defaultPython": "python3",
    "maxHistory": 50,
    "remoteTimeoutSec": 30
  },
  "export": {
    "lastDirectory": "",
    "goMode": "binary",
    "autoOpenDir": true
  },
  "go": {
    "selectedBinary": "",
    "knownBinaries": [],
    "lastInstallDirectory": "",
    "disabled": false
  },
  "ui": {
    "theme": "dracula",
    "verboseShortcuts": false
  },
  "window": {
    "width": 0,
    "height": 0,
    "x": -1,
    "y": -1,
    "maximised": false,
    "fullscreen": false
  }
}
`

func writeDefaultConfig(runtimeDir string) {
	configDir := filepath.Join(runtimeDir, "config")
	os.MkdirAll(configDir, 0755)

	appConfig := filepath.Join(configDir, "app.json")
	if _, err := os.Stat(appConfig); err == nil {
		return
	}
	os.WriteFile(appConfig, []byte(defaultConfigJSON), 0644)
}

func copyAssets(srcDir, dstDir string) {
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return
	}
	os.RemoveAll(dstDir)
	os.MkdirAll(dstDir, 0755)

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		src := filepath.Join(srcDir, e.Name())
		dst := filepath.Join(dstDir, e.Name())
		data, err := os.ReadFile(src)
		if err != nil {
			continue
		}
		os.WriteFile(dst, data, 0644)
	}
}

// resolveToolTargets 决定工具产物预编译的目标组合。
// 显式 --target 时只编译指定目标；--all 编译全平台；默认按宿主平台组合：
// Windows 宿主 → windows(宿主架构) + linux/amd64；
// macOS 宿主 → darwin(宿主架构) + linux/amd64；其余仅宿主平台。
func resolveToolTargets(targetOS, targetArch string, buildAll bool) []Target {
	if targetOS != "" && targetArch != "" {
		return []Target{{OS: targetOS, Arch: targetArch}}
	}
	if buildAll {
		return allTargets
	}
	host := Target{OS: runtime.GOOS, Arch: runtime.GOARCH}
	if host.OS != "windows" && host.OS != "darwin" {
		return []Target{host}
	}
	targets := []Target{host}
	if host.OS != "linux" || host.Arch != "amd64" {
		targets = append(targets, Target{OS: "linux", Arch: "amd64"})
	}
	return targets
}

// ---- 工具缓存预热 ----

type bundledTool struct {
	ID          string
	Kind        string // "go" / "rust"
	SourceEntry string
	ModuleDir   string // Go: "", Rust: crate根目录相对路径
}

var bundledTools = []bundledTool{
	{ID: "geojson_to_shp", Kind: "go", SourceEntry: "tools/go_tools/geojson_to_shapefile/tool.go"},
	{ID: "hdfs_download", Kind: "go", SourceEntry: "tools/go_tools/hdfs_download/tool.go"},
	{ID: "pos2gis_converter", Kind: "go", SourceEntry: "tools/go_tools/pos_trajectory_to_gis/tool.go"},
	{ID: "recursive_content_dir_diff", Kind: "go", SourceEntry: "tools/go_tools/recursive_content_dir_diff/tool.go"},
	{ID: "trajectory_match_filter_qc", Kind: "go", SourceEntry: "tools/go_tools/trajectory_match_filter_qc/tool.go"},
	{ID: "utm_geojson_converter", Kind: "go", SourceEntry: "tools/go_tools/utm_extract_to_gis/tool.go"},
	{ID: "bxn_track_merger", Kind: "go", SourceEntry: "tools/go_tools/bxn_route_track_merger/tool.go"},
	{ID: "bxn_point_cloud_renew", Kind: "go", SourceEntry: "tools/go_tools/bxn_point_cloud_renew/tool.go"},
	{ID: "bxn_delivery_point_cloud_qc", Kind: "rust", SourceEntry: "tools/rust_tools/bxn_delivery_point_cloud_qc/src/lib.rs", ModuleDir: "tools/rust_tools/bxn_delivery_point_cloud_qc"},
	{ID: "pcd_to_laz_per_frame", Kind: "rust", SourceEntry: "tools/rust_tools/pcd_to_laz_per_frame/src/lib.rs", ModuleDir: "tools/rust_tools/pcd_to_laz_per_frame"},
	{ID: "point_cloud_voxel_downsample", Kind: "rust", SourceEntry: "tools/rust_tools/point_cloud_voxel_downsample/src/lib.rs", ModuleDir: "tools/rust_tools/point_cloud_voxel_downsample"},
	{ID: "point_cloud_intensity_raster", Kind: "rust", SourceEntry: "tools/rust_tools/point_cloud_intensity_raster/src/lib.rs", ModuleDir: "tools/rust_tools/point_cloud_intensity_raster"},
	{ID: "point_cloud_batch_merge", Kind: "rust", SourceEntry: "tools/rust_tools/point_cloud_batch_merge/src/lib.rs", ModuleDir: "tools/rust_tools/point_cloud_batch_merge"},
	{ID: "restore_pcd_by_mgrs", Kind: "rust", SourceEntry: "tools/rust_tools/restore_pcd_by_mgrs/src/lib.rs", ModuleDir: "tools/rust_tools/restore_pcd_by_mgrs"},
}

// buildToolCache 把内置工具预编译到 programToolsDir（即 build/program/tools），
// 产物结构为 <toolID>/<platform>/artifact/<name>，与 builder 的预置取件路径一致。
func buildToolCache(rootDir, programToolsDir string, targets []Target) error {
	cacheDir := programToolsDir
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}

	for _, tool := range bundledTools {
		for _, target := range targets {
			platform := target.OS + "_" + target.Arch
			fmt.Printf("  %s: %s -> %s\n", tool.Kind, tool.ID, platform)

			switch tool.Kind {
			case "go":
				if err := buildGoToCache(rootDir, cacheDir, tool, target); err != nil {
					return err
				}
			case "rust":
				if err := buildRustToCache(rootDir, cacheDir, tool, target); err != nil {
					return err
				}
			}
		}
	}

	fmt.Println("  工具缓存预热完成")
	return nil
}

// ---- Go tool build ----

func buildGoToCache(rootDir, cacheDir string, tool bundledTool, target Target) error {
	importPath := goImportPath(tool.SourceEntry)
	platform := target.OS + "_" + target.Arch
	artifactPath, cacheKeyPath, err := cachePaths(cacheDir, tool.ID, platform, tool.SourceEntry)
	if err != nil {
		return err
	}

	// Compute cache key (matches builder's computeGoCacheKey)
	cacheKey := computeGoCacheKey(rootDir, tool, importPath, target)

	// Check if cached and up-to-date
	if cacheEntryMatch(artifactPath, cacheKeyPath, cacheKey) {
		fmt.Printf("    (缓存命中)\n")
		return nil
	}

	// Create wrapper main.go
	tmpDir, err := os.MkdirTemp("", tool.ID+"_build_")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	wrapperPath := filepath.Join(tmpDir, "main.go")
	wrapper := goWrapper(importPath)
	if err := os.WriteFile(wrapperPath, []byte(wrapper), 0644); err != nil {
		return fmt.Errorf("写入 Go wrapper 失败: %w", err)
	}

	// Build
	outName := tool.ID
	if target.OS == "windows" {
		outName += ".exe"
	}
	outPath := filepath.Join(tmpDir, outName)

	goCacheDir := filepath.Join(rootDir, "tools", "go_tools", "build_cache")
	os.MkdirAll(goCacheDir, 0755)
	cmd := exec.Command("go", "build", "-o", outPath, wrapperPath)
	cmd.Dir = rootDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = appendGoEnv(target)
	cmd.Env = append(cmd.Env, "GOCACHE="+goCacheDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("构建 Go 工具 %s 失败: %w", tool.ID, err)
	}

	// Copy to cache
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0755); err != nil {
		return err
	}
	if err := copyFile(outPath, artifactPath, 0755); err != nil {
		return err
	}
	// Write cache key
	if err := os.WriteFile(cacheKeyPath, []byte(cacheKey+"\n"), 0644); err != nil {
		return err
	}

	return nil
}

func appendGoEnv(target Target) []string {
	return append(os.Environ(),
		"CGO_ENABLED=0",
		"GOOS="+target.OS,
		"GOARCH="+target.Arch,
	)
}

func goImportPath(sourceEntry string) string {
	normalized := strings.ReplaceAll(sourceEntry, "\\", "/")
	importDir := path.Dir(normalized)
	return path.Clean("my_tools/" + importDir)
}

func goWrapper(importPath string) string {
	pkgName := path.Base(importPath)
	return fmt.Sprintf("package main\n\n"+
		"import (\n"+
		"\t\"context\"\n"+
		"\t\"fmt\"\n"+
		"\t\"os\"\n\n"+
		"\t%q\n"+
		")\n\n"+
		"func main() {\n"+
		"\terr := %s.Run(context.Background(), os.Args[1:], os.Stdout)\n"+
		"\tif err != nil {\n"+
		"\t\tfmt.Fprintln(os.Stderr, err)\n"+
		"\t\tos.Exit(1)\n"+
		"\t}\n"+
		"}\n",
		importPath, pkgName)
}

func computeGoCacheKey(rootDir string, tool bundledTool, importPath string, target Target) string {
	digest := sha256.New()
	writeKeyToken(digest, "go-cache-v3")
	writeKeyToken(digest, tool.ID)
	writeKeyToken(digest, importPath)
	writeKeyToken(digest, target.OS)
	writeKeyToken(digest, target.Arch)
	writeKeyToken(digest, goWrapper(importPath))

	files, _ := collectGoFiles(rootDir, tool.SourceEntry)
	for _, relPath := range files {
		hashFile(digest, filepath.Join(rootDir, filepath.FromSlash(relPath)), relPath)
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func collectGoFiles(repoRoot, sourceEntry string) ([]string, error) {
	var files []string

	// Root module/workspace files
	for _, name := range []string{"go.mod", "go.sum", "go.work", "go.work.sum"} {
		if _, err := os.Stat(filepath.Join(repoRoot, name)); err == nil {
			files = append(files, name)
		}
	}

	// Shared libraries: libs/
	collectDirGoFiles(&files, repoRoot, filepath.Join(repoRoot, "libs"))

	// Tool's own source directory
	toolDir := filepath.Dir(filepath.Join(repoRoot, filepath.FromSlash(sourceEntry)))
	collectDirGoFiles(&files, repoRoot, toolDir)

	sort.Strings(files)
	return files, nil
}

func collectDirGoFiles(files *[]string, repoRoot, dir string) {
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return
	}
	filepath.WalkDir(dir, func(currentPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(repoRoot, currentPath)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		switch strings.ToLower(filepath.Ext(rel)) {
		case ".go", ".mod", ".sum":
			*files = append(*files, rel)
		}
		return nil
	})
}

// ---- Rust tool build ----

func buildRustToCache(rootDir, cacheDir string, tool bundledTool, target Target) error {
	platform := target.OS + "_" + target.Arch
	artifactPath, cacheKeyPath, err := cachePaths(cacheDir, tool.ID, platform, tool.SourceEntry)
	if err != nil {
		return err
	}

	crateDir := filepath.Join(rootDir, filepath.FromSlash(tool.ModuleDir))
	crateDirAbs, err := filepath.Abs(crateDir)
	if err != nil {
		return err
	}

	targetTriple, nativeBuild, err := rustTargetTriple(target.OS, target.Arch)
	if err != nil {
		return fmt.Errorf("Rust 目标不支持 %s/%s: %w", target.OS, target.Arch, err)
	}

	// Compute cache key
	cacheKey := computeRustCacheKey(crateDir, tool, target.OS, target.Arch, targetTriple, nativeBuild)
	if cacheEntryMatch(artifactPath, cacheKeyPath, cacheKey) {
		fmt.Printf("    (缓存命中)\n")
		return nil
	}

	// Create wrapper crate
	wrapperDir, err := os.MkdirTemp("", tool.ID+"_rust_wrapper_")
	if err != nil {
		return err
	}
	defer os.RemoveAll(wrapperDir)

	if err := writeRustWrapper(wrapperDir, tool.ID, crateDirAbs); err != nil {
		return err
	}

	// Shared target/ under tools/rust_tools - all Rust tools share dependency compilation
	targetDir := filepath.Join(rootDir, "tools", "rust_tools", "target")

	cargoBin, _ := exec.LookPath("cargo")
	if cargoBin == "" {
		return fmt.Errorf("未找到 Cargo")
	}

	args := []string{"build", "--release"}
	if !nativeBuild {
		// Install target
		rustupBin, _ := exec.LookPath("rustup")
		if rustupBin != "" {
			addCmd := exec.Command(rustupBin, "target", "add", targetTriple)
			addCmd.Stdout = os.Stdout
			addCmd.Stderr = os.Stderr
			addCmd.Run() // best-effort
		}

		args = []string{"zigbuild", "--release", "--target", targetTriple}
	}

	buildCmd := exec.Command(cargoBin, args...)
	buildCmd.Dir = wrapperDir
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	buildCmd.Env = rustBuildEnv(rootDir, cargoBin)
	// Use crate's target/ so incremental compilation works across wrapper rebuilds
	buildCmd.Env = append(buildCmd.Env, "CARGO_TARGET_DIR="+targetDir)
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("构建 Rust 工具 %s 失败: %w", tool.ID, err)
	}

	// Find built binary
	wrapperName := "wrapper_" + tool.ID
	if target.OS == "windows" {
		wrapperName += ".exe"
	}
	builtPath := filepath.Join(targetDir, "release", wrapperName)
	if !nativeBuild {
		builtPath = filepath.Join(targetDir, targetTriple, "release", wrapperName)
	}

	if err := os.MkdirAll(filepath.Dir(artifactPath), 0755); err != nil {
		return err
	}
	if err := copyFile(builtPath, artifactPath, 0755); err != nil {
		return err
	}
	if err := os.WriteFile(cacheKeyPath, []byte(cacheKey+"\n"), 0644); err != nil {
		return err
	}

	return nil
}

func writeRustWrapper(wrapperDir, toolID, crateDirAbs string) error {
	srcDir := filepath.Join(wrapperDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		return err
	}

	cargoToml := fmt.Sprintf(`[package]
name = "wrapper_%s"
version = "0.0.0"
edition = "2024"

[dependencies]
%s = { path = %q }
`, toolID, toolID, filepath.ToSlash(crateDirAbs))

	if err := os.WriteFile(filepath.Join(wrapperDir, "Cargo.toml"), []byte(cargoToml), 0644); err != nil {
		return err
	}

	mainRs := fmt.Sprintf(`fn main() {
    let args: Vec<String> = std::env::args().skip(1).collect();
    if let Err(e) = %s::run(&args) {
        eprintln!("{e}");
        std::process::exit(1);
    }
}
`, toolID)

	return os.WriteFile(filepath.Join(srcDir, "main.rs"), []byte(mainRs), 0644)
}

func computeRustCacheKey(crateDir string, tool bundledTool, targetOS, targetArch, targetTriple string, nativeBuild bool) string {
	digest := sha256.New()
	writeKeyToken(digest, "rust-cache-v2")
	writeKeyToken(digest, tool.ID)
	writeKeyToken(digest, targetOS)
	writeKeyToken(digest, targetArch)
	writeKeyToken(digest, targetTriple)
	if nativeBuild {
		writeKeyToken(digest, "native")
	} else {
		writeKeyToken(digest, "cross")
	}

	crateDirAbs, _ := filepath.Abs(crateDir)
	writeKeyToken(digest, rustWrapperCargoTomlStr(tool.ID, crateDirAbs))
	writeKeyToken(digest, rustWrapperMainRsStr(tool.ID))

	files, _ := collectRustFiles(crateDir)
	for _, relPath := range files {
		hashFile(digest, filepath.Join(crateDir, filepath.FromSlash(relPath)), relPath)
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func rustWrapperCargoTomlStr(toolID, crateDirAbs string) string {
	return fmt.Sprintf(`[package]
name = "wrapper_%s"
version = "0.0.0"
edition = "2024"

[dependencies]
%s = { path = %q }
`, toolID, toolID, filepath.ToSlash(crateDirAbs))
}

func rustWrapperMainRsStr(toolID string) string {
	return fmt.Sprintf(`fn main() {
    let args: Vec<String> = std::env::args().skip(1).collect();
    if let Err(e) = %s::run(&args) {
        eprintln!("{e}");
        std::process::exit(1);
    }
}
`, toolID)
}

func collectRustFiles(crateRoot string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(crateRoot, func(currentPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(crateRoot, currentPath)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			switch rel {
			case ".", "":
				return nil
			case "target", ".git":
				return filepath.SkipDir
			}
			return nil
		}
		base := filepath.Base(rel)
		if base == "Cargo.toml" || base == "Cargo.lock" {
			files = append(files, rel)
		}
		if strings.HasPrefix(rel, ".cargo/") && base == "config.toml" {
			files = append(files, rel)
		}
		if strings.EqualFold(filepath.Ext(rel), ".rs") {
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func rustTargetTriple(targetOS, targetArch string) (string, bool, error) {
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
		return "", false, fmt.Errorf("暂不支持目标 %s/%s", targetOS, targetArch)
	}
}

// rustBuildEnv 组装 Rust 构建命令环境：
//  1. cargo 所在目录进入 PATH，保证 cargo-zigbuild 子命令可被发现；
//  2. 托管 zig（build/data/toolchains/zig/<version>）通过 CARGO_ZIGBUILD_ZIG_PATH
//     显式指定，系统 PATH 里已有 zig 时以系统优先；
//  3. cargo-zigbuild 与 zig 的缓存指到 tools/rust_tools/ 下，避免写入用户
//     全局目录（%LOCALAPPDATA%），构建产物可重复利用且不污染系统。
func rustBuildEnv(rootDir, cargoBinary string) []string {
	env := os.Environ()
	current := os.Getenv("PATH")

	if zigBin, ok := findManagedZigBinary(rootDir); ok {
		env = appendOrReplaceEnv(env, "CARGO_ZIGBUILD_ZIG_PATH", zigBin)
		if !strings.Contains(current, filepath.Dir(zigBin)) {
			if current == "" {
				current = filepath.Dir(zigBin)
			} else {
				current += string(os.PathListSeparator) + filepath.Dir(zigBin)
			}
		}
	}

	cargoDir := filepath.Dir(cargoBinary)
	if cargoDir != "" && cargoDir != "." && !strings.Contains(current, cargoDir) {
		if current == "" {
			current = cargoDir
		} else {
			current = cargoDir + string(os.PathListSeparator) + current
		}
	}

	if current != os.Getenv("PATH") {
		env = appendOrReplaceEnv(env, "PATH", current)
	}

	zigbuildCache := filepath.Join(rootDir, "tools", "rust_tools", "zigbuild_cache")
	env = appendOrReplaceEnv(env, "CARGO_ZIGBUILD_CACHE_DIR", zigbuildCache)
	env = appendOrReplaceEnv(env, "ZIG_GLOBAL_CACHE_DIR", filepath.Join(zigbuildCache, "zig"))
	return env
}

// findManagedZigBinary 查找应用托管的 zig 安装（build/data/toolchains/zig/<version>/zig.exe），
// 返回其可执行文件路径；未找到时返回 false，此时依赖系统 PATH 中的 zig。
func findManagedZigBinary(rootDir string) (string, bool) {
	baseDir := filepath.Join(rootDir, "build", "data", "toolchains", "zig")
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return "", false
	}
	zigName := "zig"
	if runtime.GOOS == "windows" {
		zigName += ".exe"
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		bin := filepath.Join(baseDir, entry.Name(), zigName)
		if _, err := os.Stat(bin); err == nil {
			return bin, true
		}
	}
	return "", false
}

func appendOrReplaceEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

// ---- shared helpers ----

func cachePaths(cacheDir, toolID, platform, sourceEntry string) (artifactPath, cacheKeyPath string, err error) {
	toolDir := filepath.Join(cacheDir, toolID, platform)
	artifactDir := filepath.Join(toolDir, "artifact")
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		return "", "", err
	}

	artifactName := toolID + "_" + platform
	if strings.Contains(platform, "windows") {
		artifactName += ".exe"
	}
	artifactPath = filepath.Join(artifactDir, artifactName)
	cacheKeyPath = filepath.Join(toolDir, ".cachekey")
	return artifactPath, cacheKeyPath, nil
}

func cacheEntryMatch(artifactPath, cacheKeyPath, cacheKey string) bool {
	if !fileExists(artifactPath) || !fileExists(cacheKeyPath) {
		return false
	}
	stored, err := os.ReadFile(cacheKeyPath)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(stored)) == strings.TrimSpace(cacheKey)
}

func writeKeyToken(digest hash.Hash, value string) {
	io.WriteString(digest, value)
	io.WriteString(digest, "\n")
}

func hashFile(digest hash.Hash, filePath, label string) error {
	info, err := os.Stat(filePath)
	if err != nil {
		return err
	}
	writeKeyToken(digest, label)
	writeKeyToken(digest, fmt.Sprintf("%d", info.Size()))
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	io.Copy(digest, f)
	writeKeyToken(digest, "")
	return nil
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

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
