# 🚀 火蜥蜴工具箱：未来开发计划与规范

本文档旨在为后续的开发、维护和新工具接入提供标准规范，并记录未来可能的优化方向与聪明点子。

---

## 工具箱架构

本项目采用 Go 语言开发，基于 `tview` 提供全终端 UI 交互。采用**插件化架构**，所有功能均作为“工具 (Tool)”挂载。

### 核心特性
- **Python 脚本无缝集成**：使用 `embed` 和临时文件释放机制，完美封装任意 Python 脚本。
- **极致的终端 UX**：实现了状态保持（切走再回来数据不丢）、无限滚动追加、OSC 52 跨设备剪贴板支持、`Ctrl+C` 进程安全中断、以及焦点级快捷键隔离。
- **命令调色板与执行历史**：内置了类似于 VSCode 的 `Ctrl+P` 全局搜索树，以及 `Ctrl+H` 会话历史追踪，支持 `Ctrl+U` 撤销防误删。

### 新增支持：内嵌 Python 脚本
为了向下兼容历史 Python 脚本，工具箱内置了 `python_tools` 代理适配器：
1. **原理**：使用 `//go:embed` 技术，将 `.py` 文件打包在 `.exe` 内部。
2. **执行过程**：运行时释放到系统临时目录，调用宿主机的 `python` 执行，同时将标准输出重定向回 TUI 终端。
3. **如何添加**：将你的 Python 脚本放入 `tools/python_tools/scripts/` 目录，重新编译即可。它会被自动扫描并注册！

## 视觉设计规范 (UI/UX)
1. **统一的主题与色彩**：
   - 整体色调采用 Dracula (吸血鬼) 风格暗色主题（背景 `#282a36`）。
   - **语言生态色彩约定**：
     - Go 原生工具：亮青色 (`#68f9ff`)
     - Python 脚本工具：亮绿色 (`#68ff79`)
   - 目录与分类节点的颜色应使用与语言生态色同色系的浅色或相近色（如 `Salmon`, `Khaki`）以形成视觉层级。
2. **绝对坐标与防扭曲布局**：
   - 对于顶部 ASCII 艺术字（Logo、Font），**严禁使用文本流（TextView）自动换行**。
   - 必须使用类似 `BannerBox` 中的重写 `Draw(screen tcell.Screen)` 配合底层 `screen.SetContent`，确保字符绘制在绝对坐标上并自带边界裁剪，彻底解决窗口缩放导致的排版扭曲问题。
3. **全局与焦点快捷键 (Shortcut Isolation)**：
   - `F1`: 呼出全局快捷键速查面板（替代了原先各处冗余的 `(?:快捷键)` 提示，实现 UI 极简）。
   - `Ctrl+P` / `/`: 呼出全局搜索调色板。
   - `q`: 在首页且无输入框聚焦时退出应用。
   - `Ctrl+E`: 将当前获得焦点的模块放大至全屏，再次按还原。
   - `Ctrl+C`: 安全中断当前正在执行的底层子进程（不会杀掉 TUI 主程序）。
   - `Ctrl+S`: 自动导出输出框日志到 `my_tools_logs/` 目录，并复制到剪贴板。
   - `Ctrl+L`: 清空输出框。
   - `Ctrl+U`: 撤销刚刚的清空操作，恢复日志。
   - `Ctrl+H`: 在输入框呼出该工具的历史执行记录与输出预览。
   - `Ctrl+A`: 在输入框一键提取完整命令至剪贴板（Linux 无头环境下将自动调用 OSC 52 穿透 SSH 写入本地客户端剪贴板）。

### tview 框架疑难杂症与解法 (Traps & Hacks)

#### 1. 全局事件与 Ctrl+C 劫持
由于 `tview` 底层源码中硬编码了对 `Ctrl+C` 的默认处理（若 `SetInputCapture` 返回的事件指针等于原始事件指针，则直接调用 `app.Stop()` 退出整个程序），如果想要拦截 `Ctrl+C` 用作中断底层进程而不退出 TUI，必须在全局拦截处返回一个**克隆的新事件**：
```go
if event.Key() == tcell.KeyCtrlC {
    // 强制克隆事件指针，欺骗 tview 绕过 app.Stop() 检查
    return tcell.NewEventKey(event.Key(), event.Rune(), event.Modifiers())
}
```
这使得 `Ctrl+C` 事件得以顺利向下传递至当前获得焦点的具体面板中进行消费（如停止某个后台 Task）。

#### 2. 树组件重构与焦点光标丢失 (20ms Async Redraw)
在 TUI 交互中，如果因为某个操作（如按 `ESC` 退出工具页返回主页，此时需要刷新“最近使用”记录）导致整个 `tview.TreeView` 被替换或重建，随后立即调用 `SetCurrentNode` 和 `SetFocus`，会导致光标不显示或无法滚动到可视区域内。
这是因为在触发 `Draw()` 之前，树节点的屏幕坐标（Y轴）尚未计算完毕。
**解法**：必须使用一个微小的异步延迟配合 `QueueUpdateDraw` 形成“回马枪”，确保在第一次完整渲染（产生坐标）后再执行焦点设定：
```go
// 退出并重构树后...
go func() {
    time.Sleep(20 * time.Millisecond) // 等待 20ms，让第一次渲染（盲绘）完成，计算出坐标
    app.TviewApp.QueueUpdateDraw(func() {
        if found := app.findNodeByToolID(lastToolID); found != nil {
            app.Tree.SetCurrentNode(found)
        }
        app.TviewApp.SetFocus(app.Tree)
    })
}()
```

## 🛠️ 如何开发和接入新工具？

得益于全新的 `pkg/framework` 架构，接入新工具极其简单，**完全不需要修改任何 TUI 核心代码**。

### 接入规范三步走：

1. **创建目录和文件**：在 `tools/` 目录下为你的新工具创建一个独立的文件夹，例如 `tools/my_awesome_tool/tool.go`。
2. **实现接口**：定义一个结构体，并实现 `framework.Tool` 接口：
   ```go
   package my_awesome_tool

   import (
       "context"
       "fmt"
       "io"
       "my_tools/pkg/framework"
   )

   type AwesomeTool struct{}

   func (t *AwesomeTool) ID() string { return "awesome_id" }
   func (t *AwesomeTool) Name() string { return "我的牛逼工具" }
   func (t *AwesomeTool) Category() string { return "网络工具" } // 随意定义类别，TUI会自动建文件夹

   func (t *AwesomeTool) Execute(ctx framework.AppContext) {
       usage := "这是我的工具说明...\n支持参数: -name <名称>"
       
       // 调用 ShowTerminal 唤起终端执行面板。注意 run 函数签名包含了 context.Context
       ctx.ShowTerminal(t.Name(), usage, func(runCtx context.Context, args string, out io.Writer) error {
           // args 是用户在输入框敲的参数
           // 使用 out.Write() 来向界面输出日志
           fmt.Fprintf(out, "你输入了参数: %s\n", args)
           
           // 【关键规范】: 如果你的工具包含耗时循环，必须定期检查 runCtx.Done() 以响应 Ctrl+C 中断
           for i := 0; i < 100000; i++ {
               if i%1000 == 0 {
                   select {
                   case <-runCtx.Done():
                       return fmt.Errorf("任务已被取消")
                   default:
                   }
               }
               // ... 耗时操作 ...
           }
           
           return nil // 返回 err 会自动在界面显示红色的错误提示
       })
   }
   ```
3. **注册工具**：在同一个文件的 `init()` 函数中注册，并在 `main.go` 中引入：
   ```go
   // 在 tool.go 底部：
   func init() {
       framework.Register(&AwesomeTool{})
   }

   // 在 main.go 中：
   import _ "my_tools/tools/my_awesome_tool"
   ```

---

## 💡 聪明点子 & 未来优化方向

以下是为工具箱后续发展规划的一些值得尝试的改进点：

### 2. 交互式向导模式 (Wizard Mode)
- 对于参数极其复杂的工具（如 `utm_geojson`），纯手敲参数对新手依然有门槛。
- **思路**：在 `framework.AppContext` 中扩展一个 `ShowForm()` 方法。工具可以定义一组参数表单（如下拉框选 Zone、勾选是否 full-extract、文件选择器选路径），用户填完表单后，由框架自动拼接成命令字符串并执行。

### 3. 本地文件系统浏览器组件
- 很多工具需要选择输入路径（`-input`）。目前需要用户去资源管理器复制路径并粘贴进来。
- **思路**：可以开发一个基于 `tview` 的文件浏览器模态框。当用户在命令行输入区按下特定快捷键（如 `Ctrl+F`）时，弹出一个文件树让用户选择文件/目录，选中后自动将路径填入光标位置。

### 4. 任务后台运行与系统托盘
- 如果某个工具执行时间极长（比如处理 100GB 的点云文件），一直开着终端界面会影响使用其他工具。
- **思路**：引入后台任务管理机制。允许工具在后台 goroutine 中运行，并在 TUI 右上角显示一个 "[1 个后台任务正在运行]" 的指示器。用户可以随时切出去用别的工具，然后再切回来看进度。

### 5. 跨平台自动更新机制
- 作为单文件分发的程序，更新是个痛点。
- **思路**：引入一个简单的自我更新命令。比如在工具箱内内置一个 `Check Update` 工具，它会去 GitHub Releases 或内部服务器拉取最新的可执行文件，并利用 `os.Rename` 替换自身，实现平滑升级。