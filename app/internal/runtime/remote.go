package runtime

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type RemoteExecutor struct {
	SSHClient *ssh.Client
}

func DialRemote(host string, port int, user, password string) (*RemoteExecutor, error) {
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
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
	executor, err := DialRemote(host, port, user, password)
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
	output, err := r.RunOutput(ctx, "uname -s && uname -m")
	if err != nil {
		return RemotePlatform{}, fmt.Errorf("检测远端平台失败: %w", err)
	}

	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		return RemotePlatform{}, fmt.Errorf("无法解析远端平台信息: %q", output)
	}

	platform := RemotePlatform{
		OS:   normalizeRemoteOS(lines[0]),
		Arch: normalizeRemoteArch(lines[1]),
	}
	if platform.OS == "" || platform.Arch == "" {
		return RemotePlatform{}, fmt.Errorf("暂不支持的远端平台: %q", output)
	}
	return platform, nil
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
