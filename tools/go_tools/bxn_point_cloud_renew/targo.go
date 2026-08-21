package bxn_point_cloud_renew

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// extractTarGz 解压 tar.gz 到 destDir，带路径穿越防护。
func extractTarGz(tarPath, destDir string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return fmt.Errorf("open tar.gz failed: %w", err)
	}
	defer f.Close()

	gzReader, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip reader failed: %w", err)
	}
	defer gzReader.Close()

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("mkdir dest failed: %w", err)
	}

	tarReader := tar.NewReader(gzReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar entry failed: %w", err)
		}

		targetPath := filepath.Join(destDir, header.Name)
		cleanTarget := filepath.Clean(targetPath)
		cleanDest := filepath.Clean(destDir) + string(os.PathSeparator)
		if !strings.HasPrefix(cleanTarget, cleanDest) {
			return fmt.Errorf("illegal tar path: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("mkdir failed: %w", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("mkdir parent failed: %w", err)
			}
			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("create file failed: %w", err)
			}
			_, copyErr := io.Copy(outFile, tarReader)
			outFile.Close()
			if copyErr != nil {
				return fmt.Errorf("write file failed: %w", copyErr)
			}
		}
	}
	return nil
}

// createTarGz 把 srcDir 整体打包为 tar.gz。
func createTarGz(srcDir, tarPath string) error {
	outFile, err := os.Create(tarPath)
	if err != nil {
		return fmt.Errorf("create output failed: %w", err)
	}
	defer outFile.Close()

	gzWriter := gzip.NewWriter(outFile)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("rel path failed: %w", err)
		}
		if relPath == "." {
			return nil
		}
		relPath = filepath.ToSlash(relPath)

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("tar header failed: %w", err)
		}
		header.Name = relPath

		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("write tar header failed: %w", err)
		}
		if info.IsDir() {
			return nil
		}

		srcFile, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open src failed: %w", err)
		}
		_, copyErr := io.Copy(tarWriter, srcFile)
		srcFile.Close()
		if copyErr != nil {
			return fmt.Errorf("write tar content failed: %w", copyErr)
		}
		return nil
	})
}
