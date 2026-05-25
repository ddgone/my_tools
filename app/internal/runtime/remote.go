package runtime

import (
	"bytes"
	"context"
	"fmt"
	"io"
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

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("获取stdin失败: %w", err)
	}

	go func() {
		defer stdin.Close()
		<-ctx.Done()
	}()

	cmd := fmt.Sprintf("cat > %s", remotePath)
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
