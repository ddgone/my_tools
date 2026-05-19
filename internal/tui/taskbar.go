package tui

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type TaskStatus int

const (
	StatusWaiting TaskStatus = iota
	StatusRunning
	StatusSuccess
	StatusFailed
)

func (s TaskStatus) Icon() string {
	switch s {
	case StatusWaiting:
		return "⏳"
	case StatusRunning:
		return "●"
	case StatusSuccess:
		return "✓"
	case StatusFailed:
		return "⬤"
	}
	return "?"
}

func (s TaskStatus) Label() string {
	switch s {
	case StatusWaiting:
		return "排队"
	case StatusRunning:
		return "执行"
	case StatusSuccess:
		return "完成"
	case StatusFailed:
		return "失败"
	}
	return "?"
}

func (s TaskStatus) Color() tcell.Color {
	switch s {
	case StatusWaiting:
		return tcell.ColorGray
	case StatusRunning:
		return tcell.ColorYellow
	case StatusSuccess:
		return tcell.ColorGreen
	case StatusFailed:
		return tcell.ColorRed
	}
	return tcell.ColorWhite
}

func (s TaskStatus) ColorTag() string {
	switch s {
	case StatusWaiting:
		return "gray"
	case StatusRunning:
		return "yellow"
	case StatusSuccess:
		return "green"
	case StatusFailed:
		return "red"
	}
	return "white"
}

type Task struct {
	ID        string
	ToolID    string
	ToolName  string
	Cmd       string
	Env       string
	Status    TaskStatus
	Output    string
	CreatedAt time.Time
	EndedAt   time.Time
	Cancel    context.CancelFunc
}

type outputTracker struct {
	io.Writer
	wroteBytes bool
	Task       *Task
	ShownTask  **Task
}

func (ot *outputTracker) Write(p []byte) (int, error) {
	if len(p) > 0 {
		ot.wroteBytes = true
	}
	if ot.Task != nil {
		ot.Task.Output += string(p)
	}
	if ot.ShownTask == nil || *ot.ShownTask != ot.Task {
		return len(p), nil
	}
	n, err := ot.Writer.Write(p)
	return n, err
}

type TaskBarState struct {
	mu            sync.Mutex
	ToolID        string
	Tasks         []*Task
	ActiveIdx     int
	Visible       bool
	UndoTasks     []*Task
	UndoActiveIdx int
	ResultsViewed bool // 是否已在工具页面查看过结果
}

func (a *App) ensureTaskBar(ui *TermUIState, toolID string) (*TaskBarState, *tview.Flex) {
	if bar, ok := a.TaskBars[toolID]; ok {
		if ui.TaskList == nil && len(bar.Tasks) > 0 {
			a.createTaskBarUI(ui, toolID)
			a.populateTaskList(ui, bar)
			a.showTaskBar(ui, bar)
		}
		return bar, ui.TaskBar
	}

	bar := &TaskBarState{ToolID: toolID}
	a.TaskBars[toolID] = bar

	if len(bar.Tasks) > 0 {
		a.createTaskBarUI(ui, toolID)
		a.populateTaskList(ui, bar)
		a.showTaskBar(ui, bar)
	} else {
		a.createTaskBarUI(ui, toolID)
	}

	return bar, ui.TaskBar
}

func (a *App) createTaskBarUI(ui *TermUIState, toolID string) {
	taskList := tview.NewList().
		SetMainTextColor(tcell.ColorWhite).
		SetSelectedBackgroundColor(colorBgLight).
		SetSelectedTextColor(tcell.ColorYellow).
		ShowSecondaryText(false)

	statusText := tview.NewTextView().
		SetDynamicColors(true).
		SetTextColor(tcell.ColorGray)
	statusText.SetBorderPadding(0, 0, 0, 1)

	taskBarFlex := tview.NewFlex().SetDirection(tview.FlexRow)
	taskBarFlex.SetBorder(true).
		SetTitle(a.getTitleWithShortcut("📋 任务", "Ctrl+B:隐藏", false)).
		SetTitleColor(tcell.ColorOrange).
		SetBorderColor(tcell.ColorDarkGray)
	taskBarFlex.AddItem(taskList, 0, 1, true)
	taskBarFlex.AddItem(statusText, 1, 1, false)

	taskList.SetFocusFunc(func() {
		taskBarFlex.SetBorderColor(tcell.ColorOrange)
	})
	taskList.SetBlurFunc(func() {
		taskBarFlex.SetBorderColor(tcell.ColorDarkGray)
	})

	taskList.SetSelectedFunc(func(index int, _ string, _ string, _ rune) {
		bar := a.TaskBars[toolID]
		if bar == nil || index >= len(bar.Tasks) {
			return
		}
		bar.mu.Lock()
		bar.ActiveIdx = index
		task := bar.Tasks[index]
		bar.mu.Unlock()

		ui.Output.Clear()
		_, _ = tview.ANSIWriter(ui.Output).Write([]byte(task.Output))
		ui.Output.ScrollToEnd()

		ui.ShownTask = task

		if task.Status == StatusRunning {
			ui.Executing = true
		} else if index == len(bar.Tasks)-1 {
			ui.Executing = false
		} else {
			ui.Executing = true
		}
		if ui.Input != nil {
			ui.Input.SetDisabled(ui.Executing)
		}
		a.refreshTaskList(ui, bar)
	})

	taskList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlC {
			bar := a.TaskBars[toolID]
			if bar != nil && len(bar.Tasks) > 0 {
				task := bar.Tasks[bar.ActiveIdx]
				if task.Cancel != nil {
					task.Cancel()
				}
			}
			return nil
		}
		return event
	})

	ui.TaskList = taskList
	ui.TaskBar = taskBarFlex
	ui.TaskStatus = statusText
}

func (a *App) showTaskBar(ui *TermUIState, bar *TaskBarState) {
	if bar.Visible {
		return
	}
	bar.mu.Lock()
	bar.Visible = true
	bar.mu.Unlock()

	if ui.TaskBar != nil {
		ui.OutputRow.AddItem(ui.TaskBar, 26, 1, false)
	}
	ui.OutputRow.ResizeItem(ui.Output, 0, 3)
}

func (a *App) hideTaskBar(ui *TermUIState, bar *TaskBarState) {
	if !bar.Visible {
		return
	}
	bar.mu.Lock()
	bar.Visible = false
	bar.mu.Unlock()

	if ui.TaskBar != nil {
		ui.OutputRow.RemoveItem(ui.TaskBar)
	}
	ui.OutputRow.ResizeItem(ui.Output, 0, 1)
}

func (a *App) refreshTaskList(ui *TermUIState, bar *TaskBarState) {
	if ui.TaskList == nil {
		return
	}
	a.populateTaskList(ui, bar)
	status := fmt.Sprintf(" 共%d", len(bar.Tasks))
	ui.TaskStatus.SetText(status)
}

func (a *App) populateTaskList(ui *TermUIState, bar *TaskBarState) {
	ui.TaskList.Clear()
	for i, task := range bar.Tasks {
		prefix := "  "
		if i == bar.ActiveIdx {
			prefix = "▶ "
		}
		cmd := task.Cmd
		if len(cmd) > 20 {
			cmd = cmd[:20] + "..."
		}
		itemText := fmt.Sprintf("%s[%s]%s %s[white]",
			prefix,
			task.Status.ColorTag(),
			task.Status.Icon(),
			cmd)
		ui.TaskList.AddItem(itemText, "", 0, nil)
	}
	if len(bar.Tasks) > 0 {
		ui.TaskList.SetCurrentItem(bar.ActiveIdx)
	}
}

func (a *App) removeTask(bar *TaskBarState, target *Task) {
	for i, t := range bar.Tasks {
		if t == target {
			bar.Tasks = append(bar.Tasks[:i], bar.Tasks[i+1:]...)
			if bar.ActiveIdx >= len(bar.Tasks) {
				bar.ActiveIdx = len(bar.Tasks) - 1
			}
			return
		}
	}
}

var taskFailPatterns = []string{
	`总计:\s*0\s*个`,
	`total:\s*0`,
	`共处理\s*0\s*个`,
	`unmarshal.*error`,
	`invalid character`,
	`no such file`,
	`cannot find`,
}

func parseTaskResult(task *Task, err error) TaskStatus {
	if err != nil {
		return StatusFailed
	}
	lower := strings.ToLower(task.Output)
	for _, p := range taskFailPatterns {
		if matched, _ := regexp.MatchString(p, lower); matched {
			return StatusFailed
		}
	}
	return StatusSuccess
}

func (a *App) taskStarted(ui *TermUIState) {
	ui.Executing = true
	if ui.Input != nil {
		ui.Input.SetDisabled(true)
	}
	if ui.Output != nil {
		a.TviewApp.SetFocus(ui.Output)
	}
}

func (a *App) taskFinished(ui *TermUIState) {
	ui.Executing = false
	if ui.Input != nil {
		ui.Input.SetDisabled(false)
	}
	if ui.Input != nil && ui.Output != nil && ui.Output.HasFocus() {
		a.TviewApp.SetFocus(ui.Input)
	}
}
