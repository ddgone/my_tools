package ssh

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"fire-salamander-desktop/internal/runtimeenv"
)

type Connection struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Host               string `json:"host"`
	Port               int    `json:"port"`
	User               string `json:"user"`
	AuthMethod         string `json:"authMethod"`
	Password           string `json:"password,omitempty"`
	KeyPath            string `json:"keyPath,omitempty"`
	Description        string `json:"description"`
	HostKeyFingerprint string `json:"hostKeyFingerprint,omitempty"`
	HostKeyAlgorithm   string `json:"hostKeyAlgorithm,omitempty"`
	LastUsedAt         int64  `json:"lastUsedAt,omitempty"`
}

type Store struct {
	mu          sync.RWMutex
	connections map[string]*Connection
	configPath  string
}

func NewStore() *Store {
	layout, err := runtimeenv.ResolveLayout()
	if err != nil {
		layout = runtimeenv.Layout{Root: "."}
	}

	fp := filepath.Join(layout.ConfigDir(), "ssh_connections.json")

	return &Store{
		connections: map[string]*Connection{},
		configPath:  fp,
	}
}

func (s *Store) LoadConfig() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.configPath == "" {
		return fmt.Errorf("SSH配置路径未初始化")
	}

	dir := filepath.Dir(s.configPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("无法创建配置目录 %s: %w", dir, err)
	}

	data, err := os.ReadFile(s.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("读取SSH配置(%s)失败: %w", s.configPath, err)
	}

	return json.Unmarshal(data, &s.connections)
}

func (s *Store) saveLocked() error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s.connections); err != nil {
		return fmt.Errorf("序列化SSH配置失败: %w", err)
	}

	if err := os.WriteFile(s.configPath, buf.Bytes(), 0600); err != nil {
		return fmt.Errorf("写入SSH配置失败: %w", err)
	}

	return nil
}

func (s *Store) List() []*Connection {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Connection, 0, len(s.connections))
	for _, conn := range s.connections {
		cp := *conn
		cp.Password = ""
		cp.KeyPath = ""
		cp.HostKeyFingerprint = ""
		cp.HostKeyAlgorithm = ""
		result = append(result, &cp)
	}
	return result
}

func (s *Store) Save(conn Connection) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if conn.ID == "" {
		conn.ID = fmt.Sprintf("ssh_%d_%s", len(s.connections), conn.Host)
	}

	if conn.Port == 0 {
		conn.Port = 22
	}

	s.connections[conn.ID] = &conn
	return conn.ID, s.saveLocked()
}

func (s *Store) Update(id string, conn Connection) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.connections[id]; !ok {
		return fmt.Errorf("SSH连接不存在: %s", id)
	}

	if conn.Port == 0 {
		conn.Port = 22
	}
	conn.ID = id

	s.connections[id] = &conn
	return s.saveLocked()
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.connections, id)
	return s.saveLocked()
}

func (s *Store) GetCredentials(id string) (*Connection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	conn, ok := s.connections[id]
	if !ok {
		return nil, fmt.Errorf("SSH连接不存在: %s", id)
	}

	cp := *conn
	return &cp, nil
}

// TestAndUpdate 测试连接，并在主机指纹变化时自动更新存储。
func (s *Store) TestAndUpdate(id string) TestResult {
	creds, err := s.GetCredentials(id)
	if err != nil {
		return TestResult{Success: false, Message: err.Error()}
	}
	if creds.Password == "" && creds.KeyPath == "" {
		return TestResult{Success: false, Message: "连接缺少认证凭据"}
	}
	return s.testWithFingerprintUpdate(id, *creds)
}

func (s *Store) testWithFingerprintUpdate(id string, creds Connection) TestResult {
	verifier := NewHostKeyVerifier(creds.HostKeyFingerprint)
	result := TestConnection(creds.Host, creds.Port, creds.User, creds.Password, creds.KeyPath, verifier)

	if result.Success && verifier.Accepted != "" && creds.HostKeyFingerprint != verifier.Accepted {
		creds.HostKeyFingerprint = verifier.Accepted
		if err := s.Update(id, creds); err != nil {
			result.Message += " (注意: 主机指纹保存失败)"
		}
	}
	return result
}
