package bxn_point_cloud_renew

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// cloudStat 点云与视频帧同步统计。
type cloudStat struct {
	OldFrames  int      `json:"old_frames"`
	NewFrames  int      `json:"new_frames"`
	Replaced   int      `json:"replaced"`
	Deleted    int      `json:"deleted"`
	Kept       int      `json:"kept"`
	ReplacedTS []string `json:"replaced_ts,omitempty"`
	DeletedTS  []string `json:"deleted_ts,omitempty"`
	KeptTS     []string `json:"kept_ts,omitempty"`
}

// syncDeskewCloud 按轨迹匹配结果同步 deskew_cloud 目录：
//   - pcd：框内交集且新包存在 → 用新包替换；框内老包独有 → 删除；其余保留
//   - 视频帧目录 <ts>/：仅做删除（框内老包独有）和保留，不替换（视频未修改）
func syncDeskewCloud(oldDeskewDir, newDeskewDir string, res frameMatchResult) (cloudStat, error) {
	var stat cloudStat

	entries, err := os.ReadDir(oldDeskewDir)
	if err != nil {
		return stat, fmt.Errorf("read old deskew dir failed: %w", err)
	}

	// 新包 pcd 是否存在
	newPcdSet := make(map[string]bool)
	if newEntries, err := os.ReadDir(newDeskewDir); err == nil {
		for _, e := range newEntries {
			name := e.Name()
			if strings.HasSuffix(name, ".pcd") {
				newPcdSet[strings.TrimSuffix(name, ".pcd")] = true
			}
		}
	}
	stat.NewFrames = len(newPcdSet)

	deletedSet := make(map[string]bool)
	for _, ts := range res.deletedTS {
		deletedSet[ts] = true
	}
	replaceSet := make(map[string]bool)
	for _, ts := range res.replacedTS {
		replaceSet[ts] = true
	}

	for _, e := range entries {
		name := e.Name()

		// 只处理 pcd 文件（每个 pcd 对应一个时间戳帧）
		if !strings.HasSuffix(name, ".pcd") {
			continue
		}
		ts := strings.TrimSuffix(name, ".pcd")
		stat.OldFrames++

		tsDir := filepath.Join(oldDeskewDir, ts)

		if deletedSet[ts] {
			// 删除 pcd 和视频帧目录
			os.Remove(filepath.Join(oldDeskewDir, name))
			os.RemoveAll(tsDir)
			stat.Deleted++
			stat.DeletedTS = append(stat.DeletedTS, ts)
			continue
		}

		// 保留帧：若框内交集且新包有 pcd，则替换点云
		if replaceSet[ts] && newPcdSet[ts] {
			srcPcd := filepath.Join(newDeskewDir, ts+".pcd")
			if err := copyFile(srcPcd, filepath.Join(oldDeskewDir, name)); err == nil {
				stat.Replaced++
				stat.ReplacedTS = append(stat.ReplacedTS, ts)
			} else {
				return stat, fmt.Errorf("replace pcd %s failed: %w", ts, err)
			}
		}

		stat.Kept++
		stat.KeptTS = append(stat.KeptTS, ts)
	}

	return stat, nil
}
