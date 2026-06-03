package recursive_content_dir_diff

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"my_tools/libs/framework"
)

// FileInfo 存储文件的相对路径和绝对路径
type FileInfo struct {
	RelPath string
	AbsPath string
	Size    int64
}

// Result 比较结果
type Result struct {
	RelPath string
	Diff    string
}

const defaultWorkers = 4
const defaultDiffDisplayLimit = 50

type RecursiveContentDirDiffTool struct{}

func (t *RecursiveContentDirDiffTool) ID() string       { return "recursive_content_dir_diff" }
func (t *RecursiveContentDirDiffTool) Name() string     { return "递归目录内容比对" }
func (t *RecursiveContentDirDiffTool) Category() string { return "通用测试工具 > 文件操作" }

// shouldIgnore 判断给定的相对路径（或基名）是否应该被忽略。
func shouldIgnore(ignorePatterns []string, relPath, baseName string) bool {
	for _, pattern := range ignorePatterns {
		// 如果 pattern 包含路径分隔符，视为路径匹配（前缀或精确）
		if strings.ContainsAny(pattern, string(filepath.Separator)) {
			// 清理 pattern 确保没有首尾分隔符
			cleanPattern := strings.Trim(pattern, string(filepath.Separator))
			cleanRel := strings.Trim(relPath, string(filepath.Separator))
			// 匹配：relPath 以 pattern 开头，或者完全相等
			if strings.HasPrefix(cleanRel, cleanPattern) &&
				(len(cleanRel) == len(cleanPattern) || cleanRel[len(cleanPattern)] == filepath.Separator) {
				return true
			}
			// 如果 relPath 为空且 pattern 也为空？不可能
		} else {
			// 简单名称匹配：匹配当前项的名称
			if baseName == pattern {
				return true
			}
		}
	}
	return false
}

// collect 递归遍历 root，收集未被忽略的目录和文件。
func collect(ctx context.Context, root string, ignorePatterns []string, progressMsg chan<- string) (dirs map[string]bool, files map[string]FileInfo, err error) {
	dirs = make(map[string]bool)
	files = make(map[string]FileInfo)
	root = filepath.Clean(root)

	var count int32
	done := make(chan struct{})

	// 扫描进度显示
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				c := atomic.LoadInt32(&count)
				if c > 0 {
					progressMsg <- fmt.Sprintf("正在扫描 %s ... 已发现 %d 个文件", filepath.Base(root), c)
				} else {
					progressMsg <- fmt.Sprintf("正在扫描 %s ...", filepath.Base(root))
				}
			}
		}
	}()

	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			rel = ""
		}
		base := d.Name()
		isDir := d.IsDir()

		// 检查是否忽略
		if shouldIgnore(ignorePatterns, rel, base) {
			if isDir {
				return filepath.SkipDir
			}
			return nil
		}

		if isDir {
			dirs[rel] = true
		} else {
			atomic.AddInt32(&count, 1)
			info, err := d.Info()
			if err != nil {
				return err
			}
			files[rel] = FileInfo{
				RelPath: rel,
				AbsPath: path,
				Size:    info.Size(),
			}
		}
		return nil
	})

	close(done)

	if err != nil {
		return nil, nil, err
	}
	progressMsg <- fmt.Sprintf("扫描完成！%s 中发现 %d 个目录，%d 个文件",
		filepath.Base(root), len(dirs), len(files))
	return dirs, files, nil
}

// compareFiles 比较两个文件内容。
func compareFiles(ctx context.Context, path1, path2 string) (bool, error) {
	f1, err := os.Open(path1)
	if err != nil {
		return false, err
	}
	defer f1.Close()
	f2, err := os.Open(path2)
	if err != nil {
		return false, err
	}
	defer f2.Close()

	const chunkSize = 64 * 1024
	buf1 := make([]byte, chunkSize)
	buf2 := make([]byte, chunkSize)

	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
		}

		n1, err1 := f1.Read(buf1)
		n2, err2 := f2.Read(buf2)

		if err1 != nil || err2 != nil {
			if err1 == io.EOF && err2 == io.EOF {
				return true, nil
			}
			if err1 == io.EOF || err2 == io.EOF {
				return false, nil
			}
			return false, fmt.Errorf("read error: %v, %v", err1, err2)
		}
		if n1 != n2 {
			return false, nil
		}
		for i := 0; i < n1; i++ {
			if buf1[i] != buf2[i] {
				return false, nil
			}
		}
	}
}

// compareDirs 比较两个目录。
func compareDirs(ctx context.Context, dir1, dir2 string, concurrency int, ignorePatterns []string, showProgress bool, out io.Writer) ([]string, error) {
	dir1 = filepath.Clean(dir1)
	dir2 = filepath.Clean(dir2)

	progressChan := make(chan string, 100)
	if showProgress {
		defer close(progressChan)
		go func() {
			for msg := range progressChan {
				fmt.Fprintln(out, msg)
			}
		}()
	}

	dirs1, files1, err := collect(ctx, dir1, ignorePatterns, progressChan)
	if err != nil {
		return nil, fmt.Errorf("读取目录 %s 失败: %v", dir1, err)
	}
	dirs2, files2, err := collect(ctx, dir2, ignorePatterns, progressChan)
	if err != nil {
		return nil, fmt.Errorf("读取目录 %s 失败: %v", dir2, err)
	}

	if showProgress {
		progressChan <- "正在分析目录差异..."
	}

	var diffs []string
	// 目录差异
	for d := range dirs1 {
		if !dirs2[d] {
			display := d
			if display == "" {
				display = "(根目录)"
			}
			diffs = append(diffs, fmt.Sprintf("仅存在于 %s 的目录: %s", dir1, display))
		}
	}
	for d := range dirs2 {
		if !dirs1[d] {
			display := d
			if display == "" {
				display = "(根目录)"
			}
			diffs = append(diffs, fmt.Sprintf("仅存在于 %s 的目录: %s", dir2, display))
		}
	}
	// 文件差异
	for rel := range files1 {
		if _, ok := files2[rel]; !ok {
			diffs = append(diffs, fmt.Sprintf("仅存在于 %s 的文件: %s", dir1, rel))
		}
	}
	for rel := range files2 {
		if _, ok := files1[rel]; !ok {
			diffs = append(diffs, fmt.Sprintf("仅存在于 %s 的文件: %s", dir2, rel))
		}
	}

	// 共同文件
	type commonFile struct {
		rel string
		f1  FileInfo
		f2  FileInfo
	}
	common := make([]commonFile, 0, len(files1))
	for rel, f1 := range files1 {
		if f2, ok := files2[rel]; ok {
			common = append(common, commonFile{rel, f1, f2})
		}
	}

	if len(common) > 0 && showProgress {
		progressChan <- fmt.Sprintf("开始比较 %d 个共同文件...", len(common))
	}

	if len(common) > 0 {
		tasks := make(chan commonFile, len(common))
		results := make(chan Result, len(common))
		var wg sync.WaitGroup
		var completed int32
		var lastProgressLog int32

		workerCount := concurrency
		if workerCount <= 0 {
			workerCount = runtime.NumCPU()
		}
		for i := 0; i < workerCount; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for task := range tasks {
					select {
					case <-ctx.Done():
						results <- Result{RelPath: task.rel, Diff: ""}
						return
					default:
					}

					var diff string
					if task.f1.Size != task.f2.Size {
						diff = fmt.Sprintf("文件大小不同: %s (%d vs %d)", task.rel, task.f1.Size, task.f2.Size)
					} else {
						equal, err := compareFiles(ctx, task.f1.AbsPath, task.f2.AbsPath)
						if err != nil {
							diff = fmt.Sprintf("读取文件错误 %s: %v", task.rel, err)
						} else if !equal {
							diff = fmt.Sprintf("文件内容不同: %s", task.rel)
						}
					}
					results <- Result{RelPath: task.rel, Diff: diff}
					if showProgress {
						doneCount := atomic.AddInt32(&completed, 1)
						total := int32(len(common))
						// 只在关键节点输出，避免日志刷屏。
						shouldLog := doneCount == total || doneCount == 1
						if !shouldLog && total >= 10 {
							step := total / 10
							if step == 0 {
								step = 1
							}
							if doneCount%step == 0 && atomic.LoadInt32(&lastProgressLog) != doneCount {
								atomic.StoreInt32(&lastProgressLog, doneCount)
								shouldLog = true
							}
						}
						if shouldLog {
							progressChan <- fmt.Sprintf("文件比较进度: %d/%d", doneCount, total)
						}
					}
				}
			}()
		}

		for _, t := range common {
			tasks <- t
		}
		close(tasks)
		go func() {
			wg.Wait()
			close(results)
		}()
		for res := range results {
			if res.Diff != "" {
				diffs = append(diffs, res.Diff)
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("任务已取消")
	}
	sort.Strings(diffs)
	return diffs, nil
}

func (t *RecursiveContentDirDiffTool) Execute(ctx framework.AppContext) {
	usage := `递归目录内容比对工具

说明:
递归比较两个目录是否完全一致，检查目录结构、文件名、文件大小和文件内容。

参数:
  -dir-a <目录路径>        对比目录A
  -dir-b <目录路径>        对比目录B
  -workers <数量>          并发 worker 数量。默认 4
  -ignore <规则列表>       忽略规则。支持重复传入，也支持逗号/分号/换行分隔
  -no-progress             禁用扫描和比较进度输出
  -show-all-diffs          显示全部差异，不再限制输出条数

兼容旧命令行:
  仍支持直接传两个位置参数，例如：
  "D:\a" "D:\b"

兼容旧参数:
  -left / -right 仍可使用，但推荐改用 -dir-a / -dir-b

忽略规则示例:
  -ignore logs
  -ignore logs/access.log
  -ignore "logs,cache,temp/output"
`

	ctx.ShowTerminal(t.Name(), usage, func(runCtx context.Context, args string, out io.Writer) error {
		parsedArgs, err := framework.ParseArgs(args)
		if err != nil {
			return err
		}

		fs := flag.NewFlagSet("recursive_content_dir_diff", flag.ContinueOnError)
		fs.SetOutput(out)

		var dirA string
		var dirB string
		var concurrency int
		var noProgress bool
		var showAllDiffs bool
		var ignores strSliceFlag

		fs.StringVar(&dirA, "dir-a", "", "")
		fs.StringVar(&dirB, "dir-b", "", "")
		fs.StringVar(&dirA, "left", "", "")
		fs.StringVar(&dirB, "right", "", "")
		fs.IntVar(&concurrency, "c", defaultWorkers, "")
		fs.IntVar(&concurrency, "workers", defaultWorkers, "")
		fs.BoolVar(&noProgress, "no-progress", false, "")
		fs.BoolVar(&showAllDiffs, "show-all-diffs", false, "")
		fs.Var(&ignores, "ignore", "")

		if err := fs.Parse(parsedArgs); err != nil {
			return err
		}

		rest := fs.Args()
		if dirA == "" && dirB == "" {
			if len(rest) != 2 {
				return fmt.Errorf("错误：请通过 -dir-a/-dir-b 指定两个目录，或直接提供两个位置参数")
			}
			dirA, dirB = rest[0], rest[1]
		} else if len(rest) > 0 {
			return fmt.Errorf("错误：已使用 -dir-a/-dir-b 时，不应再附加位置参数")
		}

		return runComparison(runCtx, comparisonConfig{
			DirA:         dirA,
			DirB:         dirB,
			Concurrency:  concurrency,
			IgnoreRules:  ignores.Values(),
			ShowProgress: !noProgress,
			ShowAllDiffs: showAllDiffs,
		}, out)
	})
}

type comparisonConfig struct {
	DirA         string
	DirB         string
	Concurrency  int
	IgnoreRules  []string
	ShowProgress bool
	ShowAllDiffs bool
}

func runComparison(ctx context.Context, cfg comparisonConfig, out io.Writer) error {
	info1, err := os.Stat(cfg.DirA)
	if err != nil || !info1.IsDir() {
		return fmt.Errorf("错误：目录A %s 不存在或不是目录", cfg.DirA)
	}
	info2, err := os.Stat(cfg.DirB)
	if err != nil || !info2.IsDir() {
		return fmt.Errorf("错误：目录B %s 不存在或不是目录", cfg.DirB)
	}

	abs1, err := filepath.Abs(cfg.DirA)
	if err != nil {
		return fmt.Errorf("获取目录A绝对路径失败 %s: %w", cfg.DirA, err)
	}
	abs2, err := filepath.Abs(cfg.DirB)
	if err != nil {
		return fmt.Errorf("获取目录B绝对路径失败 %s: %w", cfg.DirB, err)
	}
	if abs1 == abs2 {
		fmt.Fprintln(out, "两个目录指向同一位置，无需比较。")
		return nil
	}

	fmt.Fprintf(out, "比较目录:\n  目录A: %s\n  目录B: %s\n", abs1, abs2)
	if len(cfg.IgnoreRules) > 0 {
		fmt.Fprintf(out, "忽略规则: %s\n", strings.Join(cfg.IgnoreRules, ", "))
	}
	fmt.Fprintln(out)

	diffs, err := compareDirs(ctx, abs1, abs2, cfg.Concurrency, cfg.IgnoreRules, cfg.ShowProgress, out)
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("任务已取消")
		}
		return fmt.Errorf("比较出错: %w", err)
	}

	fmt.Fprintln(out)
	if len(diffs) == 0 {
		fmt.Fprintln(out, "比较结果: 两个目录完全一致。")
		return nil
	}

	fmt.Fprintf(out, "比较结果: 发现 %d 处差异。\n", len(diffs))
	displayLimit := defaultDiffDisplayLimit
	if cfg.ShowAllDiffs {
		displayLimit = len(diffs)
	}
	for i, d := range diffs {
		if i < displayLimit {
			fmt.Fprintf(out, "  %s\n", d)
		} else if i == displayLimit {
			fmt.Fprintf(out, "  ... 还有 %d 处差异未显示，可加 -show-all-diffs 查看全部\n", len(diffs)-displayLimit)
			break
		}
	}
	return nil
}

// strSliceFlag 用于支持多次 -ignore，也支持一次传入多条规则。
type strSliceFlag []string

func (s *strSliceFlag) String() string {
	return strings.Join(*s, ", ")
}
func (s *strSliceFlag) Set(value string) error {
	for _, item := range strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r'
	}) {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			*s = append(*s, filepath.Clean(trimmed))
		}
	}
	return nil
}

func (s *strSliceFlag) Values() []string {
	if len(*s) == 0 {
		return nil
	}
	values := make([]string, 0, len(*s))
	seen := make(map[string]struct{}, len(*s))
	for _, item := range *s {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		values = append(values, item)
	}
	return values
}

func init() {
	framework.Register(&RecursiveContentDirDiffTool{})
}
