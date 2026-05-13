# My Tools

个人工作工具合集项目，基于增强型 TUI 的命令行工具箱。

## ✨ 特性

- ✅ **增强型 TUI 界面**：键盘导航，树形展开，视觉区分
- ✅ **最近使用记录**：快速访问常用工具
- ✅ **参数记忆**：保存并显示上次输入参数
- ✅ **交互式菜单**：友好的 TUI 界面，无需记忆命令
- ✅ **分类管理**：支持多级分类，清晰组织工具
- ✅ **即开即用**：运行可执行文件即可使用
- ✅ **跨平台**：纯 Go 实现，支持 Windows/Linux/Mac
- ✅ **易于扩展**：插件化架构，添加新工具只需几行代码

## 快速开始

### 编译

```bash
go build -o my_tools.exe    # Windows
go build -o my_tools        # Linux/Mac
```

### 使用

直接运行可执行文件，进入增强型 TUI 界面：

```bash
./my_tools.exe
```

然后通过键盘导航：

```
┌─────────────────────────────────────────────────────────┐
│  🛠️  My Tools - 个人工具箱                              │
└─────────────────────────────────────────────────────────┘

▶ 🏠 主页 - 工具箱主目录
    📁 ⏰ 最近使用 - 最近使用的工具 (0)
    📁 📝 文本处理 - 文本转换、格式化等工具
      🔧 大小写转换 - 转换文本为大写或小写
      🔧 文本反转 - 反转字符串内容
      🔧 去除空格 - 去除首尾空格
    🔧 ℹ️  关于 - 查看工具箱信息

↑/k 上  ↓/j 下  →/l/Enter 展开/执行  ←/h 收起  q 退出
```

详细用法请查看 [docs/ENHANCED_TUI.md](docs/ENHANCED_TUI.md)

## 项目结构

```
my_tools/
├── main.go                    # 主程序入口，定义菜单结构
├── internal/
│   └── menu/                  # 交互式菜单系统
│       └── menu.go            # 菜单核心逻辑
├── utils/                     # 工具实现（可选）
├── config/                    # 配置文件
├── scripts/                   # 脚本文件
└── docs/                      # 文档
```

## 已包含的工具

### 📝 文本处理
- **大小写转换** - 将文本转换为大写或小写
- **文本反转** - 反转字符串内容
- **去除空格** - 去除首尾空格

### ℹ️ 系统
- **关于** - 查看工具箱信息

## 添加新工具

### 方法 1：在现有分类下添加工具

编辑 `main.go` 中的 `createMainMenu()` 函数，在对应分类下添加：

```go
menu.NewToolItem(
    "统计字数",           // 标题
    "统计文本字数",       // 描述
    "word_count",        // 命令标识
    func() error {      // 执行函数
        return runWordCountTool()
    },
)
```

然后实现工具函数：

```go
func runWordCountTool() error {
    scanner := bufio.NewScanner(os.Stdin)
    
    fmt.Print("\n请输入文本: ")
    if !scanner.Scan() {
        return fmt.Errorf("读取输入失败")
    }
    text := scanner.Text()
    
    count := len(strings.Fields(text))
    fmt.Printf("\n字数: %d\n", count)
    return nil
}
```

### 方法 2：创建新的分类

```go
menu.NewItem(
    "🔧 系统工具",          // 分类标题
    "系统相关工具",         // 分类描述
    []menu.MenuItem{       // 子菜单项
        menu.NewToolItem(
            "查看系统信息",
            "显示系统详细信息",
            "system_info",
            func() error {
                return runSystemInfoTool()
            },
        ),
    },
)
```

详见 [docs/DEMO.md](docs/DEMO.md) 的完整示例

## 技术栈

- Go 1.x
- 标准库（flag, context, io 等）
- 无第三方依赖

## 许可证

MIT
