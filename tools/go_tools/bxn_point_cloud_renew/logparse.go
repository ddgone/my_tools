package bxn_point_cloud_renew

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// logLineRe 老项目日志中提取 field_task_id 与起始时间戳的行格式。
var logLineRe = regexp.MustCompile(
	`(?:完成一个合轨分组|Finalize merged group|Build merged groups for field-task bucket)` +
		`.*?field_task_id=([^\s,，]+)` +
		`.*?(?:起始时间戳ms|first_timestamp_ms|earliest_timestamp_ms)=(\d+)`,
)

// chinaLoc 返回东八区时区，加载失败时退化为固定偏移。
func chinaLoc() *time.Location {
	if loc, err := time.LoadLocation("Asia/Shanghai"); err == nil {
		return loc
	}
	return time.FixedZone("CST", 8*3600)
}

// mergeSources 新项目 debug 目录下 merge_sources.json 的结构。
type mergeSources struct {
	Tracks []struct {
		FieldTaskID string `json:"field_task_id"`
	} `json:"tracks"`
}

// packageToUnixSec 从包名前缀解析出 Unix 秒级时间戳。
func packageToUnixSec(pkgName string) (int64, error) {
	re := regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}-\d{2}-\d{2}-\d{2})`)
	m := re.FindStringSubmatch(pkgName)
	if m == nil {
		return 0, fmt.Errorf("no timestamp found in package name: %s", pkgName)
	}
	t, err := time.ParseInLocation("2006-01-02-15-04-05", m[1], chinaLoc())
	if err != nil {
		return 0, fmt.Errorf("parse time failed: %w", err)
	}
	return t.Unix(), nil
}

// findFieldTaskID 在老项目日志中查找起始时间戳对应包的 field_task_id。
func findFieldTaskID(logPath string, targetSec int64) (string, error) {
	f, err := os.Open(logPath)
	if err != nil {
		return "", fmt.Errorf("open log failed: %w", err)
	}
	defer f.Close()

	scanner := NewLineScanner(f)
	seen := make(map[string]bool)
	var matched []string

	for scanner.Scan() {
		line := scanner.Text()
		m := logLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		startMs, err := strToInt64(m[2])
		if err != nil {
			continue
		}
		if startMs/1000 == targetSec {
			tid := m[1]
			if !seen[tid] {
				seen[tid] = true
				matched = append(matched, tid)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read log error: %w", err)
	}
	if len(matched) == 0 {
		return "", fmt.Errorf("no matching field_task_id (target_sec=%d)", targetSec)
	}
	return strings.Join(matched, ","), nil
}

// findMatchingGroup 在新项目 debug 目录中查找包含目标 field_task_id 的合轨分组。
func findMatchingGroup(debugDir string, targetFieldTaskID string) (string, error) {
	entries, err := os.ReadDir(debugDir)
	if err != nil {
		return "", fmt.Errorf("read debug dir failed: %w", err)
	}

	targetIDs := make(map[string]bool)
	for _, id := range strings.Split(targetFieldTaskID, ",") {
		id = strings.TrimSpace(id)
		if id != "" {
			targetIDs[id] = true
		}
	}

	var warns []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		groupName := entry.Name()
		jsonPath := filepath.Join(debugDir, groupName, "merge_sources.json")
		data, err := os.ReadFile(jsonPath)
		if err != nil {
			warns = append(warns, fmt.Sprintf("cannot read %s: %v", groupName, err))
			continue
		}
		var ms mergeSources
		if err := json.Unmarshal(data, &ms); err != nil {
			warns = append(warns, fmt.Sprintf("JSON parse error %s: %v", groupName, err))
			continue
		}
		for _, track := range ms.Tracks {
			if targetIDs[track.FieldTaskID] {
				return groupName, nil
			}
		}
	}

	errMsg := fmt.Sprintf("no match for field_task_id=%s (searched %d dirs)", targetFieldTaskID, len(entries))
	for _, w := range warns {
		errMsg += "\n  " + w
	}
	return "", fmt.Errorf("%s", errMsg)
}

// strToInt64 只含数字的字符串转 int64。
func strToInt64(s string) (int64, error) {
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("non-digit char: %c", c)
		}
		n = n*10 + int64(c-'0')
	}
	return n, nil
}

// ==================== 逐行扫描器 ====================

const defaultBufSize = 256 * 1024

type lineScanner struct {
	file *os.File
	buf  []byte
	pos  int
	end  int
	eof  bool
	err  error
	text string
}

func NewLineScanner(f *os.File) *lineScanner {
	return &lineScanner{
		file: f,
		buf:  make([]byte, defaultBufSize),
	}
}

func (s *lineScanner) Scan() bool {
	if s.err != nil {
		return false
	}
	for {
		for i := s.pos; i < s.end; i++ {
			if s.buf[i] == '\n' {
				s.text = string(s.buf[s.pos:i])
				s.text = strings.TrimSuffix(s.text, "\r")
				s.pos = i + 1
				return true
			}
		}
		if s.eof {
			if s.pos < s.end {
				s.text = string(s.buf[s.pos:s.end])
				s.pos = s.end
				return true
			}
			return false
		}
		if s.pos > 0 {
			copy(s.buf, s.buf[s.pos:s.end])
			s.end -= s.pos
			s.pos = 0
		}
		n, err := s.file.Read(s.buf[s.end:])
		if n > 0 {
			s.end += n
		}
		if err == io.EOF {
			s.eof = true
		} else if err != nil {
			s.err = err
			return false
		}
	}
}

func (s *lineScanner) Text() string {
	return s.text
}

func (s *lineScanner) Err() error {
	return s.err
}
