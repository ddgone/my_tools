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
	outDir := filepath.Join(rootDir, "build")

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

	os.RemoveAll(outDir)
	os.RemoveAll(filepath.Join(appDir, "build"))
	os.MkdirAll(outDir, 0755)

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

		// move all output to root build/
		srcBin := filepath.Join(appDir, "build", "bin")
		if entries, readErr := os.ReadDir(srcBin); readErr == nil {
			for _, e := range entries {
				os.Rename(filepath.Join(srcBin, e.Name()), filepath.Join(outDir, e.Name()))
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

		// move output to root build/
		srcBin := filepath.Join(appDir, "build", "bin")
		if entries, readErr := os.ReadDir(srcBin); readErr == nil {
			for _, e := range entries {
				os.Rename(filepath.Join(srcBin, e.Name()), filepath.Join(outDir, e.Name()))
			}
		}
		os.RemoveAll(filepath.Join(appDir, "build"))
	}

	if runtime.GOOS == "darwin" {
		if entries, err := os.ReadDir(outDir); err == nil {
			for _, e := range entries {
				if strings.HasSuffix(e.Name(), ".app") {
					appPath := filepath.Join(outDir, e.Name())
					exec.Command("xattr", "-d", "com.apple.quarantine", appPath).Run()
					exec.Command("xattr", "-cr", appPath).Run()
				}
			}
		}
	}

	fmt.Println("\n====================================")
	fmt.Printf("✅ 产物: %s\n", outDir)
	entries, _ := os.ReadDir(outDir)
	for _, e := range entries {
		fmt.Printf("   %s\n", e.Name())
	}
	fmt.Println("====================================")
}
