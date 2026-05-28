package ssh

import (
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"
)

func BuildAuthMethods(password, keyPath string) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	if keyPath != "" {
		key, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("读取私钥失败 (%s): %w", keyPath, err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("解析私钥失败 (%s): %w", keyPath, err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}

	if password != "" {
		methods = append(methods, ssh.Password(password))
	}

	if len(methods) == 0 {
		return nil, fmt.Errorf("未提供认证凭据（密码或密钥）")
	}

	return methods, nil
}
