package bxn_point_cloud_renew

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// fileExists 判断路径存在且是文件。
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// dirExists 判断路径存在且是目录。
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// ensureUniqueDir 在 baseDir 下为 name 找一个不冲突的目录名，冲突时追加 _1、_2 等后缀。
func ensureUniqueDir(baseDir, name string) string {
	candidate := name
	if _, err := os.Stat(filepath.Join(baseDir, candidate)); os.IsNotExist(err) {
		return candidate
	}
	for i := 1; ; i++ {
		candidate = fmt.Sprintf("%s_%d", name, i)
		if _, err := os.Stat(filepath.Join(baseDir, candidate)); os.IsNotExist(err) {
			return candidate
		}
	}
}

// copyFile 复制单个文件。
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
