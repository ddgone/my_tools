# 🚀 火蜥蜴工具箱：未来开发计划与规范

本文档旨在为后续的开发、维护和新工具接入提供标准规范，并记录未来可能的优化方向与聪明点子。

---

## 工具箱架构

本项目采用 Go 语言开发，基于 `tview` 提供全终端 UI 交互。采用**插件化架构**，所有功能均作为“工具 (Tool)”挂载。

### 新增支持：内嵌 Python 脚本
为了向下兼容历史 Python 脚本，工具箱内置了 `python_tools` 代理适配器：
1. **原理**：使用 `//go:embed` 技术，将 `.py` 文件打包在 `.exe` 内部。
2. **执行过程**：运行时释放到系统临时目录，调用宿主机的 `python` 执行，同时将标准输出重定向回 TUI 终端。
3. **如何添加**：将你的 Python 脚本放入 `tools/python_tools/scripts/` 目录，重新编译即可。它会被自动扫描并注册！

## 视觉设计规范 (UI/UX)
1. **统一的主题与色彩**：
   - 整体色调遵循火蜥蜴主题：顶部边框红色（Red），标题黄色（Yellow），特性说明橙色（Orange）。
   - **语言生态色彩约定**：
     - Go 原生工具：亮青色 (`#68f9ff`)
     - Python 脚本工具：亮绿色 (`#68ff79`)
   - 目录与分类节点的颜色应使用与语言生态色同色系的浅色或相近色（如 `Salmon`, `Khaki`）以形成视觉层级。
2. **绝对坐标与防扭曲布局**：
   - 对于顶部 ASCII 艺术字（Logo、Font），**严禁使用文本流（TextView）自动换行**。
   - 必须使用类似 `BannerBox` 中的重写 `Draw(screen tcell.Screen)` 配合底层 `screen.SetContent`，或者使用 `tview.Print` 方法，确保字符绘制在绝对坐标上并自带边界裁剪，彻底解决窗口缩放导致的排版扭曲问题。
3. **快捷交互与用户记忆**：
   - 已实现节点展开状态、参数输入历史以及“横幅折叠状态 (按 `b` 键)”的本地 JSON 持久化存储。新功能设计应尽可能利用 `a.Store` 保存用户的交互习惯。

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