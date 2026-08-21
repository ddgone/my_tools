package bxn_point_cloud_renew

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// validateRecord Phase 1 预检查：验证目录/文件存在、提取 field_task_id、匹配新包、加载任务框。
func validateRecord(index int, oldPkgDir, newProjDir, frameProjID string, frameFilterIDs []string, framesDir string, frameCache map[string][]taskFrame, out io.Writer) *recordInfo {
	info := &recordInfo{
		Index:         index,
		OldPkgDir:     oldPkgDir,
		NewProjDir:    newProjDir,
		FrameProjID:   frameProjID,
		FrameFilterID: frameFilterIDs,
		PkgName:       filepath.Base(oldPkgDir),
		Status:        "pending",
	}

	var errs []string
	addErr := func(format string, args ...interface{}) {
		errs = append(errs, fmt.Sprintf(format, args...))
	}
	fail := func() *recordInfo {
		info.Status = "failed"
		info.Error = strings.Join(errs, "; ")
		return info
	}

	// 检查老包目录
	if !dirExists(oldPkgDir) {
		addErr("old package dir not found: %s", oldPkgDir)
		return fail()
	}

	// 解析时间戳
	targetSec, err := packageToUnixSec(info.PkgName)
	if err != nil {
		addErr("timestamp parse: %v", err)
		return fail()
	}

	// 老项目日志
	oldProjDir := filepath.Dir(filepath.Dir(oldPkgDir))
	info.ProjectID = filepath.Base(oldProjDir)
	logPath := filepath.Join(oldProjDir, "logs", "tum_pos_exporter.INFO")
	if !fileExists(logPath) {
		addErr("log file not found: %s", logPath)
		return fail()
	}

	// 提取 field_task_id
	fieldTaskID, err := findFieldTaskID(logPath, targetSec)
	if err != nil {
		addErr("field_task_id lookup: %v", err)
		return fail()
	}
	info.FieldTaskID = fieldTaskID

	// 老包 tar.gz
	oldTarPath := filepath.Join(oldPkgDir, "process_result_0.tar.gz")
	if !fileExists(oldTarPath) {
		addErr("old tar.gz not found: %s", oldTarPath)
		return fail()
	}
	info.OldTarPath = oldTarPath

	// 新项目 debug 目录
	debugDir := filepath.Join(newProjDir, "debug")
	if !dirExists(debugDir) {
		addErr("new project debug dir not found: %s", debugDir)
		return fail()
	}

	// 搜索匹配组
	matchedGroup, err := findMatchingGroup(debugDir, fieldTaskID)
	if err != nil {
		addErr("match new package: %v", err)
		return fail()
	}
	info.MatchedGroup = matchedGroup

	// 新包 tar.gz
	outSourceGroup := strings.TrimPrefix(matchedGroup, "group_")
	newTarPath := filepath.Join(newProjDir, "out_source", outSourceGroup, "process_result_0.tar.gz")
	if !fileExists(newTarPath) {
		addErr("new tar.gz not found: %s", newTarPath)
		return fail()
	}
	info.NewTarPath = newTarPath

	// 加载任务框（projectId 缓存优先/在线拉取，或 geojson 路径离线读取），带内存缓存
	frames, err := loadFrames(frameProjID, framesDir, frameCache, out)
	if err != nil {
		addErr("load frames failed: %v", err)
		return fail()
	}
	// 第四列框 ID 过滤：只保留指定框，其余查询结果作废
	frames, err = filterFramesByID(frames, frameFilterIDs)
	if err != nil {
		addErr("frame filter failed: %v", err)
		return fail()
	}
	info.frames = frames

	info.Status = "validated"
	return info
}

// executeRecord Phase 2 执行：解压、框内匹配替换轨迹、同步点云、重新打包。
func executeRecord(ctx context.Context, info *recordInfo, workDir, outputDir string, logPath string, out io.Writer) {
	t0 := time.Now()
	info.StartedAt = t0.Format(time.RFC3339)
	info.Status = "running"

	var logWriter io.Writer = out
	var logFile *os.File
	if logPath != "" {
		os.MkdirAll(filepath.Dir(logPath), 0755)
		var err error
		logFile, err = os.Create(logPath)
		if err == nil {
			logWriter = io.MultiWriter(out, logFile)
		}
	}
	defer func() {
		if logFile != nil {
			logFile.Close()
		}
	}()

	logf := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		if !strings.HasSuffix(msg, "\n") {
			msg += "\n"
		}
		logWriter.Write([]byte(msg))
	}

	// consolef 只输出到控制台（不写日志文件）
	consolef := func(format string, args ...interface{}) {
		fmt.Fprintf(out, format, args...)
		if !strings.HasSuffix(format, "\n") {
			fmt.Fprintln(out)
		}
	}

	// detailf 只写入日志文件（全量明细，不进控制台）；无文件时丢弃
	detailf := func(format string, args ...interface{}) {
		if logFile != nil {
			fmt.Fprintf(logFile, format, args...)
		}
	}

	// logDetailList 输出一组时间戳列表：摘要进控制台+文件，明细全量进文件，控制台仅截断展示
	logDetailList := func(title, marker string, tsList []string) {
		logf("[INFO] %s: %d", title, len(tsList))
		if len(tsList) == 0 {
			return
		}
		for _, ts := range tsList {
			detailf("  %s %s\n", marker, ts)
		}
		const consoleLimit = 10
		for i := 0; i < len(tsList) && i < consoleLimit; i++ {
			consolef("  %s %s", marker, tsList[i])
		}
		if len(tsList) > consoleLimit {
			consolef("  ... and %d more (see log file)", len(tsList)-consoleLimit)
		}
	}

	logf("[INFO] Package: %s (project=%s, task=%s, matched=%s)",
		info.PkgName, info.ProjectID, info.FieldTaskID, info.MatchedGroup)

	cancelled := func() bool {
		select {
		case <-ctx.Done():
			info.Error = "任务已被取消"
			info.Status = "failed"
			info.FinishedAt = time.Now().Format(time.RFC3339)
			return true
		default:
			return false
		}
	}

	// Step 1: extract old
	oldExtractDir := filepath.Join(workDir, "old")
	logf("[INFO] Extract old: %s", info.OldTarPath)
	t1 := time.Now()
	if err := extractTarGz(info.OldTarPath, oldExtractDir); err != nil {
		info.Error = fmt.Sprintf("extract old failed: %v", err)
		info.Status = "failed"
		info.FinishedAt = time.Now().Format(time.RFC3339)
		return
	}
	logf("[INFO]   done in %.1fs", time.Since(t1).Seconds())
	if cancelled() {
		return
	}

	// Step 2: extract new
	newExtractDir := filepath.Join(workDir, "new")
	logf("[INFO] Extract new: %s", info.NewTarPath)
	t2 := time.Now()
	if err := extractTarGz(info.NewTarPath, newExtractDir); err != nil {
		info.Error = fmt.Sprintf("extract new failed: %v", err)
		info.Status = "failed"
		info.FinishedAt = time.Now().Format(time.RFC3339)
		return
	}
	logf("[INFO]   done in %.1fs", time.Since(t2).Seconds())
	if cancelled() {
		return
	}

	// Step 3: 框内匹配替换轨迹
	oldProcessDir := filepath.Join(oldExtractDir, "process_result_0")
	newProcessDir := filepath.Join(newExtractDir, "process_result_0")
	info.TrajStats = make(map[string]trajStat)

	logf("[INFO] Frames loaded: %d", len(info.frames))
	stats, res, err := replaceByFrames(oldProcessDir, newProcessDir, info.frames)
	if err != nil {
		info.Error = fmt.Sprintf("trajectory replace failed: %v", err)
		info.Status = "failed"
		info.FinishedAt = time.Now().Format(time.RFC3339)
		return
	}
	info.TrajStats = stats

	for fname, stat := range stats {
		logf("[INFO] %s: old=%d new=%d replaced=%d deleted=%d skipped=%d",
			fname, stat.OldLines, stat.NewLines, stat.Replaced, stat.Deleted, stat.Skipped)
	}

	// 替换的点（框内交集）：摘要+控制台截断，明细全量进文件
	logDetailList("Replaced points (inside frames, in both)", "+", res.replacedTS)

	// 删除的点（框内老包独有）
	logDetailList("Deleted points (old-only inside frames)", "-", res.deletedTS)

	// 框外保留的点
	logDetailList("Kept points (outside frames)", "=", res.keptTS)

	// 新包独有被跳过的点
	if len(res.skippedTS) > 0 {
		info.HasWarnings = true
		logf("[WARN] Skipped points (new-only, not in old): %d", len(res.skippedTS))
		for _, ts := range res.skippedTS {
			detailf("  x %s\n", ts)
		}
		const consoleLimit = 10
		for i := 0; i < len(res.skippedTS) && i < consoleLimit; i++ {
			consolef("  x %s", res.skippedTS[i])
		}
		if len(res.skippedTS) > consoleLimit {
			consolef("  ... and %d more (see log file)", len(res.skippedTS)-consoleLimit)
		}
	}

	// Step 3.5: 同步点云与视频帧（与轨迹一一对应）
	oldDeskewDir := filepath.Join(oldProcessDir, "deskew_cloud")
	newDeskewDir := filepath.Join(newProcessDir, "deskew_cloud")
	if dirExists(oldDeskewDir) {
		logf("[INFO] Sync deskew_cloud ...")
		cstat, cerr := syncDeskewCloud(oldDeskewDir, newDeskewDir, res)
		if cerr != nil {
			info.Error = fmt.Sprintf("sync deskew failed: %v", cerr)
			info.Status = "failed"
			info.FinishedAt = time.Now().Format(time.RFC3339)
			return
		}
		info.CloudStat = &cstat
		logf("[INFO] deskew_cloud: old=%d new=%d replaced=%d deleted=%d kept=%d",
			cstat.OldFrames, cstat.NewFrames, cstat.Replaced, cstat.Deleted, cstat.Kept)
		logDetailList("  Replaced pcd", "+", cstat.ReplacedTS)
		logDetailList("  Deleted frames (pcd + video)", "-", cstat.DeletedTS)
		logDetailList("  Kept frames (pcd + video)", "=", cstat.KeptTS)
	} else {
		logf("[INFO] No deskew_cloud dir in old package, skip cloud sync")
	}

	// Step 4: pack output（有新包独有点的包路由到 problem 子目录）
	hasProblem := len(res.skippedTS) > 0

	var baseDir string
	if hasProblem {
		baseDir = filepath.Join(outputDir, "problem", info.ProjectID, "out_source")
		info.HasWarnings = true
	} else {
		baseDir = filepath.Join(outputDir, info.ProjectID, "out_source")
	}
	info.EffectivePkg = ensureUniqueDir(baseDir, info.PkgName)
	info.OutputDir = filepath.Join(baseDir, info.EffectivePkg)

	if err := os.MkdirAll(info.OutputDir, 0755); err != nil {
		info.Error = fmt.Sprintf("mkdir output failed: %v", err)
		info.Status = "failed"
		info.FinishedAt = time.Now().Format(time.RFC3339)
		return
	}

	outTarPath := filepath.Join(info.OutputDir, "process_result_0.tar.gz")
	logf("[INFO] Pack: %s", outTarPath)
	t3 := time.Now()
	if err := createTarGz(oldExtractDir, outTarPath); err != nil {
		info.Error = fmt.Sprintf("pack failed: %v", err)
		info.Status = "failed"
		info.FinishedAt = time.Now().Format(time.RFC3339)
		return
	}
	logf("[INFO]   done in %.1fs", time.Since(t3).Seconds())

	// 写入问题日志（有新包独有点被跳过的包）
	if hasProblem {
		problemLogPath := filepath.Join(baseDir, info.EffectivePkg+".log")
		if pf, err := os.Create(problemLogPath); err == nil {
			fmt.Fprintf(pf, "Problem report for package: %s\n", info.PkgName)
			fmt.Fprintf(pf, "Project: %s\n", info.ProjectID)
			fmt.Fprintf(pf, "Frame project id: %s\n", info.FrameProjID)
			fmt.Fprintf(pf, "field_task_id: %s\n", info.FieldTaskID)
			fmt.Fprintf(pf, "Matched new group: %s\n", info.MatchedGroup)
			fmt.Fprintf(pf, "Processed at: %s\n\n", time.Now().Format(time.RFC3339))
			fmt.Fprintf(pf, "Frame-based matching summary:\n")
			for fname, s := range info.TrajStats {
				fmt.Fprintf(pf, "  %s: old=%d new=%d replaced=%d deleted=%d skipped=%d\n",
					fname, s.OldLines, s.NewLines, s.Replaced, s.Deleted, s.Skipped)
			}
			if len(res.deletedTS) > 0 {
				fmt.Fprintf(pf, "\nDeleted points (old-only inside frames), %d total:\n", len(res.deletedTS))
				for _, ts := range res.deletedTS {
					fmt.Fprintf(pf, "  %s\n", ts)
				}
			}
			if len(res.skippedTS) > 0 {
				fmt.Fprintf(pf, "\nSkipped points (new-only, not in old), %d total:\n", len(res.skippedTS))
				for _, ts := range res.skippedTS {
					fmt.Fprintf(pf, "  %s\n", ts)
				}
				info.Warnings = append(info.Warnings, fmt.Sprintf("new-only points skipped: %d", len(res.skippedTS)))
			}
			pf.Close()
			logf("[WARN] Problem log: %s", problemLogPath)
		}
	}

	info.Status = "done"
	info.FinishedAt = time.Now().Format(time.RFC3339)
	info.ElapsedSec = time.Since(t0).Seconds()
	info.Error = ""

	logf("[INFO] Total elapsed: %.1fs", info.ElapsedSec)
}
