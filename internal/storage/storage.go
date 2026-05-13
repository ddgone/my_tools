package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// HistoryItem 历史记录项
type HistoryItem struct {
	ToolName    string            `json:"tool_name"`
	ToolPath    string            `json:"tool_path"`
	LastParams  map[string]string `json:"last_params"`
	Timestamp   int64             `json:"timestamp"`
}

// UserData 用户数据
type UserData struct {
	RecentTools   []HistoryItem `json:"recent_tools"`
	ExpandedPaths []string      `json:"expanded_paths"` // 展开的目录路径
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
	os.MkdirAll(dataDir, 0755)
	
	return &Storage{
		dataFile: filepath.Join(dataDir, "user_data.json"),
		data:     &UserData{},
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
}

// GetRecentTools 获取最近使用的工具
func (s *Storage) GetRecentTools() []HistoryItem {
	return s.data.RecentTools
}

// IsExpanded 检查路径是否展开
func (s *Storage) IsExpanded(path string) bool {
	for _, p := range s.data.ExpandedPaths {
		if p == path {
			return true
		}
	}
	return false
}

// ToggleExpand 切换展开状态
func (s *Storage) ToggleExpand(path string) {
	if s.IsExpanded(path) {
		// 收起
		for i, p := range s.data.ExpandedPaths {
			if p == path {
				s.data.ExpandedPaths = append(s.data.ExpandedPaths[:i], s.data.ExpandedPaths[i+1:]...)
				break
			}
		}
	} else {
		// 展开
		s.data.ExpandedPaths = append(s.data.ExpandedPaths, path)
	}
}

// Expand 展开路径
func (s *Storage) Expand(path string) {
	if !s.IsExpanded(path) {
		s.data.ExpandedPaths = append(s.data.ExpandedPaths, path)
	}
}

// Collapse 收起路径
func (s *Storage) Collapse(path string) {
	for i, p := range s.data.ExpandedPaths {
		if p == path {
			s.data.ExpandedPaths = append(s.data.ExpandedPaths[:i], s.data.ExpandedPaths[i+1:]...)
			break
		}
	}
}
