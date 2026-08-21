package bxn_point_cloud_renew

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// cliArgs 命令行参数。
type cliArgs struct {
	oldDir      string
	newDir      string
	frameProjID string
	frameIDs    string
	inputPath   string
	outputDir   string
	framesDir   string
	resume      bool
}

// parseCLIArgs 解析并校验命令行参数。
func parseCLIArgs(args []string, out io.Writer) (cliArgs, error) {
	var a cliArgs
	fs := flag.NewFlagSet("bxn_point_cloud_renew", flag.ContinueOnError)
	fs.SetOutput(out)

	fs.StringVar(&a.oldDir, "old", "", "老包目录（单包模式，具体到包）")
	fs.StringVar(&a.newDir, "new", "", "新项目目录（单包模式，项目级别）")
	fs.StringVar(&a.frameProjID, "frame-project-id", "", "新项目的纯数字项目 id（单包模式，如 60410257）")
	fs.StringVar(&a.frameIDs, "frame-ids", "", "可选框 ID 过滤（单包模式，逗号分隔；填写后只使用指定框）")
	fs.StringVar(&a.inputPath, "input", "", "批量模式输入文件：csv/txt/xls/xlsx，每行 old_pkg_dir,new_proj_dir,frame_project_id[,frame_ids]")
	fs.StringVar(&a.outputDir, "output", "", "输出根目录")
	fs.StringVar(&a.framesDir, "frames", "", "任务框缓存目录：缓存命中则离线读取，缺失则在线拉取并保存")
	fs.BoolVar(&a.resume, "resume", false, "断点续传，跳过台账中已完成的记录")

	if err := fs.Parse(args); err != nil {
		return a, err
	}
	if err := a.validate(); err != nil {
		return a, err
	}
	return a, nil
}

func (a cliArgs) validate() error {
	if a.outputDir == "" {
		return fmt.Errorf("必须指定 -output 参数")
	}
	hasSingle := a.oldDir != "" || a.newDir != ""
	hasInput := a.inputPath != ""
	if hasSingle && hasInput {
		return fmt.Errorf("-old/-new 与 -input 不能同时使用")
	}
	if !hasSingle && !hasInput {
		return fmt.Errorf("必须指定 -old 和 -new（单包模式），或 -input（批量模式）")
	}
	if hasInput {
		return nil
	}
	// 单包模式
	if a.oldDir == "" || a.newDir == "" {
		return fmt.Errorf("单包模式必须同时指定 -old 和 -new")
	}
	if a.frameProjID == "" {
		return fmt.Errorf("单包模式必须指定 -frame-project-id")
	}
	return nil
}

// Run 工具入口：批量模式（-input）或单包模式（-old/-new）。
func Run(ctx context.Context, args []string, out io.Writer) error {
	a, err := parseCLIArgs(args, out)
	if err != nil {
		return err
	}

	if a.inputPath != "" {
		return runBatch(ctx, a, out)
	}
	return runSingle(ctx, a, out)
}

func checkCancelled(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("任务已被取消")
	default:
		return nil
	}
}

// runSingle 单包模式：验证并处理一条记录。
func runSingle(ctx context.Context, args cliArgs, out io.Writer) error {
	workDir, err := os.MkdirTemp("", "bxn_pcr_extract_")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(workDir)

	frameCache := map[string][]taskFrame{}
	info := validateRecord(1, args.oldDir, args.newDir, args.frameProjID,
		parseFrameFilterIDs(args.frameIDs), args.framesDir, frameCache, out)
	if info.Error != "" {
		return fmt.Errorf("预检查失败: %s", info.Error)
	}
	fmt.Fprintf(out, "[Phase 1] Validated: pkg=%s project=%s field_task_id=%s matched=%s\n",
		info.PkgName, info.ProjectID, info.FieldTaskID, info.MatchedGroup)

	// 单包模式也写日志文件，保证全量明细可溯源
	logPath := ""
	logDir := filepath.Join(args.outputDir, "log")
	if err := os.MkdirAll(logDir, 0755); err == nil {
		logPath = filepath.Join(logDir, "single_"+info.PkgName+".log")
	}
	executeRecord(ctx, info, workDir, args.outputDir, logPath, out)
	if info.Status != "done" {
		return fmt.Errorf("%s", info.Error)
	}
	if info.HasWarnings {
		fmt.Fprintf(out, "\n========== Done (with warnings) ==========\n")
		for _, w := range info.Warnings {
			fmt.Fprintf(out, "  WARN: %s\n", w)
		}
	} else {
		fmt.Fprintf(out, "\n========== Done ==========\n")
	}
	fmt.Fprintf(out, "Output: %s\n", info.OutputDir)
	return nil
}

// runBatch 批量模式：读取输入文件，两阶段处理全部记录。
func runBatch(ctx context.Context, args cliArgs, out io.Writer) error {
	records, err := readInputFile(args.inputPath, out)
	if err != nil {
		return fmt.Errorf("读取输入文件失败: %w", err)
	}
	if len(records) == 0 {
		return fmt.Errorf("输入文件中没有有效记录")
	}

	// 前置拉框：--frames 缓存目录里缺失的 projectId 统一拉取并保存。
	// 不依赖记录有效性（第一/二列无效也按 projectId 拉缓存，方便本地备好缓存后离线运行）
	if err := prefetchFrames(ctx, records, args.framesDir, out); err != nil {
		return fmt.Errorf("预拉任务框失败: %w", err)
	}

	logDir := filepath.Join(args.outputDir, "log")
	ledgerPath := filepath.Join(logDir, "ledger.json")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}

	// 加载已有台账
	var l *ledger
	if args.resume {
		l = loadLedger(ledgerPath)
		if l != nil {
			fmt.Fprintf(out, "[RESUME] Loaded ledger with %d records\n", len(l.Records))
		}
	}
	if l == nil {
		l = &ledger{Version: 1, OutputDir: args.outputDir}
	}

	// 构建索引：已有记录的 index -> record 映射
	existingByIndex := make(map[int]*recordInfo)
	for _, rec := range l.Records {
		existingByIndex[rec.Index] = rec
	}

	// Phase 1 从头跑所有记录，但跳过已完成的
	fmt.Fprintf(out, "========== Phase 1: Validation (%d records) ==========\n", len(records))
	phase1Start := time.Now()

	var newRecords []*recordInfo
	needsExec := 0
	skipped := 0
	frameCache := map[string][]taskFrame{}

	for i, r := range records {
		if err := checkCancelled(ctx); err != nil {
			return err
		}
		idx := i + 1

		if existing, ok := existingByIndex[idx]; ok && existing.Status == "done" && args.resume {
			newRecords = append(newRecords, existing)
			fmt.Fprintf(out, "  [%d/%d] %s -> SKIP (already done)\n", idx, len(records), filepath.Base(r[0]))
			skipped++
			continue
		}

		// 已存在但未完成（之前失败或中断），重新验证
		frameFilter := ""
		if len(r) >= 4 {
			frameFilter = r[3]
		}
		info := validateRecord(idx, r[0], r[1], r[2], parseFrameFilterIDs(frameFilter), args.framesDir, frameCache, out)
		newRecords = append(newRecords, info)

		if info.Error != "" {
			fmt.Fprintf(out, "  [%d/%d] %s -> FAIL: %s\n", idx, len(records), info.PkgName, info.Error)
			continue
		}
		fmt.Fprintf(out, "  [%d/%d] %s -> OK (project=%s, task=%s, matched=%s, frames=%d)\n",
			idx, len(records), info.PkgName, info.ProjectID, info.FieldTaskID, info.MatchedGroup, len(info.frames))
		needsExec++
	}

	l.Records = newRecords
	saveLedger(ledgerPath, l)

	fmt.Fprintf(out, "[Phase 1] Done in %.1fs. %d valid, %d skipped, %d failed, %d total.\n",
		time.Since(phase1Start).Seconds(), needsExec, skipped, countFailed(l), len(l.Records))

	if needsExec == 0 {
		fmt.Fprintf(out, "Nothing to execute.\n")
		return nil
	}

	// ---------- Phase 2: 执行 ----------
	fmt.Fprintf(out, "\n========== Phase 2: Execution (%d records) ==========\n", needsExec)
	phase2Start := time.Now()

	workDir, err := os.MkdirTemp("", "bxn_pcr_extract_")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(workDir)

	summaryPath := filepath.Join(logDir, "_summary.log")
	summaryFile, err := os.Create(summaryPath)
	if err != nil {
		return fmt.Errorf("创建汇总日志失败: %w", err)
	}
	defer summaryFile.Close()

	summaryLog := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		fmt.Fprint(out, msg)
		summaryFile.WriteString(msg)
		summaryFile.Sync()
	}

	runIdx := 0
	for _, info := range l.Records {
		if err := checkCancelled(ctx); err != nil {
			return err
		}
		if info.Status != "validated" {
			if info.Status == "done" && args.resume {
				runIdx++
				summaryLog("  [%d/%d] %s -> SKIP (resume)\n", runIdx, needsExec, info.PkgName)
			}
			continue
		}

		runIdx++
		summaryLog("\n[%d/%d] %s\n", runIdx, needsExec, info.PkgName)

		os.RemoveAll(workDir)
		logPath := filepath.Join(logDir, fmt.Sprintf("%03d_%s.log", info.Index, info.PkgName))
		executeRecord(ctx, info, workDir, args.outputDir, logPath, out)

		// 更新台账
		saveLedger(ledgerPath, l)

		if info.Status == "done" {
			tag := "[OK]"
			if info.HasWarnings {
				tag = "[OK/WARN]"
			}
			summaryLog("  %s -> %s (%.1fs)\n", tag, info.OutputDir, info.ElapsedSec)
			if info.TrajStats != nil {
				for fname, s := range info.TrajStats {
					summaryLog("       %s: old=%d new=%d replaced=%d deleted=%d skipped=%d\n",
						fname, s.OldLines, s.NewLines, s.Replaced, s.Deleted, s.Skipped)
				}
			}
		} else {
			summaryLog("  [FAIL] %s\n", info.Error)
		}
		os.RemoveAll(workDir)

		// 执行后重命名日志以反映实际包名
		if info.EffectivePkg != "" && info.EffectivePkg != info.PkgName {
			newLogPath := filepath.Join(logDir, fmt.Sprintf("%03d_%s.log", info.Index, info.EffectivePkg))
			os.Rename(logPath, newLogPath)
		}
	}

	done := countDone(l)
	fail := countFailed(l)
	warn := countWarn(l)
	summaryLog("\n========== Batch done (%.1fs) ==========\n", time.Since(phase2Start).Seconds())
	summaryLog("Success: %d, Warnings: %d, Failed: %d, Total: %d\n", done-warn, warn, fail, len(l.Records))
	return nil
}

// prefetchFrames 前置拉框缓存：对 --frames 缓存目录中缺失的 projectId 统一拉取并保存。
// framesDir 为空时跳过（在线模式，验证阶段按需拉取）。不依赖记录有效性。
func prefetchFrames(ctx context.Context, records [][]string, framesDir string, out io.Writer) error {
	if framesDir == "" {
		return nil
	}
	if err := os.MkdirAll(framesDir, 0755); err != nil {
		return fmt.Errorf("mkdir frames dir failed: %w", err)
	}

	seen := make(map[string]bool)
	for _, r := range records {
		if err := checkCancelled(ctx); err != nil {
			return err
		}
		spec := ""
		if len(r) >= 3 {
			spec = r[2]
		}
		if spec == "" || !isProjectID(spec) || seen[spec] {
			continue
		}
		seen[spec] = true

		cachePath := filepath.Join(framesDir, spec+".geojson")
		if fileExists(cachePath) {
			fmt.Fprintf(out, "[FRAMES] %s -> cached (%s)\n", spec, cachePath)
			continue
		}
		fmt.Fprintf(out, "[FRAMES] Fetching frames for project %s ...\n", spec)
		frames, err := fetchTaskFrames(spec)
		if err != nil {
			return fmt.Errorf("fetch frames for project %s failed: %w", spec, err)
		}
		if err := saveFramesToGeojson(frames, cachePath); err != nil {
			return fmt.Errorf("save frames for %s failed: %w", spec, err)
		}
		fmt.Fprintf(out, "[FRAMES] Project %s: %d frames -> %s\n", spec, len(frames), cachePath)
	}
	return nil
}
