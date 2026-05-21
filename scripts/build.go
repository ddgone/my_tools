package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// Target defines a compilation target
type Target struct {
	OS   string
	Arch string
}

var targets = []Target{
	{"windows", "amd64"},
	{"windows", "arm64"},
	{"linux", "amd64"},
	{"linux", "arm64"},
	{"darwin", "amd64"},
	{"darwin", "arm64"},
}

func main() {
	fmt.Println("====================================")
	fmt.Println("    开始跨平台编译 my_tools")
	fmt.Println("====================================")

	outDir := "build"
	if err := os.MkdirAll(outDir, 0755); err != nil {
		fmt.Printf("无法创建输出目录: %v\n", err)
		os.Exit(1)
	}

	start := time.Now()
	successCount := 0

	for _, target := range targets {
		ext := ""
		if target.OS == "windows" {
			ext = ".exe"
		}

		outName := fmt.Sprintf("my_tools_%s_%s%s", target.OS, target.Arch, ext)
		outPath := filepath.Join(outDir, outName)

		fmt.Printf("编译 %s/%s -> %s ... ", target.OS, target.Arch, outName)

		cmd := exec.Command("go", "build", "-o", outPath, "main.go")

		// Set environment variables for cross-compilation
		env := os.Environ()
		env = append(env, "CGO_ENABLED=0") // Ensure static linking for better portability
		env = append(env, "GOOS="+target.OS)
		env = append(env, "GOARCH="+target.Arch)
		cmd.Env = env

		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("失败!\n错误信息:\n%s\n", string(output))
		} else {
			fmt.Printf("成功\n")
			successCount++
		}
	}

	elapsed := time.Since(start)
	fmt.Println("====================================")
	fmt.Printf("编译完成! 耗时: %v\n", elapsed)
	fmt.Printf("成功: %d/%d\n", successCount, len(targets))
	fmt.Printf("输出目录: %s\n", outDir)

	if runtime.GOOS == "windows" {
		fmt.Println("提示: 你可以随时运行 `go run scripts/build.go` 来重新编译。")
	} else {
		fmt.Println("提示: 你可以随时运行 `go run scripts/build.go` 来重新编译。")
	}

	fmt.Println("\n⚠️ [注意] Linux 平台背景音乐(BGM)说明:")
	fmt.Println("  当前通过交叉编译生成的 Linux 版本是不带音频支持的（静音版）。")
	fmt.Println("  如果你需要在 Linux 下听到 BGM，必须在 Linux 真机上进行原生编译：")
	fmt.Println("    1. 安装 ALSA 依赖: sudo apt-get install libasound2-dev")
	fmt.Println("    2. 原生编译: CGO_ENABLED=1 go build -o my_tools_linux .")
}
