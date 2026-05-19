# 任务栏 (TaskBar) 功能开发文档

## 设计摘要

- **任务栏**：在工具执行页面终端输出区的右侧，额外切出的一个紧凑面板
- 无任务时自动隐藏，有任务时自动出现
- 名称统称"任务栏"，不叫"侧边栏"
- 工具没真正跑起来（输入错误等前置校验失败）不创建任务，不入任务栏
- Ctrl+H 历史弹窗移除，融合进任务栏
- 最多支持同时 10 个任务

## 数据结构

```go
// internal/tui/app.go 中新增

type TaskStatus int
const (
    StatusWaiting TaskStatus = iota // 排队等待（槽位满时）
    StatusRunning                    // 执行中
    StatusSuccess                    // 完成
    StatusFailed                     // 异常失败（运行时出错，产生了真实输出）
)

type Task struct {
    ID        string
    ToolID    string
    ToolName  string
    Cmd       string              // 执行的命令文本
    Env       string              // Python解释器路径（仅Python工具）
    Status    TaskStatus
    Output    string              // 完整输出
    CreatedAt time.Time
    StartedAt time.Time
    EndedAt   time.Time
    Cancel    context.CancelFunc
    wroteAny  bool                // 内部：是否写过输出到流
}

type TaskBarState struct {
    mu          sync.Mutex
    toolID      string
    tasks       []*Task
    activeIdx   int               // 当前终端输出展示的是哪个任务
    visible     bool
    focus       bool              // 任务栏自身是否获得焦点
}
```

App 结构体新增：

```go
type App struct {
    // ... 原有字段
    taskBars    map[string]*TaskBarState  // toolID -> task bar
    activeBar   string                    // 当前页面所属 toolID
}
```

## 错误过滤

使用 `outputTracker` 包装输出流：

```go
type outputTracker struct {
    io.Writer
    wroteBytes bool
}

func (ot *outputTracker) Write(p []byte) (int, error) {
    if len(p) > 0 {
        ot.wroteBytes = true
    }
    return ot.Writer.Write(p)
}
```

在执行 goroutine 中：

```go
ot := &outputTracker{Writer: tview.ANSIWriter(outputView)}
err := run(runCtx, cmdText, ot)

if err != nil && !ot.wroteBytes {
    // 这是前置校验错误，工具没跑起来
    // 终端输出显示错误，但不创建任务
    // 把之前临时创建的任务对象丢弃
    return
}

// 这是真实任务（完成或运行中 crash）
// 创建/更新任务对象，加入任务栏
```

---

## Phase 1 → 任务栏骨架 + 任务创建

### 目标

实现任务栏的显示/隐藏、任务创建、状态更新、Ctrl+B 焦点切换。此阶段不实现任务切换和清理。

### 改动清单

#### 1. `internal/tui/app.go` — 全部核心改动

**a. 数据结构**

- 新增 `TaskStatus` 枚举、`Task` 结构体、`outputTracker` 结构体
- `App` 结构体新增 `taskBars map[string]*TaskBarState`、`activeBar string`

**b. TaskBar UI 组件**

定义一个 `newTaskBar(toolID string) *tview.Flex`，内容：

```
┌── 📋 任务 ────┐
│ ▶ ● -input...│
│   ✓ -input...│
│              │
│ 共 2         │
└──────────────┘
```

- 固定宽度 26 列（包含边框）
- 使用 `tview.Flex` + `tview.List` 实现任务列表
- 顶部标题行：`📋 任务`（未来可加工具名后缀）
- 底部状态行：`共 N`

**c. ShowTerminal / ShowPythonTerminal 改造**

布局从：
```
┌──────────────────────────────┐
│ 使用说明 (全宽)              │
├──────────────────────────────┤
│ 终端输出 (全宽)              │
├──────────────────────────────┤
│ 命令行输入 (全宽)            │
└──────────────────────────────┘
```

改为：
```
┌────────────────────────────┬──────┐
│ 使用说明                   │      │
├────────────────────────────┤      │
│ 终端输出                   │ 任务 │
│                            │ 栏   │
├────────────────────────────┤      │
│ 命令行输入                 │      │
└────────────────────────────┴──────┘
```

用嵌套 Flex 实现：

```go
outputRow := tview.NewFlex().SetDirection(tview.FlexColumn)
outputRow.AddItem(outputView, 0, 3, false)
// taskBar 组件，一开始 hidden (高度=0)
outputRow.AddItem(taskBar, 26, 0, false)

flex.AddItem(usageView, 0, 1, false)
flex.AddItem(outputRow, 0, 3, false)  // 使用输出行
flex.AddItem(inputField, 5, 1, true)
```

**d. 执行逻辑改造**

现有 Enter 处理逻辑中：
1. 创建临时 Task 对象
2. 使用 `outputTracker` 包裹输出 writer
3. Goroutine 执行 `run()`
4. 如果 err != nil 且 `!ot.wroteBytes` → 丢弃任务，不加入任务栏，只在终端输出显示错误
5. 否则 → 将 Task 加入 `taskBar.tasks`，`activeIdx` 指向新任务，`visible = true`
6. 更新任务栏 UI
7. 任务栏可见时，重新布局

**e. 任务栏显示/隐藏**

- `visible=false` 时：任务栏 Flex 子项高度为 0（`ResizeItem(item, 0, 0)`）
- `visible=true` 时：任务栏 Flex 子项恢复 `ResizeItem(item, 26, 0)`，高度为整个可用区域

实际上用 `Flex.ResizeItem()` 控制即可，不需要重建布局。

**f. Ctrl+B 快捷键**

在 `flex.SetInputCapture` 中新增：

```go
if event.Key() == tcell.KeyCtrlB && barState.visible {
    if barState.focus {
        // 焦点回到输入框
        a.TviewApp.SetFocus(inputField)
        barState.focus = false
    } else {
        // 焦点进入任务栏
        a.TviewApp.SetFocus(taskBarList)
        barState.focus = true
    }
    return nil
}
```

**g. 焦点轮转 Tab 更新**

原 `focusables` 改为：`[usageView, outputView, taskBarList, inputField]`

**h. Enter 创建任务的交互**

按下 Enter 后：
- 还在执行中的任务 → 追加到任务栏列表底部
- 终端输出始终展示最新执行的任务（当前 activeIdx 指向最新的）
- 输出实时滚动，用户看到的是最新任务的输出

#### 2. `pkg/framework/framework.go` — 无改动

（不依赖框架层的改动，全部在 UI 层实现）

#### 3. 测试验证 (Phase 1)

- 进入任意工具 → 输入错误命令（如不存在的路径）→ 终端输出显示错误，任务栏不出现
- 输入正确命令 → 任务栏出现，显示执行中 → 完成后变绿
- 执行多个任务 → 任务栏逐条显示
- 清理所有任务后重新进入 → 任务栏隐藏
- Ctrl+B 在输入框和任务栏之间切换焦点

---

## Phase 2 → 任务切换 + 清理 + 体验完善

### 目标

实现任务列表中切换到旧任务查看输出、Ctrl+D 清理、输入框置灰逻辑、移除 Ctrl+H 历史弹窗。

### 改动清单

#### 1. `internal/tui/app.go`

**a. 任务切换（Enter 在任务栏列表中）**

在任务栏的 `List.SetSelectedFunc` 中：

```go
list.SetSelectedFunc(func(index int, ...) {
    if index < len(barState.tasks) {
        barState.activeIdx = index
        task := barState.tasks[index]
        
        // 清除终端输出，显示该任务的输出
        outputView.Clear()
        _, _ = tview.ANSIWriter(outputView).Write([]byte(task.Output))
        
        // 根据任务状态控制输入框
        if task.Status == StatusRunning {
            inputField.SetDisabled(true)   // 执行中，置灰
        } else {
            inputField.SetDisabled(true)   // 已完成/失败，查看模式也置灰
        }
    }
})
```

任务栏列表失去焦点时：

```go
list.SetBlurFunc(func() {
    // 焦点离开任务栏 → 不做切换，保持当前展示的任务
})
```

**b. 输入框状态规则**

```go
// 在 Enter 执行后：
inputField.SetDisabled(false)
barState.focus = false

// 在切换到旧任务后：
inputField.SetDisabled(true)

// 在 Ctrl+B 回到输入框时：
// 如果当前展示的是最新任务且没有在运行
if barState.activeIdx == len(barState.tasks)-1 {
    task := barState.tasks[barState.activeIdx]
    if task.Status != StatusRunning {
        inputField.SetDisabled(false)
    }
}
```

**c. 新命令 Enter 的"自动回到最新任务"**

当用户在输入框输入新命令并按 Enter 时：
- 创建新任务
- `activeIdx` 指向新任务（最后一个）
- 终端输出展示新任务的输出
- 输入框保持可编辑（如果上一个任务已完成）

**d. Ctrl+L 融合任务清理**

- 移除了原有的 `Ctrl+D` 清理任务功能。
- 将清理任务与 `Ctrl+L`（清空输出）合并。当用户按下 `Ctrl+L` 且没有任务在运行时，将清空输出，并同时清理任务栏中所有已完成/失败的任务，隐藏任务栏。
- 按下 `Ctrl+U` 时，不仅恢复终端输出，也会恢复被清理的任务栏。

**e. 移除 Ctrl+H 历史弹窗**

- 删除 `showHistoryModal` 方法
- 删除 `inputField.SetInputCapture` 中 `KeyCtrlH` 的分支
- 更新 inputField 的 border title，去掉 `Ctrl+H:历史`

**f. 退出工具页面（Esc）时的行为**

```go
if event.Key() == tcell.KeyEscape {
    // 不清除任务，任务栏状态保留在内存中
    // 下次进入时，如果还有任务，任务栏自动恢复显示
    a.Pages.SwitchToPage("main")
    a.TviewApp.SetFocus(a.Tree)
}
```

#### 2. Tab 焦点轮转完整顺序

Phase 2 最终焦点轮转：

```
使用说明 → 终端输出 → 任务栏列表 → 命令行输入 → (回到使用说明)
```

通过 `focusables` 数组控制，任务栏 `visible` 为 false 时跳过任务栏。

#### 3. 测试验证 (Phase 2)

- 执行 3 个任务 → 任务栏显示 3 条
- ↑/↓ 浏览任务列表 → 按 Enter 切换 → 终端输出切换到对应任务的输出
- 切换到已完成的任务 → 输入框置灰，不能编辑
- Ctrl+B 回到输入框 → 输入框恢复（如果展示的是最新任务且不在运行）
- 输入新命令 Enter → 自动切回最新任务
- Ctrl+D → 已完成/失败的任务被清理
- 全部清理完 → 任务栏消失
- Ctrl+H 不再有任何响应

---

## 文件改动总结

| 文件 | Phase 1 | Phase 2 |
|------|---------|---------|
| `internal/tui/app.go` | 新增 Task/TaskStatus/outputTracker/TaskBarState/App字段，改 ShowTerminal/ShowPythonTerminal 布局，加 Ctrl+B 和 Tab 逻辑，改执行逻辑加 outputTracker | 改任务切换逻辑、加 Ctrl+D 清理、加输入框置灰逻辑、移除 Ctrl+H 历史弹窗 |
| `pkg/framework/framework.go` | 无改动 | 无改动 |
| `internal/storage/storage.go` | 无改动 | 无改动 |

---

## 开发建议

1. Phase 1 完成后即可编译运行测试，确认任务栏出现/隐藏/任务创建正常工作
2. Phase 2 在 Phase 1 基础上做，每个交互改动后都编译验证一次
3. 用 `go build -o my_tools.exe .` 编译，不要用 `go run`（因为 embed 文件在 go run 下可能路径不对）
