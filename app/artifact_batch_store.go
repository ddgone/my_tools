package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"fire-salamander-desktop/internal/runtimeenv"
)

func artifactBatchTasksFilePath(layout runtimeenv.Layout) string {
	return filepath.Join(layout.ConfigDir(), artifactBatchTasksFileName)
}

func loadArtifactBatchTasksFile(filePath string) ([]*ArtifactBatchTask, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("读取产物任务持久化文件失败: %w", err)
	}

	var tasks []*ArtifactBatchTask
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, fmt.Errorf("解析产物任务持久化文件失败: %w", err)
	}
	return normalizePersistedArtifactTasks(tasks), nil
}

func saveArtifactBatchTasksFile(filePath string, tasks []*ArtifactBatchTask) error {
	normalized := normalizePersistedArtifactTasks(tasks)
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("创建产物任务持久化目录失败: %w", err)
	}
	data, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化产物任务持久化文件失败: %w", err)
	}
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("写入产物任务持久化文件失败: %w", err)
	}
	return nil
}

func normalizePersistedArtifactTasks(tasks []*ArtifactBatchTask) []*ArtifactBatchTask {
	normalized := make([]*ArtifactBatchTask, 0, len(tasks))
	for _, task := range tasks {
		if task == nil || strings.TrimSpace(task.ID) == "" {
			continue
		}
		copyTask := cloneArtifactTask(task)
		recountArtifactTask(copyTask)
		normalized = append(normalized, copyTask)
	}
	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i].StartedAt > normalized[j].StartedAt
	})
	if len(normalized) > maxArtifactBatchTaskHistory {
		normalized = normalized[:maxArtifactBatchTaskHistory]
	}
	return normalized
}

func sameArtifactFile(targetPath string, cachePath string) (bool, error) {
	targetPath = strings.TrimSpace(targetPath)
	cachePath = strings.TrimSpace(cachePath)
	if targetPath == "" || cachePath == "" {
		return false, nil
	}
	if _, err := os.Stat(targetPath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	targetDigest, err := digestFile(targetPath)
	if err != nil {
		return false, err
	}
	cacheDigest, err := digestFile(cachePath)
	if err != nil {
		return false, err
	}
	return targetDigest == cacheDigest, nil
}

func digestFile(path string) ([32]byte, error) {
	var zero [32]byte
	file, err := os.Open(path)
	if err != nil {
		return zero, err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return zero, err
	}
	var sum [32]byte
	copy(sum[:], digest.Sum(nil))
	return sum, nil
}
