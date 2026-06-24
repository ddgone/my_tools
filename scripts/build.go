package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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
	imageDir := filepath.Join(rootDir, "build", "image")
	hostDir := filepath.Join(imageDir, "host")
	runtimeDir := filepath.Join(rootDir, "build", "runtime")

	buildAll := false
	for _, a := range os.Args[1:] {
		if a == "-all" || a == "--all" {
			buildAll = true
		}
	}

	fmt.Println("====================================")
	fmt.Println("   火蜥蜴工具箱 Desktop - 构建脚本")
	fmt.Println("====================================")

	runGoVet(rootDir, appDir)

	os.RemoveAll(hostDir)
	os.RemoveAll(filepath.Join(appDir, "build"))
	os.MkdirAll(hostDir, 0755)
	os.MkdirAll(imageDir, 0755)

	cleanBuildRootStale(rootDir)

	initRuntimeDirs(runtimeDir)

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

	writeDefaultConfig(runtimeDir)

	copyAssets(filepath.Join(rootDir, "app", "assets"), filepath.Join(hostDir, "assets"))
	if err := buildBundledRustTools(rootDir, hostDir, buildAll); err != nil {
		fmt.Printf("\n❌ Rust 工具构建失败: %v\n", err)
		os.Exit(1)
	}

	// Go tools are only built for native platform (same as Rust native build).
	// Cross-compilation via GOOS/GOARCH is available but not used in default build.
	if err := buildBundledGoTools(rootDir, hostDir); err != nil {
		fmt.Printf("\n❌ Go 工具构建失败: %v\n", err)
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

func runGoVet(rootDir, appDir string) {
	fmt.Println("\ngo vet...")

	rootPackages := []string{"./libs/...", "./tools/...", "./scripts/..."}
	runVetCommand(rootDir, rootPackages, "根模块")
	runVetCommand(appDir, []string{"./..."}, "app 模块")
}

func runVetCommand(dir string, packages []string, label string) {
	args := append([]string{"vet"}, packages...)
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("⚠ %s go vet 失败: %v\n", label, err)
	}
}

func cleanBuildRootStale(rootDir string) {
	buildDir := filepath.Join(rootDir, "build")
	entries, err := os.ReadDir(buildDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		os.Remove(filepath.Join(buildDir, e.Name()))
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

type rustTool struct {
	ID       string
	CrateDir string
}

var bundledRustTools = []rustTool{
	{
		ID:       "las_voxelizer",
		CrateDir: filepath.Join("tools", "rust_tools", "las_voxelizer"),
	},
}

func buildBundledRustTools(rootDir, hostDir string, buildAll bool) error {
	if len(bundledRustTools) == 0 {
		return nil
	}

	targets := []Target{{OS: runtime.GOOS, Arch: runtime.GOARCH}}
	if buildAll {
		targets = allTargets
	}

	assetsRoot := filepath.Join(hostDir, "assets", "rust")
	if err := os.MkdirAll(assetsRoot, 0755); err != nil {
		return err
	}

	fmt.Println("\n构建 Rust 工具...")
	for _, tool := range bundledRustTools {
		crateDir := filepath.Join(rootDir, tool.CrateDir)
		for _, target := range targets {
			if !rustTargetSupported(target.OS, target.Arch, target.OS == runtime.GOOS && target.Arch == runtime.GOARCH) {
				fmt.Printf("⚠ 跳过 Rust 目标 %s/%s（当前未支持）\n", target.OS, target.Arch)
				continue
			}
			outputDir := filepath.Join(assetsRoot, target.OS+"_"+target.Arch)
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				return err
			}
			outputPath := filepath.Join(outputDir, rustBinaryFileName(tool.ID, target.OS))
			fmt.Printf("  Rust: %s -> %s/%s\n", tool.ID, target.OS, target.Arch)
			if err := buildBundledRustTool(crateDir, tool.ID, target.OS, target.Arch, outputPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func buildBundledRustTool(crateDir, toolID, targetOS, targetArch, outputPath string) error {
	cargoBinary, err := lookPathRequired("cargo", "未找到 Cargo，请先安装 Rust 工具链")
	if err != nil {
		return err
	}

	crateDirAbs, err := filepath.Abs(crateDir)
	if err != nil {
		return fmt.Errorf("解析 crate 目录绝对路径失败: %w", err)
	}

	wrapperDir, err := os.MkdirTemp("", toolID+"_bundled_rust_wrapper_")
	if err != nil {
		return fmt.Errorf("创建 Rust wrapper 临时目录失败: %w", err)
	}
	defer os.RemoveAll(wrapperDir)

	if err := writeRustWrapperCrate(wrapperDir, toolID, crateDirAbs); err != nil {
		return err
	}

	targetDir, err := os.MkdirTemp("", toolID+"_bundled_rust_target_")
	if err != nil {
		return fmt.Errorf("创建 Rust 临时构建目录失败: %w", err)
	}
	defer os.RemoveAll(targetDir)

	nativeBuild := targetOS == runtime.GOOS && targetArch == runtime.GOARCH
	var targetTriple string
	zigBinary := ""
	cargoZigbuildBinary := ""
	if !nativeBuild {
		targetTriple, err = rustTargetTriple(targetOS, targetArch)
		if err != nil {
			return err
		}
		zigBinary, err = lookPathRequiredWithFallback("zig", "未找到 zig，无法执行 Rust 交叉编译")
		if err != nil {
			return err
		}
		cargoZigbuildBinary, err = lookPathRequiredWithFallback("cargo-zigbuild", "未找到 cargo-zigbuild，无法执行 Rust 交叉编译")
		if err != nil {
			return err
		}
		rustupBinary, rustupErr := lookPathRequired("rustup", "未找到 rustup，无法准备交叉编译目标")
		if rustupErr != nil {
			return rustupErr
		}
		targetCmd := exec.Command(rustupBinary, "target", "add", targetTriple)
		targetCmd.Dir = crateDirAbs
		targetCmd.Stdout = os.Stdout
		targetCmd.Stderr = os.Stderr
		if err := targetCmd.Run(); err != nil {
			return fmt.Errorf("安装 Rust 目标失败: %w", err)
		}
	}

	args := []string{"build", "--release"}
	if !nativeBuild {
		args = []string{"zigbuild", "--release", "--target", targetTriple}
	}
	cmd := exec.Command(cargoBinary, args...)
	cmd.Dir = wrapperDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = rustBuildEnv(cargoBinary, zigBinary, cargoZigbuildBinary, targetDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("构建 Rust 工具失败: %w", err)
	}

	wrapperBinaryName := rustBinaryFileName("wrapper_"+toolID, targetOS)
	builtBinary := filepath.Join(targetDir, "release", wrapperBinaryName)
	if !nativeBuild {
		builtBinary = filepath.Join(targetDir, targetTriple, "release", wrapperBinaryName)
	}
	return copyBundledRustBinary(builtBinary, outputPath)
}

func writeRustWrapperCrate(wrapperDir string, toolID string, crateDirAbs string) error {
	srcDir := filepath.Join(wrapperDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		return fmt.Errorf("创建 wrapper src 目录失败: %w", err)
	}

	cargoToml := fmt.Sprintf(`[package]
name = "wrapper_%s"
version = "0.0.0"
edition = "2024"

[dependencies]
%s = { path = %q }
`, toolID, toolID, filepath.ToSlash(crateDirAbs))

	if err := os.WriteFile(filepath.Join(wrapperDir, "Cargo.toml"), []byte(cargoToml), 0644); err != nil {
		return fmt.Errorf("写入 wrapper Cargo.toml 失败: %w", err)
	}

	mainRs := fmt.Sprintf(`fn main() {
    let args: Vec<String> = std::env::args().skip(1).collect();
    if let Err(e) = %s::run(&args) {
        eprintln!("{e}");
        std::process::exit(1);
    }
}
`, toolID)

	if err := os.WriteFile(filepath.Join(srcDir, "main.rs"), []byte(mainRs), 0644); err != nil {
		return fmt.Errorf("写入 wrapper main.rs 失败: %w", err)
	}

	return nil
}

func rustTargetSupported(targetOS, targetArch string, nativeBuild bool) bool {
	if nativeBuild {
		return true
	}
	_, err := rustTargetTriple(targetOS, targetArch)
	return err == nil
}

func rustTargetTriple(targetOS, targetArch string) (string, error) {
	switch targetOS + "/" + targetArch {
	case "linux/amd64":
		return "x86_64-unknown-linux-musl", nil
	case "linux/arm64":
		return "aarch64-unknown-linux-musl", nil
	case "darwin/amd64":
		return "x86_64-apple-darwin", nil
	case "darwin/arm64":
		return "aarch64-apple-darwin", nil
	case "windows/amd64":
		return "x86_64-pc-windows-gnu", nil
	case "windows/arm64":
		return "aarch64-pc-windows-gnullvm", nil
	default:
		return "", fmt.Errorf("暂不支持构建 Rust 目标 %s/%s", targetOS, targetArch)
	}
}

func rustBinaryFileName(toolID, targetOS string) string {
	if targetOS == "windows" {
		return toolID + ".exe"
	}
	return toolID
}

func copyBundledRustBinary(srcPath, dstPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return err
	}
	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	return os.Chmod(dstPath, 0755)
}

func lookPathRequired(bin string, message string) (string, error) {
	return lookPathRequiredWithFallback(bin, message)
}

func lookPathRequiredWithFallback(bin string, message string) (string, error) {
	path, err := exec.LookPath(bin)
	if err != nil {
		if fallback, ok := fallbackToolPath(bin); ok {
			return fallback, nil
		}
		return "", errors.New(message)
	}
	return path, nil
}

func rustBuildEnv(cargoBinary string, zigBinary string, cargoZigbuildBinary string, targetDir string) []string {
	env := append([]string{}, os.Environ()...)
	env = appendOrReplaceEnvVar(env, "CARGO_TARGET_DIR", targetDir)
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
		if !pathVarContains(currentPath, binDir) {
			currentPath = binDir + string(os.PathListSeparator) + currentPath
		}
	}
	if currentPath != "" {
		return appendOrReplaceEnvVar(env, "PATH", currentPath)
	}
	return env
}

func fallbackToolPath(bin string) (string, bool) {
	if fallback, ok := fallbackCargoToolPath(bin); ok {
		return fallback, true
	}
	for _, candidate := range commonToolFallbacks(bin) {
		info, statErr := os.Stat(candidate)
		if statErr == nil && !info.IsDir() {
			return candidate, true
		}
	}
	return "", false
}

func fallbackCargoToolPath(bin string) (string, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	candidate := filepath.Join(home, ".cargo", "bin", rustToolExecutableName(bin))
	info, statErr := os.Stat(candidate)
	return candidate, statErr == nil && !info.IsDir()
}

func commonToolFallbacks(bin string) []string {
	executable := rustToolExecutableName(bin)
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join("/opt/homebrew/bin", executable),
		filepath.Join("/usr/local/bin", executable),
	}
	if home != "" {
		candidates = append(candidates,
			filepath.Join(home, ".local", "bin", executable),
			filepath.Join(home, "bin", executable),
		)
	}
	return candidates
}

func rustToolExecutableName(bin string) string {
	if runtime.GOOS == "windows" {
		return bin + ".exe"
	}
	return bin
}

func appendOrReplaceEnvVar(env []string, key string, value string) []string {
	prefix := key + "="
	for i, item := range env {
		if strings.HasPrefix(item, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

func pathVarContains(currentPath string, candidate string) bool {
	for _, item := range filepath.SplitList(currentPath) {
		if filepath.Clean(item) == filepath.Clean(candidate) {
			return true
		}
	}
	return false
}

type goTool struct {
	ID          string
	SourceEntry string
}

var bundledGoTools = []goTool{
	{ID: "echo_tool", SourceEntry: "tools/go_tools/echo_tool/tool.go"},
	{ID: "geojson_to_shp", SourceEntry: "tools/go_tools/geojson_to_shapefile/tool.go"},
	{ID: "hdfs_download", SourceEntry: "tools/go_tools/hdfs_download/tool.go"},
	{ID: "pos2gis_converter", SourceEntry: "tools/go_tools/pos_trajectory_to_gis/tool.go"},
	{ID: "recursive_content_dir_diff", SourceEntry: "tools/go_tools/recursive_content_dir_diff/tool.go"},
	{ID: "utm_geojson_converter", SourceEntry: "tools/go_tools/utm_extract_to_gis/tool.go"},
}

func buildBundledGoTools(rootDir, hostDir string) error {
	if len(bundledGoTools) == 0 {
		return nil
	}

	assetsRoot := filepath.Join(hostDir, "assets", "go", runtime.GOOS+"_"+runtime.GOARCH)
	if err := os.MkdirAll(assetsRoot, 0755); err != nil {
		return err
	}

	goBinary, err := lookPathRequired("go", "未找到 Go，请先安装 Go 工具链")
	if err != nil {
		return err
	}

	fmt.Println("\n构建 Go 工具...")
	for _, tool := range bundledGoTools {
		sourcePath := filepath.Join(rootDir, filepath.FromSlash(tool.SourceEntry))
		outputPath := filepath.Join(assetsRoot, goBinaryFileName(tool.ID))
		fmt.Printf("  Go: %s\n", tool.ID)

		if err := buildBundledGoTool(rootDir, goBinary, sourcePath, outputPath); err != nil {
			return err
		}
	}
	return nil
}

func buildBundledGoTool(rootDir, goBinary, sourcePath, outputPath string) error {
	importPath := goImportPath(sourcePath, rootDir)

	tmpDir, err := os.MkdirTemp("", "go_tool_bundle_")
	if err != nil {
		return fmt.Errorf("创建 Go 临时目录失败: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	wrapperPath := filepath.Join(tmpDir, "main.go")
	wrapper := goWrapperMain(importPath)
	if err := os.WriteFile(wrapperPath, []byte(wrapper), 0644); err != nil {
		return fmt.Errorf("写入 Go wrapper 失败: %w", err)
	}

	cmd := exec.Command(goBinary, "build", "-o", outputPath, wrapperPath)
	cmd.Dir = rootDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0",
		"GOOS="+runtime.GOOS,
		"GOARCH="+runtime.GOARCH,
	)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("构建 Go 工具失败: %w", err)
	}
	return nil
}

func goImportPath(sourceEntry string, rootDir string) string {
	rel, err := filepath.Rel(rootDir, sourceEntry)
	if err != nil {
		return "my_tools/tools/go_tools/unknown"
	}
	dir := filepath.ToSlash(filepath.Dir(rel))
	return "my_tools/" + dir
}

func goWrapperMain(importPath string) string {
	pkgName := filepath.Base(importPath)
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

func goBinaryFileName(toolID string) string {
	if runtime.GOOS == "windows" {
		return toolID + ".exe"
	}
	return toolID
}
