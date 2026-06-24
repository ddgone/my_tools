package hdfs_download

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"my_tools/libs/core/procutil"
	"my_tools/libs/framework"
)

const (
	defaultNamenode = "10.11.5.136:50070"
	defaultUser     = "hdfs"
	defaultParallel = 4
	maxRetries      = 3
	httpTimeout     = 60 * time.Second
)

var errSkipExisting = errors.New("skip existing file with matching size")

// WebHDFS JSON 响应结构体
type FileStatus struct {
	PathSuffix       string `json:"pathSuffix"`
	Type             string `json:"type"`
	Length           int64  `json:"length"`
	ModificationTime int64  `json:"modificationTime"`
}

type FileStatuses struct {
	FileStatus []FileStatus `json:"FileStatus"`
}

type StatusResponse struct {
	FileStatus FileStatus `json:"FileStatus"`
}

type ListStatusResponse struct {
	FileStatuses FileStatuses `json:"FileStatuses"`
}

// HDFSClient 封装 WebHDFS 操作
type HDFSClient struct {
	BaseURL  string
	Username string
	Client   *http.Client
}

type fileEntry struct {
	HdfsPath  string
	LocalPath string
	Size      int64
}

type batch struct {
	Files     []fileEntry
	HdfsPath  string
	LocalPath string
}

type downloadTask struct {
	Index int
	Entry fileEntry
}

type downloadResult struct {
	Task downloadTask
	Err  error
}

type HDFSDownloadTool struct{}

func (t *HDFSDownloadTool) ID() string       { return "hdfs_download" }
func (t *HDFSDownloadTool) Name() string     { return "HDFS 批量下载工具" }
func (t *HDFSDownloadTool) Category() string { return "KD测试工具 > HDFS工具" }

func (t *HDFSDownloadTool) Execute(ctx framework.AppContext) {
	usage := `HDFS 批量下载工具

说明:
通过 WebHDFS 从 HDFS 下载单个文件或整个目录到本地目录。
支持一次输入多个 HDFS 路径，适合批量拉取测试数据。

参数:
  -input <路径列表>       HDFS 路径列表，支持逗号、分号或换行分隔。
  -output <本地目录>      本地保存目录。
  -client <host:port>     WebHDFS Namenode 地址。默认: 10.11.5.136:50070
  -user <用户名>          WebHDFS 用户名。默认: hdfs
  -parallel <并发数>      并发下载文件数。默认: 4
  -skip                   如果本地已有同名文件且大小一致，则跳过下载。

示例:
1. 下载单个目录:
   -input "/user/test/job_001" -output "D:\downloads\job_001"

2. 批量下载多个路径:
   -input "/user/test/a,/user/test/b" -output "D:\downloads"

3. 指定用户名并跳过已存在文件:
   -input "/user/test/a" -output "D:\downloads" -client "10.11.5.136:50070" -user "hdfs" -skip
`

	ctx.ShowTerminal(t.Name(), usage, func(runCtx context.Context, args string, out io.Writer) error {
		parsedArgs, err := procutil.ParseArgs(args)
		if err != nil {
			return err
		}

		fs := flag.NewFlagSet("hdfs_download", flag.ContinueOnError)
		fs.SetOutput(out)

		var inputRaw string
		var outputDir string
		var clientStr string
		var username string
		var parallel int
		var skip bool

		fs.StringVar(&inputRaw, "input", "", "")
		fs.StringVar(&outputDir, "output", "", "")
		fs.StringVar(&clientStr, "client", defaultNamenode, "")
		fs.StringVar(&username, "user", defaultUser, "")
		fs.IntVar(&parallel, "parallel", defaultParallel, "")
		fs.BoolVar(&skip, "skip", false, "")

		if err := fs.Parse(parsedArgs); err != nil {
			return err
		}

		return runDownload(runCtx, downloadConfig{
			InputRaw:  inputRaw,
			OutputDir: outputDir,
			Client:    clientStr,
			User:      username,
			Parallel:  parallel,
			Skip:      skip,
		}, out)
	})
}

func NewHDFSClient(namenode, username string) *HDFSClient {
	return &HDFSClient{
		BaseURL:  fmt.Sprintf("http://%s/webhdfs/v1", namenode),
		Username: username,
		Client:   &http.Client{Timeout: httpTimeout},
	}
}

func (c *HDFSClient) buildURL(hdfsPath, op string) string {
	u, _ := url.Parse(c.BaseURL)
	u.Path = fmt.Sprintf("/webhdfs/v1%s", hdfsPath)
	q := u.Query()
	q.Set("op", op)
	q.Set("user.name", c.Username)
	u.RawQuery = q.Encode()
	return u.String()
}

func (c *HDFSClient) doRequest(ctx context.Context, hdfsPath, op string) (*http.Response, error) {
	reqURL := c.buildURL(hdfsPath, op)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%s build request failed: %w", op, err)
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s request failed: %w", op, err)
	}
	return resp, nil
}

func (c *HDFSClient) GetFileStatus(ctx context.Context, hdfsPath string) (*FileStatus, error) {
	resp, err := c.doRequest(ctx, hdfsPath, "GETFILESTATUS")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GETFILESTATUS returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var sr StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("decode GETFILESTATUS response: %w", err)
	}
	return &sr.FileStatus, nil
}

func (c *HDFSClient) ListDir(ctx context.Context, hdfsPath string) ([]FileStatus, error) {
	resp, err := c.doRequest(ctx, hdfsPath, "LISTSTATUS")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("LISTSTATUS returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var lr ListStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return nil, fmt.Errorf("decode LISTSTATUS response: %w", err)
	}
	return lr.FileStatuses.FileStatus, nil
}

func (c *HDFSClient) OpenFile(ctx context.Context, hdfsPath string) (io.ReadCloser, error) {
	resp, err := c.doRequest(ctx, hdfsPath, "OPEN")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("OPEN returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return resp.Body, nil
}

// collectFiles 递归收集目录下所有文件
func (c *HDFSClient) collectFiles(ctx context.Context, hdfsDir, localBase string) ([]fileEntry, error) {
	entries, err := c.ListDir(ctx, hdfsDir)
	if err != nil {
		return nil, fmt.Errorf("list dir %s: %w", hdfsDir, err)
	}

	files := make([]fileEntry, 0, len(entries))
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		fullHDFS := joinPath(hdfsDir, entry.PathSuffix)
		localPath := filepath.Join(localBase, entry.PathSuffix)
		switch entry.Type {
		case "DIRECTORY":
			subFiles, err := c.collectFiles(ctx, fullHDFS, localPath)
			if err != nil {
				return nil, err
			}
			files = append(files, subFiles...)
		case "FILE":
			files = append(files, fileEntry{
				HdfsPath:  fullHDFS,
				LocalPath: localPath,
				Size:      entry.Length,
			})
		}
	}
	return files, nil
}

// downloadFile 下载文件，若 skipExisting 为 true 且本地文件大小匹配则跳过
func (c *HDFSClient) downloadFile(ctx context.Context, entry fileEntry, skipExisting bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if skipExisting {
		if info, err := os.Stat(entry.LocalPath); err == nil && info.Size() == entry.Size {
			return errSkipExisting
		}
	}

	dir := filepath.Dir(entry.LocalPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create parent dir %s: %w", dir, err)
	}

	tmpPath := entry.LocalPath + ".tmp"
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			_ = os.Remove(tmpPath)
			return err
		}

		lastErr = func() error {
			rc, err := c.OpenFile(ctx, entry.HdfsPath)
			if err != nil {
				return err
			}
			defer rc.Close()

			f, err := os.Create(tmpPath)
			if err != nil {
				return fmt.Errorf("create temp file: %w", err)
			}
			defer f.Close()

			written, err := io.Copy(f, rc)
			if err != nil {
				return fmt.Errorf("copy data: %w", err)
			}
			if written != entry.Size {
				return fmt.Errorf("size mismatch during copy: expected %d, got %d", entry.Size, written)
			}
			return nil
		}()

		if lastErr == nil {
			break
		}
		_ = os.Remove(tmpPath)
	}
	if lastErr != nil {
		return fmt.Errorf("download after %d retries: %w", maxRetries, lastErr)
	}

	info, err := os.Stat(tmpPath)
	if err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("stat temp file: %w", err)
	}
	if info.Size() != entry.Size {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("temp file size mismatch: expected %d, got %d", entry.Size, info.Size())
	}

	if err := replaceFile(tmpPath, entry.LocalPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

type downloadConfig struct {
	InputRaw  string
	OutputDir string
	Client    string
	User      string
	Parallel  int
	Skip      bool
}

func runDownload(ctx context.Context, cfg downloadConfig, out io.Writer) error {
	inputRaw := strings.TrimSpace(cfg.InputRaw)
	if inputRaw == "" {
		return fmt.Errorf("错误：必须指定 -input 参数")
	}

	outputDir := strings.TrimSpace(cfg.OutputDir)
	if outputDir == "" {
		return fmt.Errorf("错误：必须指定 -output 参数")
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败 %s: %w", outputDir, err)
	}

	clientStr := strings.TrimSpace(cfg.Client)
	if clientStr == "" {
		clientStr = defaultNamenode
	}
	username := strings.TrimSpace(cfg.User)
	if username == "" {
		username = defaultUser
	}
	if cfg.Parallel < 1 {
		return fmt.Errorf("错误：-parallel 必须大于等于 1")
	}

	cleanPaths := parseInputPaths(inputRaw)
	if len(cleanPaths) == 0 {
		return fmt.Errorf("错误：未解析到有效的 HDFS 路径")
	}

	client := NewHDFSClient(clientStr, username)
	hadFailures := false
	batches := make([]batch, 0, len(cleanPaths))
	localRoots := buildUniqueLocalRoots(cleanPaths)

	fmt.Fprintf(out, "连接 WebHDFS: %s (user=%s)\n", clientStr, username)
	fmt.Fprintf(out, "输出目录: %s\n", outputDir)
	fmt.Fprintf(out, "输入路径数: %d\n\n", len(cleanPaths))

	for _, hdfsPath := range cleanPaths {
		if err := ctx.Err(); err != nil {
			return err
		}

		status, err := client.GetFileStatus(ctx, hdfsPath)
		if err != nil {
			fmt.Fprintf(out, "[警告] 获取路径状态失败 %s: %v，已跳过\n", hdfsPath, err)
			hadFailures = true
			continue
		}

		switch status.Type {
		case "DIRECTORY":
			localBase := filepath.Join(outputDir, localRoots[hdfsPath])
			if err := os.MkdirAll(localBase, 0755); err != nil {
				fmt.Fprintf(out, "[警告] 创建本地目录失败 %s: %v，已跳过\n", localBase, err)
				hadFailures = true
				continue
			}
			files, err := client.collectFiles(ctx, hdfsPath, localBase)
			if err != nil {
				fmt.Fprintf(out, "[警告] 枚举目录失败 %s: %v，已跳过\n", hdfsPath, err)
				hadFailures = true
				continue
			}
			batches = append(batches, batch{Files: files, HdfsPath: hdfsPath, LocalPath: localBase})
		case "FILE":
			localPath := filepath.Join(outputDir, localRoots[hdfsPath])
			batches = append(batches, batch{
				Files: []fileEntry{{
					HdfsPath:  hdfsPath,
					LocalPath: localPath,
					Size:      status.Length,
				}},
				HdfsPath:  hdfsPath,
				LocalPath: localPath,
			})
		default:
			fmt.Fprintf(out, "[警告] 不支持的路径类型 %s: %s，已跳过\n", status.Type, hdfsPath)
			hadFailures = true
		}
	}

	totalFiles := 0
	for _, currentBatch := range batches {
		totalFiles += len(currentBatch.Files)
	}
	if totalFiles == 0 {
		if hadFailures {
			return fmt.Errorf("未找到可下载文件，且存在路径处理失败")
		}
		fmt.Fprintln(out, "没有需要下载的文件。")
		return nil
	}

	tasks := make([]downloadTask, 0, totalFiles)
	for _, currentBatch := range batches {
		if err := ctx.Err(); err != nil {
			return err
		}

		if len(currentBatch.Files) > 1 {
			fmt.Fprintf(out, "处理目录: %s -> %s (%d files)\n", currentBatch.HdfsPath, currentBatch.LocalPath, len(currentBatch.Files))
		}

		for _, entry := range currentBatch.Files {
			tasks = append(tasks, downloadTask{Index: len(tasks) + 1, Entry: entry})
		}
	}

	workers := cfg.Parallel
	if workers > len(tasks) {
		workers = len(tasks)
	}
	fmt.Fprintf(out, "\n开始下载: 文件数=%d, 并发数=%d\n", totalFiles, workers)

	workerFailures, err := runDownloadTasks(ctx, client, tasks, cfg.Skip, workers, out)
	if err != nil {
		return err
	}
	hadFailures = hadFailures || workerFailures

	if hadFailures {
		fmt.Fprintln(out, "\n部分路径处理失败，详情见上方日志。")
		return fmt.Errorf("部分文件下载失败")
	}

	fmt.Fprintln(out, "\n全部下载完成。")
	return nil
}

func runDownloadTasks(ctx context.Context, client *HDFSClient, tasks []downloadTask, skipExisting bool, workers int, out io.Writer) (bool, error) {
	taskCh := make(chan downloadTask)
	resultCh := make(chan downloadResult, len(tasks))

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range taskCh {
				err := client.downloadFile(ctx, task.Entry, skipExisting)
				resultCh <- downloadResult{Task: task, Err: err}
			}
		}()
	}

	go func() {
		defer close(taskCh)
		for _, task := range tasks {
			select {
			case <-ctx.Done():
				return
			case taskCh <- task:
			}
		}
	}()

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	hadFailures := false
	total := len(tasks)
	for result := range resultCh {
		prefix := fmt.Sprintf("[%d/%d]", result.Task.Index, total)
		fmt.Fprintf(out, "%s 下载: %s -> %s", prefix, result.Task.Entry.HdfsPath, result.Task.Entry.LocalPath)

		switch {
		case errors.Is(result.Err, errSkipExisting):
			fmt.Fprintln(out, " ... Skipped (already exists & size match)")
		case result.Err != nil:
			fmt.Fprintf(out, " ... FAILED: %v\n", result.Err)
			hadFailures = true
		default:
			fmt.Fprintf(out, " ... Done (%s)\n", formatSize(result.Task.Entry.Size))
		}
	}

	if err := ctx.Err(); err != nil {
		return hadFailures, err
	}
	return hadFailures, nil
}

func parseInputPaths(input string) []string {
	normalized := strings.NewReplacer("\r\n", "\n", "，", ",", "；", ";").Replace(input)
	rawParts := strings.FieldsFunc(normalized, func(r rune) bool {
		return r == ',' || r == '\n' || r == ';'
	})

	result := make([]string, 0, len(rawParts))
	seen := make(map[string]struct{}, len(rawParts))
	for _, part := range rawParts {
		cleaned := normalizeHDFSPath(part)
		if cleaned == "" {
			continue
		}
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}
		result = append(result, cleaned)
	}
	return result
}

func normalizeHDFSPath(input string) string {
	cleaned := strings.TrimSpace(input)
	if cleaned == "" {
		return ""
	}
	cleaned = strings.ReplaceAll(cleaned, "\\", "/")
	if !strings.HasPrefix(cleaned, "/") {
		cleaned = "/" + cleaned
	}
	return cleaned
}

func replaceFile(tmpPath, targetPath string) error {
	if err := os.Rename(tmpPath, targetPath); err == nil {
		return nil
	}

	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove existing file %s: %w", targetPath, err)
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit && exp < 3; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(size)/float64(div), "KMG"[exp])
}

func joinPath(base, name string) string {
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(name, "/")
}

func buildUniqueLocalRoots(paths []string) map[string]string {
	result := make(map[string]string, len(paths))
	if len(paths) == 0 {
		return result
	}

	segmentsByPath := make(map[string][]string, len(paths))
	maxDepth := 0
	for _, hdfsPath := range paths {
		segments := splitHDFSPath(hdfsPath)
		segmentsByPath[hdfsPath] = segments
		if len(segments) > maxDepth {
			maxDepth = len(segments)
		}
	}

	candidateCounts := make([]map[string]int, maxDepth+1)
	for depth := 1; depth <= maxDepth; depth++ {
		counts := make(map[string]int, len(paths))
		for _, hdfsPath := range paths {
			segments := segmentsByPath[hdfsPath]
			if len(segments) < depth {
				continue
			}
			counts[buildSuffixPath(segments, depth)]++
		}
		candidateCounts[depth] = counts
	}

	for _, hdfsPath := range paths {
		segments := segmentsByPath[hdfsPath]
		if len(segments) == 0 {
			result[hdfsPath] = "root"
			continue
		}
		for depth := 1; depth <= len(segments); depth++ {
			candidate := buildSuffixPath(segments, depth)
			if candidateCounts[depth][candidate] == 1 {
				result[hdfsPath] = candidate
				break
			}
		}
		if result[hdfsPath] == "" {
			result[hdfsPath] = buildSuffixPath(segments, len(segments))
		}
	}

	return result
}

func splitHDFSPath(hdfsPath string) []string {
	trimmed := strings.Trim(strings.TrimSpace(hdfsPath), "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func buildSuffixPath(segments []string, depth int) string {
	if len(segments) == 0 {
		return "root"
	}
	if depth >= len(segments) {
		return filepath.Join(segments...)
	}
	return filepath.Join(segments[len(segments)-depth:]...)
}

func init() {
	framework.Register(&HDFSDownloadTool{})
}
