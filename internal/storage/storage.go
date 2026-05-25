package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type HistoryItem struct {
	ToolName   string            `json:"tool_name"`
	ToolPath   string            `json:"tool_path"`
	LastParams map[string]string `json:"last_params"`
	Timestamp  int64             `json:"timestamp"`
}

type UserData struct {
	RecentTools          []HistoryItem                  `json:"recent_tools"`
	NodeStates           map[string]bool                `json:"node_states"`
	ToolHistory          map[string][]map[string]string `json:"tool_history"`
	ShowVerboseShortcuts bool                           `json:"show_verbose_shortcuts"`
	LogExportDir         string                         `json:"log_export_dir"`
	RecentToolsCount     int                            `json:"recent_tools_count"`
	HistoryRetention     int                            `json:"history_retention"`
	ConfirmExit          bool                           `json:"confirm_exit"`
	DefaultPythonPath    string                         `json:"default_python_path"`
	AutoWordWrap         bool                           `json:"auto_word_wrap"`
	AutoExpandAll        bool                           `json:"auto_expand_all"`
	BGMEnabled           bool                           `json:"bgm_enabled"`
	Favorites            []string                       `json:"favorites"`
}

type Storage struct {
	mu       sync.RWMutex
	dataFile string
	data     *UserData
}

func NewStorage() *Storage {
	homeDir, _ := os.UserHomeDir()
	dataDir := filepath.Join(homeDir, ".my_tools")
	_ = os.MkdirAll(dataDir, 0755)

	return &Storage{
		dataFile: filepath.Join(dataDir, "user_data.json"),
		data: &UserData{
			NodeStates:        make(map[string]bool),
			LogExportDir:      "my_tools_logs",
			RecentToolsCount:  3,
			HistoryRetention:  50,
			DefaultPythonPath: "python",
			AutoWordWrap:      true,
		},
	}
}

func (s *Storage) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if s.data.NodeStates == nil {
		s.data.NodeStates = make(map[string]bool)
	}
	if s.data.ToolHistory == nil {
		s.data.ToolHistory = make(map[string][]map[string]string)
	}
	if s.data.Favorites == nil {
		s.data.Favorites = make([]string, 0)
	}

	if err := json.Unmarshal(data, s.data); err != nil {
		return err
	}

	if s.data.LogExportDir == "" {
		s.data.LogExportDir = "my_tools_logs"
	}
	if s.data.RecentToolsCount <= 0 {
		s.data.RecentToolsCount = 3
	}
	if s.data.HistoryRetention <= 0 {
		s.data.HistoryRetention = 50
	}
	if s.data.DefaultPythonPath == "" {
		s.data.DefaultPythonPath = "python"
	}
	if s.data.Favorites == nil {
		s.data.Favorites = make([]string, 0)
	}
	if s.data.NodeStates == nil {
		s.data.NodeStates = make(map[string]bool)
	}
	if s.data.ToolHistory == nil {
		s.data.ToolHistory = make(map[string][]map[string]string)
	}

	return nil
}

func (s *Storage) Save() error {
	s.mu.RLock()
	data, err := json.MarshalIndent(s.data, "", "  ")
	s.mu.RUnlock()
	if err != nil {
		return err
	}

	return os.WriteFile(s.dataFile, data, 0644)
}

func (s *Storage) AddRecentTool(toolName, toolPath string, params map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, item := range s.data.RecentTools {
		if item.ToolPath == toolPath {
			s.data.RecentTools = append(s.data.RecentTools[:i], s.data.RecentTools[i+1:]...)
			break
		}
	}

	item := HistoryItem{
		ToolName:   toolName,
		ToolPath:   toolPath,
		LastParams: params,
		Timestamp:  time.Now().Unix(),
	}

	s.data.RecentTools = append([]HistoryItem{item}, s.data.RecentTools...)

	if len(s.data.RecentTools) > 10 {
		s.data.RecentTools = s.data.RecentTools[:10]
	}

	if s.data.ToolHistory == nil {
		s.data.ToolHistory = make(map[string][]map[string]string)
	}
	history := s.data.ToolHistory[toolPath]
	if len(history) == 0 || !isMapEqual(history[0], params) {
		history = append([]map[string]string{params}, history...)
		if len(history) > s.data.HistoryRetention {
			history = history[:s.data.HistoryRetention]
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

func (s *Storage) GetRecentTools() []HistoryItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.RecentTools
}

func (s *Storage) GetToolHistory(toolPath string) []map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.data.ToolHistory == nil {
		return nil
	}
	return s.data.ToolHistory[toolPath]
}

func (s *Storage) GetNodeState(nodeID string, defaultVal bool) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.data.NodeStates == nil {
		s.data.NodeStates = make(map[string]bool)
	}
	if state, exists := s.data.NodeStates[nodeID]; exists {
		return state
	}
	return defaultVal
}

func (s *Storage) GetShowVerboseShortcuts() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.ShowVerboseShortcuts
}

func (s *Storage) SetShowVerboseShortcuts(show bool) {
	s.mu.Lock()
	s.data.ShowVerboseShortcuts = show
	s.mu.Unlock()
	_ = s.Save()
}

func (s *Storage) SetNodeState(nodeID string, expanded bool) {
	s.mu.Lock()
	if s.data.NodeStates == nil {
		s.data.NodeStates = make(map[string]bool)
	}
	s.data.NodeStates[nodeID] = expanded
	s.mu.Unlock()
	_ = s.Save()
}

func (s *Storage) ClearNodeStates() {
	s.mu.Lock()
	s.data.NodeStates = make(map[string]bool)
	s.mu.Unlock()
	_ = s.Save()
}

func (s *Storage) Reset() error {
	return os.Remove(s.dataFile)
}

func (s *Storage) GetLogExportDir() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.LogExportDir
}

func (s *Storage) SetLogExportDir(dir string) {
	s.mu.Lock()
	s.data.LogExportDir = dir
	s.mu.Unlock()
	_ = s.Save()
}

func (s *Storage) GetRecentToolsCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.RecentToolsCount
}

func (s *Storage) SetRecentToolsCount(count int) {
	s.mu.Lock()
	s.data.RecentToolsCount = count
	s.mu.Unlock()
	_ = s.Save()
}

func (s *Storage) GetHistoryRetention() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.HistoryRetention
}

func (s *Storage) SetHistoryRetention(retention int) {
	s.mu.Lock()
	s.data.HistoryRetention = retention
	s.mu.Unlock()
	_ = s.Save()
}

func (s *Storage) GetConfirmExit() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.ConfirmExit
}

func (s *Storage) SetConfirmExit(confirm bool) {
	s.mu.Lock()
	s.data.ConfirmExit = confirm
	s.mu.Unlock()
	_ = s.Save()
}

func (s *Storage) GetDefaultPythonPath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.DefaultPythonPath
}

func (s *Storage) SetDefaultPythonPath(path string) {
	s.mu.Lock()
	s.data.DefaultPythonPath = path
	s.mu.Unlock()
	_ = s.Save()
}

func (s *Storage) GetAutoWordWrap() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.AutoWordWrap
}

func (s *Storage) SetAutoWordWrap(wrap bool) {
	s.mu.Lock()
	s.data.AutoWordWrap = wrap
	s.mu.Unlock()
	_ = s.Save()
}

func (s *Storage) GetAutoExpandAll() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.AutoExpandAll
}

func (s *Storage) SetAutoExpandAll(expand bool) {
	s.mu.Lock()
	s.data.AutoExpandAll = expand
	s.mu.Unlock()
	_ = s.Save()
}

func (s *Storage) GetFavorites() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.data.Favorites == nil {
		return nil
	}
	return s.data.Favorites
}

func (s *Storage) AddFavorite(toolID string) {
	s.mu.Lock()
	for _, id := range s.data.Favorites {
		if id == toolID {
			s.mu.Unlock()
			return
		}
	}
	s.data.Favorites = append(s.data.Favorites, toolID)
	s.mu.Unlock()
	_ = s.Save()
}

func (s *Storage) RemoveFavorite(toolID string) {
	s.mu.Lock()
	for i, id := range s.data.Favorites {
		if id == toolID {
			s.data.Favorites = append(s.data.Favorites[:i], s.data.Favorites[i+1:]...)
			break
		}
	}
	s.mu.Unlock()
	_ = s.Save()
}

func (s *Storage) IsFavorite(toolID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, id := range s.data.Favorites {
		if id == toolID {
			return true
		}
	}
	return false
}

func (s *Storage) RawData() *UserData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data
}

func (s *Storage) GetBGMEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.BGMEnabled
}

func (s *Storage) SetBGMEnabled(enabled bool) {
	s.mu.Lock()
	s.data.BGMEnabled = enabled
	s.mu.Unlock()
	_ = s.Save()
}
