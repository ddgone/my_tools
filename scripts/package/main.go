package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func main() {
	rootDir, _ := os.Getwd()
	buildDir := filepath.Join(rootDir, "build")
	outputDir := filepath.Join(rootDir, "build", "exports")

	if _, err := os.Stat(buildDir); os.IsNotExist(err) {
		fmt.Println("❌ 未找到构建产物 build/，请先运行 go run scripts/build.go")
		os.Exit(1)
	}

	version := "dev"
	buildTime := time.Now().Format("20060102-150405")
	for _, a := range os.Args[1:] {
		if !strings.HasPrefix(a, "-") {
			version = a
		}
	}

	os.MkdirAll(outputDir, 0755)

	outFile := filepath.Join(outputDir, fmt.Sprintf("fire-salamander-%s-%s-%s.zip",
		version, runtime.GOOS, runtime.GOARCH))

	fmt.Println("====================================")
	fmt.Println("   火蜥蜴工具箱 Desktop - 打包脚本")
	fmt.Println("====================================")
	fmt.Printf("\n版本: %s\n", version)
	fmt.Printf("构建时间: %s\n", buildTime)
	fmt.Printf("目标: %s\n", outFile)

	if err := packageZipDir(buildDir, outFile); err != nil {
		fmt.Printf("\n❌ 打包失败: %v\n", err)
		os.Exit(1)
	}

	info, _ := os.Stat(outFile)
	fmt.Printf("\n✅ 安装包已生成: %s (%.1f MB)\n", outFile, float64(info.Size())/1024/1024)
	fmt.Println("====================================")
}

// packageZipDir 把 build/ 目录打成一个 zip（zip 根即 build/ 内容，交付展开即目录形态）。
// data/ 下的用户数据（设置、缓存、使用记录等）不进包，仅保留目录骨架；
// 顶层 exports/ 为打包输出目录，与运行时临时 sqlite 文件一并排除。
func packageZipDir(srcDir, dstFile string) error {
	outDir := filepath.Dir(dstFile)
	outAbs, err := filepath.Abs(dstFile)
	if err != nil {
		return err
	}

	f, err := os.Create(dstFile)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, rerr := filepath.Rel(srcDir, path)
		if rerr != nil {
			return rerr
		}
		relPath = filepath.ToSlash(relPath)
		if relPath == "." {
			return nil
		}

		// 顶层 exports 为打包输出目录，不进包
		if relPath == "exports" {
			return filepath.SkipDir
		}

		// data/ 为用户可写数据目录（设置、缓存、使用记录、托管工具链等均为
		// 运行时生成），不进包；应用启动时 Ensure() 自建所需子目录，
		// 包内仅保留顶层空目录占位。
		if relPath == "data" {
			if !info.IsDir() {
				return nil
			}
			if _, derr := zw.Create("data/"); derr != nil {
				return derr
			}
			return filepath.SkipDir
		}

		absPath, aerr := filepath.Abs(path)
		if aerr == nil {
			// 不打包输出目录及其中的 zip 文件自身
			if filepath.Clean(filepath.Dir(absPath)) == filepath.Clean(outDir) {
				return nil
			}
			if filepath.Clean(absPath) == filepath.Clean(outAbs) {
				return nil
			}
		}
		if info.IsDir() {
			return nil
		}

		// 排除运行时 sqlite 临时文件（-shm / -wal）等
		if strings.HasSuffix(relPath, "-shm") || strings.HasSuffix(relPath, "-wal") || strings.HasSuffix(relPath, ".tmp") {
			return nil
		}

		w, err := zw.Create(relPath)
		if err != nil {
			return err
		}

		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()

		_, err = io.Copy(w, src)
		return err
	})
}
