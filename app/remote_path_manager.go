package main

import (
	"context"
	"fmt"

	"fire-salamander-desktop/internal/runtime"
	"fire-salamander-desktop/internal/ssh"
)

func (a *App) BrowseRemotePath(connID string, requestedPath string) (*RemotePathBrowseResult, error) {
	conn, verifier, err := a.remoteBrowseCredentials(connID)
	if err != nil {
		return nil, err
	}

	executor, err := runtime.DialRemote(conn.Host, conn.Port, conn.User, conn.Password, conn.KeyPath, verifier.Callback())
	if err != nil {
		return nil, err
	}
	defer executor.Close()

	return executor.BrowsePath(context.Background(), requestedPath)
}

func (a *App) remoteBrowseCredentials(connID string) (*ssh.Connection, *ssh.HostKeyVerifier, error) {
	if connID == "" {
		return nil, nil, fmt.Errorf("请先选择远程环境")
	}

	conn, err := a.state.SSHStore.GetCredentials(connID)
	if err != nil {
		return nil, nil, err
	}
	if conn.Password == "" && conn.KeyPath == "" {
		return nil, nil, fmt.Errorf("当前 SSH 连接缺少认证凭据")
	}

	verifier := ssh.NewHostKeyVerifier(conn.HostKeyFingerprint)
	return conn, verifier, nil
}
