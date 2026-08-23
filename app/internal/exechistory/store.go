package exechistory

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"fire-salamander-desktop/internal/runtimeenv"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// ExecRecord is a single execution record persisted to DB.
type ExecRecord struct {
	ID           string `json:"id" gorm:"primaryKey"`
	ToolID       string `json:"toolId" gorm:"index"`
	ToolName     string `json:"toolName"`
	Args         string `json:"args"`
	Status       string `json:"status"` // "success" | "error" | "canceled"
	Target       string `json:"target"` // "local" | "remote:host"
	RemoteConnID string `json:"remoteConnId,omitempty"`
	StartedAt    int64  `json:"startedAt"`
	EndedAt      int64  `json:"endedAt"`
}

// LogLine is a single line of execution log.
type LogLine struct {
	ID        uint   `gorm:"primaryKey;autoIncrement"`
	TaskID    string `gorm:"index;not null"`
	Line      string `gorm:"not null"`
	CreatedAt int64  `gorm:"autoCreateTime:milli"`
}

// Store persists execution records and logs using SQLite.
type Store struct {
	db      *gorm.DB
	mu      sync.Mutex
	logsDir string // kept for LogFilePath compatibility
}

// NewStore creates a new Store using the runtime layout.
func NewStore() (*Store, error) {
	layout, err := runtimeenv.ResolveLayout()
	if err != nil {
		return nil, fmt.Errorf("解析运行时目录失败: %w", err)
	}
	if err := layout.Ensure(); err != nil {
		return nil, fmt.Errorf("初始化运行时目录失败: %w", err)
	}

	dbPath := filepath.Join(layout.ConfigDir(), "execution.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("打开执行记录数据库失败: %w", err)
	}

	// Enable WAL mode and busy timeout for concurrent access.
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA busy_timeout=5000")
	db.Exec("PRAGMA synchronous=NORMAL")

	// Auto-migrate tables.
	if err := db.AutoMigrate(&ExecRecord{}, &LogLine{}); err != nil {
		return nil, fmt.Errorf("自动迁移表结构失败: %w", err)
	}

	return &Store{
		db:      db,
		logsDir: layout.LogsDir(),
	}, nil
}

// Load is a no-op with SQLite (data is persistent by design).
func (s *Store) Load() error {
	return nil
}

// Close releases the underlying database connection.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Append adds a record to the database.
func (s *Store) Append(record *ExecRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Create(record).Error
}

// UpdateStatus updates the status and end time of a record.
func (s *Store) UpdateStatus(taskID, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Model(&ExecRecord{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"status":   status,
		"ended_at": time.Now().UnixMilli(),
	}).Error
}

// List returns all records in reverse chronological order.
func (s *Store) List() []*ExecRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	var records []*ExecRecord
	s.db.Order("started_at desc").Find(&records)
	return records
}

// AppendLogLine writes a single log line to the database.
func (s *Store) AppendLogLine(taskID, line string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Create(&LogLine{TaskID: taskID, Line: line}).Error
}

// ReadLog reads all log lines for a given task ID.
func (s *Store) ReadLog(taskID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var lines []LogLine
	if err := s.db.Where("task_id = ?", taskID).Order("id asc").Find(&lines).Error; err != nil {
		return "", err
	}

	result := ""
	for i, l := range lines {
		if i > 0 {
			result += "\n"
		}
		result += l.Line
	}
	return result, nil
}

// LogFilePath returns the log file path for backwards compatibility (unused with SQLite).
func (s *Store) LogFilePath(taskID string) string {
	return filepath.Join(s.logsDir, "task_"+taskID+".log")
}
