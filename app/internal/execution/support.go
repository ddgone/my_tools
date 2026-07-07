package execution

import (
	"bytes"
	"log"
	"strings"
	"sync"

	"fire-salamander-desktop/internal/shared"
)

type TaskLogEvent struct {
	TaskID   string `json:"taskId"`
	Message  string `json:"message"`
	Recorded int64  `json:"recorded"`
}

// LogSaver persists individual log lines (e.g., to SQLite).
type LogSaver interface {
	AppendLogLine(taskID, line string) error
}

type taskEventWriter struct {
	taskID  string
	emitter shared.TaskEventEmitter
	saver   LogSaver
	mu      sync.Mutex
	buffer  bytes.Buffer
}

func newTaskEventWriter(taskID string, emitter shared.TaskEventEmitter) *taskEventWriter {
	return &taskEventWriter{taskID: taskID, emitter: emitter}
}

func (w *taskEventWriter) SetLogSaver(saver LogSaver) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.saver = saver
}

func (w *taskEventWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buffer.Write(p)
	for {
		line, err := w.buffer.ReadString('\n')
		if err != nil {
			w.buffer.WriteString(line)
			break
		}
		trimmed := strings.TrimRight(line, "\r\n")
		w.emitter.EmitTaskLog(w.taskID, trimmed)
		if w.saver != nil {
			if err := w.saver.AppendLogLine(w.taskID, trimmed); err != nil {
				log.Printf("[exec] save log line: %v", err)
			}
		}
	}

	return len(p), nil
}

func (w *taskEventWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.buffer.Len() == 0 {
		return
	}
	remaining := w.buffer.String()
	w.emitter.EmitTaskLog(w.taskID, strings.TrimRight(remaining, "\r\n"))
	if w.saver != nil {
		if err := w.saver.AppendLogLine(w.taskID, remaining); err != nil {
			log.Printf("[exec] flush log line: %v", err)
		}
	}
	w.buffer.Reset()
}
