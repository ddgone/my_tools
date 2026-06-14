package runtimeenv

import (
	"os"
	"path/filepath"
)

func FindBundledAssetsDir() (string, bool) {
	if repoRoot, ok := FindRepoRoot(); ok {
		candidate := filepath.Join(repoRoot, "build", "image", "host", "assets")
		if dirExists(candidate) {
			return candidate, true
		}
	}

	executablePath, err := os.Executable()
	if err != nil {
		return "", false
	}
	if resolved, resolveErr := filepath.EvalSymlinks(executablePath); resolveErr == nil {
		executablePath = resolved
	}

	for dir := filepath.Dir(executablePath); ; dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, "assets")
		if dirExists(candidate) {
			return candidate, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
	}
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
