package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// HistoryItem 历史记录项
type HistoryItem struct {
	ToolName   string            `json:"tool_name"`
	ToolPath   string            `json:"tool_path"`
	LastParams map[string]string `json:"last_params"`
	Timestamp  int64             `json:"timestamp"`
}

// UserData 用户数据
type UserData struct {
	RecentTools []HistoryItem                  `json:"recent_tools"`
	NodeStates  map[string]bool                `json:"node_states"`  // 记录节点展开/收起状态
	ToolHistory map[string][]map[string]string `json:"tool_history"` // 记录每个工具的历史执行参数
}

// Storage 数据存储
type Storage struct {
	dataFile string
	data     *UserData
}

// NewStorage 创建存储实例
func NewStorage() *Storage {
	homeDir, _ := os.UserHomeDir()
	dataDir := filepath.Join(homeDir, ".my_tools")
	_ = os.MkdirAll(dataDir, 0755)

	return &Storage{
		dataFile: filepath.Join(dataDir, "user_data.json"),
		data: &UserData{
			NodeStates: make(map[string]bool),
		},
	}
}

// Load 加载数据
func (s *Storage) Load() error {
	data, err := os.ReadFile(s.dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 文件不存在，使用默认值
		}
		return err
	}

	if s.data.NodeStates == nil {
		s.data.NodeStates = make(map[string]bool)
	}
	if s.data.ToolHistory == nil {
		s.data.ToolHistory = make(map[string][]map[string]string)
	}
	return json.Unmarshal(data, s.data)
}

// Save 保存数据
func (s *Storage) Save() error {
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.dataFile, data, 0644)
}

// AddRecentTool 添加最近使用的工具
func (s *Storage) AddRecentTool(toolName, toolPath string, params map[string]string) {
	// 移除已存在的相同工具
	for i, item := range s.data.RecentTools {
		if item.ToolPath == toolPath {
			s.data.RecentTools = append(s.data.RecentTools[:i], s.data.RecentTools[i+1:]...)
			break
		}
	}

	// 添加到开头
	item := HistoryItem{
		ToolName:   toolName,
		ToolPath:   toolPath,
		LastParams: params,
		Timestamp:  0, // TODO: 添加时间戳
	}

	s.data.RecentTools = append([]HistoryItem{item}, s.data.RecentTools...)

	// 只保留最近10个
	if len(s.data.RecentTools) > 10 {
		s.data.RecentTools = s.data.RecentTools[:10]
	}

	// 记录到 ToolHistory
	if s.data.ToolHistory == nil {
		s.data.ToolHistory = make(map[string][]map[string]string)
	}
	history := s.data.ToolHistory[toolPath]
	// 避免连续重复
	if len(history) == 0 || !isMapEqual(history[0], params) {
		history = append([]map[string]string{params}, history...)
		if len(history) > 50 {
			history = history[:50]
		}
		s.data.ToolHistory[toolPath] = history
	}
}

func isMapEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// GetRecentTools 获取最近使用的工具
func (s *Storage) GetRecentTools() []HistoryItem {
	return s.data.RecentTools
}

// GetToolHistory 获取某个工具的历史执行参数
func (s *Storage) GetToolHistory(toolPath string) []map[string]string {
	if s.data.ToolHistory == nil {
		return nil
	}
	return s.data.ToolHistory[toolPath]
}

// GetNodeState 获取节点展开状态，如果不存在则返回 defaultVal
func (s *Storage) GetNodeState(nodeID string, defaultVal bool) bool {
	if s.data.NodeStates == nil {
		s.data.NodeStates = make(map[string]bool)
	}
	if state, exists := s.data.NodeStates[nodeID]; exists {
		return state
	}
	return defaultVal
}

// SetNodeState 设置节点展开状态
func (s *Storage) SetNodeState(nodeID string, expanded bool) {
	if s.data.NodeStates == nil {
		s.data.NodeStates = make(map[string]bool)
	}
	s.data.NodeStates[nodeID] = expanded
	_ = s.Save()
}

// ClearNodeStates 清空展开状态并恢复默认
func (s *Storage) ClearNodeStates() {
	s.data.NodeStates = make(map[string]bool)
	_ = s.Save()
}
