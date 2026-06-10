package runtime

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/ssh"
)

type RemoteExecutor struct {
	SSHClient *ssh.Client
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

func (r *RemoteExecutor) RunOutput(ctx context.Context, cmd string) (string, error) {
	var buf bytes.Buffer
	if err := r.Execute(ctx, cmd, &buf); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

func (r *RemoteExecutor) DetectPlatform(ctx context.Context) (RemotePlatform, error) {
	probes := []string{
		"uname -s && uname -m",
		"sh -lc 'uname -s && uname -m'",
		"bash -lc 'uname -s && uname -m'",
	}

	failures := make([]string, 0, len(probes))
	for _, probe := range probes {
		output, err := r.RunOutput(ctx, probe)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s => err: %v", probe, err))
			continue
		}

		platform, parseErr := parseRemotePlatformOutput(output)
		if parseErr == nil {
			return platform, nil
		}
		failures = append(failures, fmt.Sprintf("%s => output: %q (%v)", probe, output, parseErr))
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
		output, err := r.RunOutput(ctx, probe)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s => err: %v", probe, err))
			continue
		}
		value, parseErr := parseInt64(strings.TrimSpace(output))
		if parseErr == nil {
			return value, nil
		}
		failures = append(failures, fmt.Sprintf("%s => output: %q", probe, output))
	}
	return 0, fmt.Errorf("无法探测远端大小，探测详情: %s", strings.Join(failures, " | "))
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
