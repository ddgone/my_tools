package ssh

import (
	"fmt"
	"net"

	gossh "golang.org/x/crypto/ssh"
)

type HostKeyVerifier struct {
	KnownFingerprint string
	Accepted         string
}

func NewHostKeyVerifier(knownFingerprint string) *HostKeyVerifier {
	return &HostKeyVerifier{KnownFingerprint: knownFingerprint}
}

func (v *HostKeyVerifier) Callback() gossh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key gossh.PublicKey) error {
		fingerprint := gossh.FingerprintSHA256(key)

		if v.KnownFingerprint == "" {
			v.Accepted = fingerprint
			return nil
		}

		if fingerprint == v.KnownFingerprint {
			return nil
		}

		return fmt.Errorf("SSH主机密钥指纹不匹配:\n  期望: %s\n  实际: %s\n如果确认主机更改，请删除此连接后重新创建", v.KnownFingerprint, fingerprint)
	}
}
