# 🚀 白犀牛工具箱：未来开发计划与规范

本文档旨在为后续的开发、维护和新工具接入提供标准规范，并记录未来可能的优化方向与聪明点子。

---

## 🛠️ 如何开发和接入新工具？

得益于全新的 `pkg/framework` 架构，接入新工具极其简单，**完全不需要修改任何 TUI 核心代码**。

### 接入规范三步走：

1. **创建目录和文件**：在 `tools/` 目录下为你的新工具创建一个独立的文件夹，例如 `tools/my_awesome_tool/tool.go`。
2. **实现接口**：定义一个结构体，并实现 `framework.Tool` 接口：
   ```go
   package my_awesome_tool

   import (
       "io"
       "my_tools/pkg/framework"
   )

   type AwesomeTool struct{}

   func (t *AwesomeTool) ID() string { return "awesome_id" }
   func (t *AwesomeTool) Name() string { return "我的牛逼工具" }
   func (t *AwesomeTool) Category() string { return "网络工具" } // 随意定义类别，TUI会自动建文件夹

   func (t *AwesomeTool) Execute(ctx framework.AppContext) {
       usage := "这是我的工具说明...\n支持参数: -name <名称>"
       
       // 调用 ShowTerminal 唤起终端执行面板
       ctx.ShowTerminal(t.Name(), usage, func(args string, out io.Writer) error {
           // args 是用户在输入框敲的参数
           // 使用 out.Write() 来向界面输出日志
           fmt.Fprintf(out, "你输入了参数: %s\n", args)
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

### 1. 终端面板的高级增强 (Terminal Enhancements)
- **命令历史记录上下翻页**：就像真实的 Linux 终端一样，在下方的输入框中按 `↑` 或 `↓` 键，可以翻出该工具更早之前的运行参数历史，而不仅仅是最后一次。
- **输出面板的 ANSI 颜色解析**：目前 `out io.Writer` 接收的是普通文本。如果未来接入的第三方程序自带 ANSI 颜色输出（如 `\033[31m`），可以考虑在 `app.go` 中解析这些逃逸字符，让输出面板像真终端一样五颜六色。

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