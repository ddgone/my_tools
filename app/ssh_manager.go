package main

import "fire-salamander-desktop/internal/ssh"

func (a *App) ListSSHConnections() []*ssh.Connection {
	return a.state.SSHStore.List()
}

func (a *App) GetSSHConnection(id string) (*ssh.Connection, error) {
	return a.state.SSHStore.GetCredentials(id)
}

func (a *App) SaveSSHConnection(conn ssh.Connection) (ssh.Connection, error) {
	id, err := a.state.SSHStore.Save(conn)
	if err != nil {
		return ssh.Connection{}, err
	}
	saved, err := a.state.SSHStore.GetCredentials(id)
	if err != nil {
		return ssh.Connection{}, err
	}
	return *saved, nil
}

func (a *App) DeleteSSHConnection(id string) error {
	return a.state.SSHStore.Delete(id)
}

func (a *App) UpdateSSHConnection(id string, conn ssh.Connection) error {
	return a.state.SSHStore.Update(id, conn)
}

func (a *App) TestSSHConnection(id string) ssh.TestResult {
	return a.state.SSHStore.TestAndUpdate(id)
}

func (a *App) TestSSHConnectionRaw(host string, port int, user, password, keyPath string) ssh.TestResult {
	if host == "" || user == "" {
		return ssh.TestResult{Success: false, Message: "主机地址和用户名不能为空"}
	}
	if password == "" && keyPath == "" {
		return ssh.TestResult{Success: false, Message: "必须提供密码或密钥路径"}
	}
	verifier := ssh.NewHostKeyVerifier("")
	return ssh.TestConnection(host, port, user, password, keyPath, verifier)
}
