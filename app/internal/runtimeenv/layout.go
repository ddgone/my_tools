package runtimeenv

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const envRuntimeDir = "FIRE_SALAMANDER_RUNTIME_DIR"

type Layout struct {
	// Root 是可变数据根（config/logs/exports/cache 均在它下面）。
	// ProgramRoot 是只读程序根（program/），只有便携部署态才非空。
	Root        string
	ProgramRoot string
}

func ResolveLayout() (Layout, error) {
	layout, err := detectRuntimeLayout()
	if err != nil {
		return Layout{}, err
	}
	if err := os.MkdirAll(layout.Root, 0755); err != nil {
		return Layout{}, fmt.Errorf("创建运行时目录失败: %w", err)
	}
	return layout, nil
}

func (l Layout) CacheDir() string {
	return filepath.Join(l.Root, "cache")
}

func (l Layout) BuildCacheDir() string {
	return filepath.Join(l.CacheDir(), "builds")
}

func (l Layout) ScriptCacheDir() string {
	return filepath.Join(l.CacheDir(), "scripts")
}

func (l Layout) LogsDir() string {
	return filepath.Join(l.Root, "logs")
}

func (l Layout) ExportsDir() string {
	return filepath.Join(l.Root, "exports")
}

func (l Layout) ConfigDir() string {
	return filepath.Join(l.Root, "config")
}

// ProgramToolsDir 返回便携部署态下预置编译产物的只读目录（program/tools）。
// 仅便携部署态（ProgramRoot 非空）有效；非便携态返回空串，由调用方回退到构建缓存。
func (l Layout) ProgramToolsDir() string {
	if l.ProgramRoot == "" {
		return ""
	}
	return filepath.Join(l.ProgramRoot, "tools")
}

func (l Layout) Ensure() error {
	dirs := []string{
		l.Root,
		l.CacheDir(),
		l.BuildCacheDir(),
		l.ScriptCacheDir(),
		l.LogsDir(),
		l.ExportsDir(),
		l.ConfigDir(),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建运行时子目录失败 %s: %w", dir, err)
		}
	}
	return nil
}

func detectRuntimeLayout() (Layout, error) {
	if root, programRoot, ok := detectPortableLayout(); ok {
		return Layout{Root: dataRootFromPortable(root), ProgramRoot: programRoot}, nil
	}

	if override := stringsTrimSpace(os.Getenv(envRuntimeDir)); override != "" {
		return Layout{Root: filepath.Clean(override)}, nil
	}

	if repoRoot, ok := FindRepoRoot(); ok {
		return Layout{
			Root:        filepath.Join(repoRoot, "build", "data"),
			ProgramRoot: filepath.Join(repoRoot, "build", "program"),
		}, nil
	}

	home, err := userHomeDir()
	if err != nil {
		return Layout{}, err
	}
	return Layout{Root: filepath.Join(home, ".fire-salamander")}, nil
}

// detectPortableLayout 检测便携部署态：宿主 exe 旁同时存在 program/ 与 data/ 目录标记。
func detectPortableLayout() (root string, programRoot string, ok bool) {
	executablePath, err := os.Executable()
	if err != nil {
		return "", "", false
	}
	if resolved, resolveErr := filepath.EvalSymlinks(executablePath); resolveErr == nil {
		executablePath = resolved
	}
	dir := filepath.Dir(executablePath)
	programDir := filepath.Join(dir, "program")
	if !isDir(programDir) || !isDir(filepath.Join(dir, "data")) {
		return "", "", false
	}
	return dir, programDir, true
}

func dataRootFromPortable(root string) string {
	return filepath.Join(root, "data")
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func FindRepoRoot() (string, bool) {
	if repoRoot, ok := findRepoRootFromWorkingDir(); ok {
		return repoRoot, true
	}
	return findRepoRootFromExecutable()
}

func findRepoRootFromWorkingDir() (string, bool) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", false
	}
	return findRepoRootFromPath(cwd)
}

func findRepoRootFromExecutable() (string, bool) {
	executablePath, err := os.Executable()
	if err != nil {
		return "", false
	}
	if resolved, resolveErr := filepath.EvalSymlinks(executablePath); resolveErr == nil {
		executablePath = resolved
	}
	return findRepoRootFromPath(filepath.Dir(executablePath))
}

func findRepoRootFromPath(start string) (string, bool) {
	dir := filepath.Clean(start)
	for {
		if fileExists(filepath.Join(dir, "go.work")) && fileExists(filepath.Join(dir, "app", "wails.json")) {
			return dir, true
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func userHomeDir() (string, error) {
	if runtime.GOOS == "windows" {
		if home := stringsTrimSpace(os.Getenv("USERPROFILE")); home != "" {
			return home, nil
		}
	}
	if home := stringsTrimSpace(os.Getenv("HOME")); home != "" {
		return home, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("解析用户目录失败: %w", err)
	}
	return home, nil
}

func stringsTrimSpace(value string) string {
	for len(value) > 0 && (value[0] == ' ' || value[0] == '\t' || value[0] == '\n' || value[0] == '\r') {
		value = value[1:]
	}
	for len(value) > 0 {
		last := value[len(value)-1]
		if last != ' ' && last != '\t' && last != '\n' && last != '\r' {
			break
		}
		value = value[:len(value)-1]
	}
	return value
}
