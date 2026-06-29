package runtime

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/ssh"
)

type RemoteExecutor struct {
	SSHClient *ssh.Client
}

type RemotePathEntry struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Kind      string `json:"kind"`
	IsSymlink bool   `json:"isSymlink"`
}

type RemotePathBrowseResult struct {
	RequestedPath string            `json:"requestedPath"`
	CurrentPath   string            `json:"currentPath"`
	HomePath      string            `json:"homePath"`
	Fallback      bool              `json:"fallback"`
	Message       string            `json:"message,omitempty"`
	Entries       []RemotePathEntry `json:"entries"`
}

type countingWriter struct {
	writer     io.Writer
	written    int64
	onProgress func(int64)
}

func (w *countingWriter) Write(p []byte) (int, error) {
	n, err := w.writer.Write(p)
	if n > 0 {
		w.written += int64(n)
		if w.onProgress != nil {
			w.onProgress(w.written)
		}
	}
	return n, err
}

func DialRemote(host string, port int, user, password, keyPath string, hostKeyCallback ssh.HostKeyCallback) (*RemoteExecutor, error) {
	authMethods := []ssh.AuthMethod{}
	if keyPath != "" {
		key, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("读取私钥失败 (%s): %w", keyPath, err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("解析私钥失败 (%s): %w", keyPath, err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}
	if password != "" {
		authMethods = append(authMethods, ssh.Password(password))
	}
	if len(authMethods) == 0 {
		return nil, fmt.Errorf("未提供认证凭据（密码或密钥）")
	}

	config := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         10 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("SSH连接失败 %s: %w", addr, err)
	}

	return &RemoteExecutor{SSHClient: client}, nil
}

func (r *RemoteExecutor) Close() error {
	if r.SSHClient != nil {
		return r.SSHClient.Close()
	}
	return nil
}

func (r *RemoteExecutor) Upload(ctx context.Context, localPath, remotePath string) error {
	session, err := r.SSHClient.NewSession()
	if err != nil {
		return fmt.Errorf("创建上传会话失败: %w", err)
	}
	defer session.Close()

	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("打开本地产物失败: %w", err)
	}
	defer file.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("获取stdin失败: %w", err)
	}

	go func() {
		defer stdin.Close()
		done := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				_ = stdin.Close()
			case <-done:
			}
		}()
		_, _ = io.Copy(stdin, file)
		close(done)
	}()

	cmd := fmt.Sprintf("cat > %s", ShellQuote(remotePath))
	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("上传失败: %w", err)
	}

	return nil
}

func (r *RemoteExecutor) streamCommandToWriter(ctx context.Context, cmd string, out io.Writer) error {
	session, err := r.SSHClient.NewSession()
	if err != nil {
		return fmt.Errorf("创建下载会话失败: %w", err)
	}
	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("获取下载输出失败: %w", err)
	}
	var stderr bytes.Buffer
	session.Stderr = &stderr

	if err := session.Start(cmd); err != nil {
		return fmt.Errorf("启动远端命令失败: %w", err)
	}

	copyDone := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(out, stdout)
		copyDone <- copyErr
	}()

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- session.Wait()
	}()

	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGTERM)
		return ctx.Err()
	case copyErr := <-copyDone:
		if copyErr != nil {
			return fmt.Errorf("接收远端输出失败: %w", copyErr)
		}
		if waitErr := <-waitDone; waitErr != nil {
			detail := strings.TrimSpace(stderr.String())
			if detail != "" {
				return fmt.Errorf("%w: %s", waitErr, detail)
			}
			return waitErr
		}
		return nil
	}
}

func (r *RemoteExecutor) ExecuteSuccess(ctx context.Context, cmd string) (bool, error) {
	session, err := r.SSHClient.NewSession()
	if err != nil {
		return false, fmt.Errorf("创建探测会话失败: %w", err)
	}
	defer session.Close()

	done := make(chan error, 1)
	go func() {
		done <- session.Run(cmd)
	}()

	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGTERM)
		return false, ctx.Err()
	case err := <-done:
		if err == nil {
			return true, nil
		}
		if _, ok := err.(*ssh.ExitError); ok {
			return false, nil
		}
		return false, err
	}
}

func (r *RemoteExecutor) downloadToFile(ctx context.Context, cmd string, localPath string, onProgress func(int64)) error {
	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("创建本地文件失败: %w", err)
	}
	success := false
	defer func() {
		_ = file.Close()
		if !success {
			_ = os.Remove(localPath)
		}
	}()

	writer := io.Writer(file)
	if onProgress != nil {
		writer = &countingWriter{writer: file, onProgress: onProgress}
	}
	if err := r.streamCommandToWriter(ctx, cmd, writer); err != nil {
		return err
	}
	success = true
	return nil
}

func (r *RemoteExecutor) DownloadFile(ctx context.Context, remotePath string, localPath string) error {
	cmd := fmt.Sprintf("cat %s", ShellQuote(remotePath))
	return r.downloadToFile(ctx, cmd, localPath, nil)
}

func (r *RemoteExecutor) DownloadFileWithProgress(ctx context.Context, remotePath string, localPath string, onProgress func(int64)) error {
	cmd := fmt.Sprintf("cat %s", ShellQuote(remotePath))
	return r.downloadToFile(ctx, cmd, localPath, onProgress)
}

func (r *RemoteExecutor) DownloadDirectoryTarGz(ctx context.Context, remotePath string, localPath string) error {
	parent := path.Dir(remotePath)
	base := path.Base(remotePath)
	cmd := fmt.Sprintf("tar -czf - -C %s %s", ShellQuote(parent), ShellQuote(base))
	return r.downloadToFile(ctx, cmd, localPath, nil)
}

func (r *RemoteExecutor) DownloadDirectoryTarGzWithProgress(ctx context.Context, remotePath string, localPath string, onProgress func(int64)) error {
	parent := path.Dir(remotePath)
	base := path.Base(remotePath)
	cmd := fmt.Sprintf("tar -czf - -C %s %s", ShellQuote(parent), ShellQuote(base))
	return r.downloadToFile(ctx, cmd, localPath, onProgress)
}

func (r *RemoteExecutor) DownloadDirectoryTarGzViaTempArchive(ctx context.Context, remotePath string, localPath string, remoteArchivePath string, onProgress func(downloaded int64, total int64)) error {
	parent := path.Dir(remotePath)
	base := path.Base(remotePath)
	createCmd := fmt.Sprintf("tar -czf %s -C %s %s", ShellQuote(remoteArchivePath), ShellQuote(parent), ShellQuote(base))
	if err := r.Execute(ctx, createCmd, io.Discard); err != nil {
		return fmt.Errorf("创建远端归档失败: %w", err)
	}
	defer func() {
		_, _ = r.RunOutput(context.Background(), fmt.Sprintf("rm -f %s", ShellQuote(remoteArchivePath)))
	}()

	if onProgress != nil {
		onProgress(0, 0)
	}

	var downloadedBytes atomic.Int64
	var totalBytes atomic.Int64
	sizeDone := make(chan struct{})
	go func() {
		defer close(sizeDone)
		total, err := r.GetFileSize(ctx, remoteArchivePath)
		if err != nil {
			return
		}
		totalBytes.Store(total)
		if onProgress != nil {
			onProgress(downloadedBytes.Load(), total)
		}
	}()

	if err := r.DownloadFileWithProgress(ctx, remoteArchivePath, localPath, func(downloaded int64) {
		downloadedBytes.Store(downloaded)
		if onProgress != nil {
			onProgress(downloaded, totalBytes.Load())
		}
	}); err != nil {
		return err
	}

	<-sizeDone
	if onProgress != nil {
		onProgress(downloadedBytes.Load(), totalBytes.Load())
	}
	return nil
}

func (r *RemoteExecutor) Execute(ctx context.Context, cmd string, out io.Writer) error {
	session, err := r.SSHClient.NewSession()
	if err != nil {
		return fmt.Errorf("创建执行会话失败: %w", err)
	}
	defer session.Close()

	session.Stdout = out
	session.Stderr = out

	done := make(chan error, 1)
	go func() {
		done <- session.Run(cmd)
	}()

	select {
	case <-ctx.Done():
		session.Signal(ssh.SIGTERM)
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			session.Signal(ssh.SIGKILL)
		}
		return ctx.Err()
	case err := <-done:
		return err
	}
}

func RunOneShot(host string, port int, user, password, cmd string, out io.Writer) error {
	executor, err := DialRemote(host, port, user, password, "", ssh.InsecureIgnoreHostKey())
	if err != nil {
		return err
	}
	defer executor.Close()

	return executor.Execute(context.Background(), cmd, out)
}

func BuildRemoteRunScript(scriptPath, args string) string {
	parts := []string{"python3", scriptPath}
	if strings.TrimSpace(args) != "" {
		parts = append(parts, strings.Fields(args)...)
	}
	return strings.Join(parts, " ")
}

type RemotePlatform struct {
	OS   string
	Arch string
}

type commandOutput struct {
	Stdout string
	Stderr string
}

func (r *RemoteExecutor) captureOutput(ctx context.Context, cmd string) (commandOutput, error) {
	session, err := r.SSHClient.NewSession()
	if err != nil {
		return commandOutput{}, fmt.Errorf("创建输出捕获会话失败: %w", err)
	}
	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		return commandOutput{}, fmt.Errorf("获取stdout失败: %w", err)
	}

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	session.Stderr = &stderrBuf

	if err := session.Start(cmd); err != nil {
		return commandOutput{}, fmt.Errorf("启动远端命令失败: %w", err)
	}

	copyDone := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(&stdoutBuf, stdout)
		copyDone <- copyErr
	}()

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- session.Wait()
	}()

	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGTERM)
		select {
		case <-waitDone:
		case <-time.After(3 * time.Second):
			_ = session.Signal(ssh.SIGKILL)
			<-waitDone
		}
		return commandOutput{}, ctx.Err()
	case copyErr := <-copyDone:
		result := commandOutput{
			Stdout: strings.TrimSpace(stdoutBuf.String()),
			Stderr: strings.TrimSpace(stderrBuf.String()),
		}
		if copyErr != nil {
			return result, fmt.Errorf("接收远端stdout失败: %w", copyErr)
		}
		if waitErr := <-waitDone; waitErr != nil {
			if result.Stderr != "" {
				return result, fmt.Errorf("%w: %s", waitErr, result.Stderr)
			}
			return result, waitErr
		}
		return result, nil
	}
}

func (r *RemoteExecutor) RunOutput(ctx context.Context, cmd string) (string, error) {
	result, err := r.captureOutput(ctx, cmd)
	if err != nil {
		return "", err
	}
	return result.Stdout, nil
}

func (r *RemoteExecutor) DetectPlatform(ctx context.Context) (RemotePlatform, error) {
	probes := []string{
		"uname -s && uname -m",
		"sh -lc 'uname -s && uname -m'",
		"bash -lc 'uname -s && uname -m'",
	}

	failures := make([]string, 0, len(probes))
	for _, probe := range probes {
		result, err := r.captureOutput(ctx, probe)
		if err != nil {
			failures = append(failures, formatProbeDetail(probe, result.Stdout, result.Stderr, err))
			continue
		}

		platform, parseErr := parseRemotePlatformOutput(result.Stdout)
		if parseErr == nil {
			return platform, nil
		}
		failures = append(failures, formatProbeDetail(probe, result.Stdout, result.Stderr, parseErr))
	}

	return RemotePlatform{}, fmt.Errorf("无法解析远端平台信息，探测详情: %s", strings.Join(failures, " | "))
}

func (r *RemoteExecutor) DetectPathKind(ctx context.Context, remotePath string) (string, bool, error) {
	probes := []string{
		fmt.Sprintf("test -d %s", ShellQuote(remotePath)),
		fmt.Sprintf("sh -lc 'test -d \"$1\"' sh %s", ShellQuote(remotePath)),
		fmt.Sprintf("bash -lc 'test -d \"$1\"' bash %s", ShellQuote(remotePath)),
	}
	for _, probe := range probes {
		ok, err := r.ExecuteSuccess(ctx, probe)
		if err != nil {
			continue
		}
		if ok {
			return "directory", true, nil
		}
	}

	fileProbes := []string{
		fmt.Sprintf("test -f %s || test -e %s", ShellQuote(remotePath), ShellQuote(remotePath)),
		fmt.Sprintf("sh -lc 'test -f \"$1\" || test -e \"$1\"' sh %s", ShellQuote(remotePath)),
		fmt.Sprintf("bash -lc 'test -f \"$1\" || test -e \"$1\"' bash %s", ShellQuote(remotePath)),
	}
	for _, probe := range fileProbes {
		ok, err := r.ExecuteSuccess(ctx, probe)
		if err != nil {
			continue
		}
		if ok {
			return "file", true, nil
		}
	}

	return "", false, nil
}

func (r *RemoteExecutor) BrowsePath(ctx context.Context, requestedPath string) (*RemotePathBrowseResult, error) {
	homePath, err := r.HomeDirectory(ctx)
	if err != nil {
		return nil, err
	}

	currentPath, fallback, err := r.resolveBrowseDirectory(ctx, requestedPath, homePath)
	if err != nil {
		return nil, err
	}

	entries, err := r.ListDirectory(ctx, currentPath)
	if err != nil {
		return nil, err
	}

	result := &RemotePathBrowseResult{
		RequestedPath: strings.TrimSpace(requestedPath),
		CurrentPath:   currentPath,
		HomePath:      homePath,
		Fallback:      fallback,
		Entries:       entries,
	}
	if fallback && strings.TrimSpace(requestedPath) != "" {
		result.Message = fmt.Sprintf("未找到 %q，已回到 ~", strings.TrimSpace(requestedPath))
	}
	return result, nil
}

func (r *RemoteExecutor) HomeDirectory(ctx context.Context) (string, error) {
	probes := []string{
		`pwd`,
		`sh -lc 'cd ~ >/dev/null 2>&1 && pwd'`,
		`bash -lc 'cd ~ >/dev/null 2>&1 && pwd'`,
	}
	failures := make([]string, 0, len(probes))
	for _, probe := range probes {
		result, err := r.captureOutput(ctx, probe)
		if err != nil {
			failures = append(failures, formatProbeDetail(probe, result.Stdout, result.Stderr, err))
			continue
		}
		home := normalizeRemotePath(result.Stdout)
		if home != "" {
			return home, nil
		}
		failures = append(failures, formatProbeDetail(probe, result.Stdout, result.Stderr, fmt.Errorf("empty home path")))
	}
	return "", fmt.Errorf("无法解析远端 home 目录，探测详情: %s", strings.Join(failures, " | "))
}

func (r *RemoteExecutor) ListDirectory(ctx context.Context, remotePath string) ([]RemotePathEntry, error) {
	normalizedPath := normalizeRemotePath(remotePath)
	if normalizedPath == "" {
		return nil, fmt.Errorf("远端目录不能为空")
	}

	result, err := r.captureOutput(ctx, fmt.Sprintf(
		"sh -lc 'dir=$1; "+
			"for entry in \"$dir\"/.[!.]* \"$dir\"/..?* \"$dir\"/*; do "+
			"[ -e \"$entry\" ] || [ -L \"$entry\" ] || continue; "+
			"name=$(basename \"$entry\"); "+
			"[ \"$name\" = \".\" ] && continue; "+
			"[ \"$name\" = \"..\" ] && continue; "+
			"kind=file; "+
			"[ -d \"$entry\" ] && kind=directory; "+
			"symlink=false; "+
			"[ -L \"$entry\" ] && symlink=true; "+
			"printf \"%%s\\t%%s\\t%%s\\t%%s\\n\" \"$name\" \"$entry\" \"$kind\" \"$symlink\"; "+
			"done' sh %s",
		ShellQuote(normalizedPath),
	))
	if err != nil {
		return nil, fmt.Errorf("读取远端目录失败: %w", err)
	}
	return parseRemotePathEntries(result.Stdout), nil
}

func (r *RemoteExecutor) resolveBrowseDirectory(ctx context.Context, requestedPath string, homePath string) (string, bool, error) {
	trimmedPath := strings.TrimSpace(requestedPath)
	if trimmedPath == "" {
		return homePath, false, nil
	}

	result, err := r.captureOutput(ctx, fmt.Sprintf(
		"sh -lc 'requested=$1; home=$2; target=$requested; "+
			"case \"$target\" in "+
			"\"\") target=$home ;; "+
			"\"~\") target=$home ;; "+
			"~/*) target=\"$home/${target#~/}\" ;; "+
			"esac; "+
			"if [ \"${target#/}\" = \"$target\" ]; then target=\"$home/$target\"; fi; "+
			"if [ -d \"$target\" ]; then cd \"$target\" >/dev/null 2>&1 && printf \"resolved\\t%%s\" \"$(pwd)\" && exit 0; fi; "+
			"if [ -e \"$target\" ]; then parent=$(dirname \"$target\"); cd \"$parent\" >/dev/null 2>&1 && printf \"resolved\\t%%s\" \"$(pwd)\" && exit 0; fi; "+
			"cd \"$home\" >/dev/null 2>&1 && printf \"fallback\\t%%s\" \"$(pwd)\"' sh %s %s",
		ShellQuote(trimmedPath),
		ShellQuote(homePath),
	))
	if err != nil {
		return "", false, fmt.Errorf("解析远端浏览目录失败: %w", err)
	}

	parts := strings.SplitN(strings.TrimSpace(result.Stdout), "\t", 2)
	if len(parts) != 2 {
		return "", false, fmt.Errorf("无法解析远端浏览目录输出: %q", result.Stdout)
	}

	currentPath := normalizeRemotePath(parts[1])
	if currentPath == "" {
		return "", false, fmt.Errorf("远端浏览目录为空")
	}

	return currentPath, parts[0] == "fallback", nil
}

func (r *RemoteExecutor) GetFileSize(ctx context.Context, remotePath string) (int64, error) {
	probes := []string{
		fmt.Sprintf("wc -c < %s", ShellQuote(remotePath)),
		fmt.Sprintf("stat -c %%s %s", ShellQuote(remotePath)),
		fmt.Sprintf("sh -lc 'wc -c < \"$1\"' sh %s", ShellQuote(remotePath)),
	}
	return r.runInt64Probe(ctx, probes)
}

func (r *RemoteExecutor) EstimateDirectoryArchiveSize(ctx context.Context, remotePath string) (int64, error) {
	probes := []string{
		fmt.Sprintf("du -sb %s | cut -f1", ShellQuote(remotePath)),
		fmt.Sprintf("sh -lc 'du -sb \"$1\" | cut -f1' sh %s", ShellQuote(remotePath)),
		fmt.Sprintf("bash -lc 'du -sb \"$1\" | cut -f1' bash %s", ShellQuote(remotePath)),
	}
	return r.runInt64Probe(ctx, probes)
}

func (r *RemoteExecutor) runInt64Probe(ctx context.Context, probes []string) (int64, error) {
	failures := make([]string, 0, len(probes))
	for _, probe := range probes {
		result, err := r.captureOutput(ctx, probe)
		if err != nil {
			failures = append(failures, formatProbeDetail(probe, result.Stdout, result.Stderr, err))
			continue
		}
		value, parseErr := parseInt64(result.Stdout)
		if parseErr == nil {
			return value, nil
		}
		failures = append(failures, formatProbeDetail(probe, result.Stdout, result.Stderr, parseErr))
	}
	return 0, fmt.Errorf("无法探测远端大小，探测详情: %s", strings.Join(failures, " | "))
}

func formatProbeDetail(probe string, stdout string, stderr string, detail error) string {
	parts := []string{
		fmt.Sprintf("stdout: %q", stdout),
		fmt.Sprintf("stderr: %q", stderr),
	}
	if detail != nil {
		parts = append(parts, detail.Error())
	}
	return fmt.Sprintf("%s => %s", probe, strings.Join(parts, " | "))
}

func parseRemotePathEntries(output string) []RemotePathEntry {
	lines := strings.Split(output, "\n")
	entries := make([]RemotePathEntry, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) != 4 {
			continue
		}
		entry := RemotePathEntry{
			Name:      strings.TrimSpace(parts[0]),
			Path:      normalizeRemotePath(parts[1]),
			Kind:      normalizeRemoteEntryKind(parts[2]),
			IsSymlink: strings.EqualFold(strings.TrimSpace(parts[3]), "true"),
		}
		if entry.Name == "" || entry.Path == "" || entry.Kind == "" {
			continue
		}
		entries = append(entries, entry)
	}
	sortRemotePathEntries(entries)
	return entries
}

func sortRemotePathEntries(entries []RemotePathEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Kind != entries[j].Kind {
			return entries[i].Kind == "directory"
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
}

func parseInt64(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("empty")
	}
	var parsed int64
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("invalid")
		}
		parsed = parsed*10 + int64(ch-'0')
	}
	return parsed, nil
}

func ShellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func normalizeRemotePath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	cleaned := path.Clean(value)
	if cleaned == "." {
		return ""
	}
	return cleaned
}

func normalizeRemoteEntryKind(value string) string {
	switch strings.TrimSpace(value) {
	case "directory":
		return "directory"
	case "file":
		return "file"
	default:
		return ""
	}
}

func normalizeRemoteOS(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "linux":
		return "linux"
	case "darwin":
		return "darwin"
	default:
		return ""
	}
}

func normalizeRemoteArch(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "x86_64", "amd64":
		return "amd64"
	case "aarch64", "arm64":
		return "arm64"
	default:
		return ""
	}
}

func parseRemotePlatformOutput(output string) (RemotePlatform, error) {
	lines := strings.Split(output, "\n")
	normalized := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		normalized = append(normalized, line)
	}
	if len(normalized) < 2 {
		return RemotePlatform{}, fmt.Errorf("平台输出不足: %q", output)
	}

	platform := RemotePlatform{
		OS:   normalizeRemoteOS(normalized[0]),
		Arch: normalizeRemoteArch(normalized[1]),
	}
	if platform.OS == "" || platform.Arch == "" {
		return RemotePlatform{}, fmt.Errorf("暂不支持的远端平台: %q", output)
	}
	return platform, nil
}

type RemoteExecResult struct {
	Output string
	Error  error
}

func RunRemoteCommand(host string, port int, user, password, cmd string) RemoteExecResult {
	var buf bytes.Buffer
	err := RunOneShot(host, port, user, password, cmd, &buf)
	return RemoteExecResult{
		Output: buf.String(),
		Error:  err,
	}
}
