package main

import (
	"fmt"
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

	fmt.Println("\ngo vet...")
	vet := exec.Command("go", "vet", "./...")
	vet.Dir = rootDir
	vet.Stdout = os.Stdout
	vet.Stderr = os.Stderr
	vet.Run()

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

	fmt.Println("\n====================================")
	fmt.Printf("✅ 产物: %s\n", hostDir)
	entries, _ := os.ReadDir(hostDir)
	for _, e := range entries {
		fmt.Printf("   %s\n", e.Name())
	}
	fmt.Println("====================================")
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
