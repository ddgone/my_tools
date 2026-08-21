package bxn_point_cloud_renew

import (
	"encoding/json"
	"os"
)

// trajStat 单个轨迹文件的处理统计。
type trajStat struct {
	OldLines int `json:"old_lines"`
	NewLines int `json:"new_lines"`
	Replaced int `json:"replaced"`
	Deleted  int `json:"deleted"`
	Skipped  int `json:"skipped"`
}

// recordInfo 单条记录的完整信息（台账条目）。
type recordInfo struct {
	Index         int                 `json:"index"`
	OldPkgDir     string              `json:"old_pkg_dir"`
	NewProjDir    string              `json:"new_proj_dir"`
	PkgName       string              `json:"pkg_name"`
	ProjectID     string              `json:"project_id"`
	FrameProjID   string              `json:"frame_project_id"`
	FrameFilterID []string            `json:"frame_filter_ids,omitempty"`
	FieldTaskID   string              `json:"field_task_id"`
	MatchedGroup  string              `json:"matched_group"`
	OldTarPath    string              `json:"old_tar_path"`
	NewTarPath    string              `json:"new_tar_path"`
	Status        string              `json:"status"` // pending/validated/running/done/failed
	HasWarnings   bool                `json:"has_warnings,omitempty"`
	Warnings      []string            `json:"warnings,omitempty"`
	OutputDir     string              `json:"output_dir,omitempty"`
	EffectivePkg  string              `json:"effective_pkg,omitempty"`
	TrajStats     map[string]trajStat `json:"trajectory_stats,omitempty"`
	CloudStat     *cloudStat          `json:"cloud_stats,omitempty"`
	Error         string              `json:"error,omitempty"`
	StartedAt     string              `json:"started_at,omitempty"`
	FinishedAt    string              `json:"finished_at,omitempty"`
	ElapsedSec    float64             `json:"elapsed_sec,omitempty"`

	// 运行时内部字段（不序列化到台账）
	frames []taskFrame `json:"-"`
}

// ledger 台账文件结构。
type ledger struct {
	Version   int           `json:"version"`
	OutputDir string        `json:"output_dir"`
	Records   []*recordInfo `json:"records"`
}

// loadLedger 读取台账文件，文件缺失或损坏时返回 nil。
func loadLedger(path string) *ledger {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var l ledger
	if err := json.Unmarshal(data, &l); err != nil {
		return nil
	}
	return &l
}

// saveLedger 保存台账文件。
func saveLedger(path string, l *ledger) {
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(path, data, 0644)
}

func countDone(l *ledger) int {
	n := 0
	for _, r := range l.Records {
		if r.Status == "done" {
			n++
		}
	}
	return n
}

func countFailed(l *ledger) int {
	n := 0
	for _, r := range l.Records {
		if r.Status == "failed" {
			n++
		}
	}
	return n
}

func countWarn(l *ledger) int {
	n := 0
	for _, r := range l.Records {
		if r.HasWarnings {
			n++
		}
	}
	return n
}
