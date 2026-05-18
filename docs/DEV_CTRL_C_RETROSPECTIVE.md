# 📝 TUI 进程中断 (Ctrl+C) 修复复盘

## 现象描述
在运行耗时任务（如执行包含大量点的 GeoJSON 转换、或者死循环的 `ping` 命令）时，预期按下 `Ctrl+C` 能够安全地中断当前后台任务而不影响 TUI 主程序。
但在之前的某个版本中，按下 `Ctrl+C` 会导致主程序直接崩溃退出，并残留一个异常的空白终端界面。

## 问题根源分析

这个问题的根源分为两个层面：**TUI 框架底层机制**与**业务任务执行逻辑**。

### 1. `tview` 框架的硬编码退出机制
在 `tview` 的核心事件分发循环 (`application.go`) 中，对 `Ctrl+C` 有一个特殊的处理逻辑：
```go
// Ctrl-C closes the application.
if event == originalEvent && event.Key() == tcell.KeyCtrlC {
    a.Stop()
    break
}
```
这意味着：如果经过全局 `SetInputCapture` 钩子处理后返回的事件指针（`event`）与原始事件指针（`originalEvent`）**内存地址完全相同**，`tview` 就会无条件杀掉整个主程序。

在之前的代码中，我们曾在全局 `SetInputCapture` 中拦截 `Ctrl+C` 并直接返回 `nil` 以试图阻止 `app.Stop()`。虽然这确实阻止了主程序的崩溃，但**这也把事件彻底抛弃了**，导致下游（比如拥有焦点的输入框或输出框）根本收不到 `Ctrl+C` 信号。

### 2. 纯 Go 耗时循环无视 Context 信号
对于诸如 `ping`、Python 脚本等外部子进程，我们在代码里使用了 `exec.CommandContext` 并在另一个 `goroutine` 监听 `ctx.Done()` 以调用 `taskkill`，这能够完美杀掉它们。
但是，对于诸如 `geojson2shp`、`pos2gis` 这些纯粹用 Go 语言在当前进程中编写的庞大 `for` 循环任务，即使 `CancelFunc` 触发了，Go 循环本身由于没有加入 `<-ctx.Done()` 的检测，依然会固执地继续执行，直到几十万次循环跑完，导致给用户的感觉是“无法停止”。

## 最终修复方案

### 1. 拦截层：“事件克隆术”
为了既绕过 `tview` 的硬编码指针检查，又能让事件传递下去，我们在全局拦截处返回了一个**克隆的新事件**：
```go
a.TviewApp.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
    // 强制克隆事件指针，欺骗 tview 绕过 app.Stop() 检查
    if event.Key() == tcell.KeyCtrlC {
        return tcell.NewEventKey(event.Key(), event.Rune(), event.Modifiers())
    }
    // ...
})
```

### 2. 业务层：循环埋点
在所有原生的、包含长时间循环的 Go 工具代码中，注入对 `ctx.Done()` 的检测。为了不影响性能，通常每处理一定数量（如 1000 次）再检查一次：
```go
for i, f := range features {
    if i%1000 == 0 {
        select {
        case <-ctx.Done():
            return fmt.Errorf("任务已被取消")
        default:
        }
    }
    // 处理逻辑...
}
```

## Python 脚本适配器的表现
经过检查，Python 脚本由于是通过 `exec.CommandContext`（Linux/Mac）或额外的 `goroutine + taskkill`（Windows）在独立子进程中运行的，其在底层完美响应了 TUI 面板下发的 `CancelFunc`，因此 Python 脚本的 `Ctrl+C` 打断是天然生效且安全的，无需担心后续踩坑。

## 结论与规范更新
通过这两个修改，我们实现了 TUI 级别的完美优雅降级中断。
这一标准已同步更新至 `docs/DEVELOPMENT.md` 的《如何开发和接入新工具？》章节中，以后所有新加入的带有长循环的原生 Go 工具，都必须遵循此 `ctx.Done()` 检测规范。