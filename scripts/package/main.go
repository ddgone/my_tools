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
	imageDir := filepath.Join(rootDir, "build", "image")
	outputDir := filepath.Join(rootDir, "build", "exports")

	if _, err := os.Stat(imageDir); os.IsNotExist(err) {
		fmt.Println("❌ 未找到构建产物，请先运行 go run scripts/build.go")
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

	if err := packageZip(imageDir, outFile); err != nil {
		fmt.Printf("\n❌ 打包失败: %v\n", err)
		os.Exit(1)
	}

	info, _ := os.Stat(outFile)
	fmt.Printf("\n✅ 安装包已生成: %s (%.1f MB)\n", outFile, float64(info.Size())/1024/1024)
	fmt.Println("====================================")
}

func packageZip(srcDir, dstFile string) error {
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
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

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
