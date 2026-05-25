package builder

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type ToolKind string

const (
	KindGo     ToolKind = "go"
	KindPython ToolKind = "python"
)

type BuildRequest struct {
	ToolID    string
	ToolName  string
	Kind      ToolKind
	OutputDir string
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
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	toolsDir := filepath.Join(cwd, "tools")
	entries, err := os.ReadDir(toolsDir)
	if err != nil {
		return "", fmt.Errorf("读取tools目录失败: %w", err)
	}

	var pkgDir string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(toolsDir, entry.Name())
		files, _ := os.ReadDir(dir)
		for _, f := range files {
			if !f.IsDir() && strings.HasSuffix(f.Name(), ".py") {
				content, err := os.ReadFile(filepath.Join(dir, f.Name()))
				if err != nil {
					continue
				}
				if strings.Contains(string(content), req.ToolID) {
					pkgDir = dir
					break
				}
			}
		}
		if pkgDir != "" {
			break
		}
	}

	outFile := filepath.Join(req.OutputDir, req.ToolID+".tar.gz")
	if pkgDir == "" {
		f, err := os.Create(outFile)
		if err != nil {
			return "", err
		}
		f.Close()
		return outFile, fmt.Errorf("未找到工具 %s 的Python脚本目录", req.ToolID)
	}

	if err := createTarGz(pkgDir, outFile); err != nil {
		return "", fmt.Errorf("打包失败: %w", err)
	}

	return outFile, nil
}

func buildGoPackage(req BuildRequest) (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("获取当前程序路径失败: %w", err)
	}

	outFile := filepath.Join(req.OutputDir, req.ToolID+"_executable")
	src, err := os.Open(exePath)
	if err != nil {
		return "", fmt.Errorf("打开程序文件失败: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(outFile)
	if err != nil {
		return "", fmt.Errorf("创建输出文件失败: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("复制程序文件失败: %w", err)
	}

	if err := os.Chmod(outFile, 0755); err != nil {
		return "", fmt.Errorf("设置可执行权限失败: %w", err)
	}

	return outFile, nil
}

func createTarGz(srcDir, outFile string) error {
	f, err := os.Create(outFile)
	if err != nil {
		return err
	}
	defer f.Close()

	gzw := gzip.NewWriter(f)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		link := ""
		if info.Mode()&os.ModeSymlink != 0 {
			link, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}

		hdr, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}
		hdr.Name = relPath

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}

		if info.IsDir() || link != "" {
			return nil
		}

		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()

		_, err = io.Copy(tw, src)
		return err
	})
}
