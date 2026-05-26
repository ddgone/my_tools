package builder

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type ToolKind string

const (
	KindGo      ToolKind = "go"
	KindPython  ToolKind = "python"
	envGoBinary          = "FIRE_SALAMANDER_GO_BIN"
)

type BuildRequest struct {
	ToolID      string
	ToolName    string
	Kind        ToolKind
	OutputDir   string
	RepoRoot    string
	SourceEntry string
	TargetOS    string
	TargetArch  string
}

func BuildPackage(req BuildRequest) (string, error) {
	if err := os.MkdirAll(req.OutputDir, 0755); err != nil {
		return "", fmt.Errorf("创建输出目录失败: %w", err)
	}

	switch req.Kind {
	case KindPython:
		return buildPythonPackage(req)
	case KindGo:
		return buildGoPackage(req)
	default:
		return "", fmt.Errorf("不支持的工具类型: %s", req.Kind)
	}
}

func buildPythonPackage(req BuildRequest) (string, error) {
	scriptPath := req.SourceEntry
	if scriptPath == "" {
		return "", fmt.Errorf("工具 %s 缺少 Python 脚本入口", req.ToolID)
	}
	if !filepath.IsAbs(scriptPath) {
		if req.RepoRoot == "" {
			return "", fmt.Errorf("工具 %s 缺少仓库根目录，无法解析脚本入口", req.ToolID)
		}
		scriptPath = filepath.Join(req.RepoRoot, filepath.FromSlash(scriptPath))
	}
	if _, err := os.Stat(scriptPath); err != nil {
		return "", fmt.Errorf("Python 脚本不存在: %w", err)
	}

	outFile := filepath.Join(req.OutputDir, req.ToolID+".py")
	if err := copyFile(scriptPath, outFile, 0644); err != nil {
		return "", fmt.Errorf("复制 Python 脚本失败: %w", err)
	}

	return outFile, nil
}

func buildGoPackage(req BuildRequest) (string, error) {
	if req.SourceEntry == "" {
		return "", fmt.Errorf("工具 %s 缺少 Go 源码入口", req.ToolID)
	}
	if req.RepoRoot == "" {
		return "", fmt.Errorf("工具 %s 缺少仓库根目录，无法生成单工具产物", req.ToolID)
	}

	targetOS := req.TargetOS
	if targetOS == "" {
		targetOS = runtime.GOOS
	}
	targetArch := req.TargetArch
	if targetArch == "" {
		targetArch = runtime.GOARCH
	}

	importPath := buildImportPath(req.SourceEntry)
	buildDir := filepath.Join(req.OutputDir, req.ToolID+"_"+targetOS+"_"+targetArch+"_src")
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return "", fmt.Errorf("创建构建目录失败: %w", err)
	}

	wrapperPath := filepath.Join(buildDir, "main.go")
	if err := os.WriteFile(wrapperPath, []byte(renderGoWrapper(req.ToolID, importPath)), 0644); err != nil {
		return "", fmt.Errorf("写入构建入口失败: %w", err)
	}

	outputName := req.ToolID + "_" + targetOS + "_" + targetArch
	if targetOS == "windows" {
		outputName += ".exe"
	}
	outFile := filepath.Join(req.OutputDir, outputName)

	goBinary, err := resolveGoBinary()
	if err != nil {
		return "", err
	}

	cmd := exec.Command(goBinary, "build", "-o", outFile, wrapperPath)
	cmd.Dir = req.RepoRoot
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0",
		"GOOS="+targetOS,
		"GOARCH="+targetArch,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("构建单工具产物失败: %w\n%s", err, strings.TrimSpace(string(output)))
	}

	if err := os.Chmod(outFile, 0755); err != nil {
		return "", fmt.Errorf("设置可执行权限失败: %w", err)
	}

	return outFile, nil
}

func buildImportPath(sourceEntry string) string {
	normalized := filepath.ToSlash(strings.TrimSpace(sourceEntry))
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

	if path, err := exec.LookPath("go"); err == nil && path != "" {
		return path, nil
	}

	for _, candidate := range candidateGoBinaryPaths() {
		if isExecutableFile(candidate) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("构建单工具产物失败: 未找到 Go 工具链；请将 `go` 加入 PATH，或设置 `%s` 指向 go 可执行文件", envGoBinary)
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

	addGOROOTCandidate(runtime.GOROOT())
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

func copyFile(srcPath, dstPath string, mode os.FileMode) error {
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
