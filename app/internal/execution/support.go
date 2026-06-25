package execution

import (
	"bytes"
	"strings"
	"sync"

	"fire-salamander-desktop/internal/shared"
)

type TaskLogEvent struct {
	TaskID   string `json:"taskId"`
	Message  string `json:"message"`
	Recorded int64  `json:"recorded"`
}

type taskEventWriter struct {
	taskID  string
	emitter shared.TaskEventEmitter
	mu      sync.Mutex
	buffer  bytes.Buffer
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
		w.emitter.EmitTaskLog(w.taskID, strings.TrimRight(line, "\r\n"))
	}

	return len(p), nil
}

func (w *taskEventWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.buffer.Len() == 0 {
		return
	}
	w.emitter.EmitTaskLog(w.taskID, strings.TrimRight(w.buffer.String(), "\r\n"))
	w.buffer.Reset()
}
