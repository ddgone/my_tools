package ssh

import (
	"fmt"
	"time"

	"golang.org/x/crypto/ssh"
)

type TestResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func TestConnection(host string, port int, user, password string) TestResult {
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("连接失败: %v", err),
		}
	}
	defer client.Close()

	return TestResult{
		Success: true,
		Message: fmt.Sprintf("连接成功 (%s@%s)", user, addr),
	}
}
