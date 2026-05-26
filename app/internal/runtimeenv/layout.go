package runtimeenv

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const envRuntimeDir = "FIRE_SALAMANDER_RUNTIME_DIR"

type Layout struct {
	Root string
}

func ResolveLayout() (Layout, error) {
	root, err := detectRuntimeRoot()
	if err != nil {
		return Layout{}, err
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		return Layout{}, fmt.Errorf("创建运行时目录失败: %w", err)
	}
	return Layout{Root: root}, nil
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

func detectRuntimeRoot() (string, error) {
	if override := stringsTrimSpace(os.Getenv(envRuntimeDir)); override != "" {
		return filepath.Clean(override), nil
	}

	if repoRoot, ok := findRepoRootFromWorkingDir(); ok {
		return filepath.Join(repoRoot, "build", "runtime"), nil
	}

	home, err := userHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".fire-salamander"), nil
}

func findRepoRootFromWorkingDir() (string, bool) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", false
	}

	dir := filepath.Clean(cwd)
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
