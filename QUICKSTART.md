# 快速开始

## 编译项目

```bash
go build -o my_tools.exe    # Windows
go build -o my_tools        # Linux/Mac
```

## 基本使用

### 1. 启动工具箱

直接运行可执行文件：

```bash
./my_tools.exe
```

你会看到交互式菜单界面。

### 2. 导航菜单

- **数字选择**：输入对应的数字选择功能
- **0 退出**：在任何层级输入 0 可以退出或返回上级
- **回车继续**：执行完工具后按回车返回菜单

### 3. 使用工具示例

以文本处理为例：

```
主菜单 -> 选择 1 (文本处理) -> 选择 1 (大小写转换)
-> 输入文本: hello world
-> 选择操作: 1 (大写)
-> 结果: HELLO WORLD
```

## 添加新工具

### 简单三步走

**步骤 1**: 在 `main.go` 的菜单定义中添加工具项

```go
menu.NewToolItem(
    "工具名称",
    "工具描述",
    "command_id",
    func() error {
        return runMyTool()  // 你的工具函数
    },
)
```

**步骤 2**: 实现工具函数

```go
func runMyTool() error {
    // 获取用户输入
    scanner := bufio.NewScanner(os.Stdin)
    
    fmt.Print("\n请输入: ")
    scanner.Scan()
    input := scanner.Text()
    
    // 处理逻辑
    result := process(input)
    
    // 显示结果
    fmt.Printf("\n结果: %s\n", result)
    return nil
}
```

**步骤 3**: 重新编译

```bash
go build -o my_tools.exe
```

就这么简单！

## 更多示例

查看 [docs/DEMO.md](docs/DEMO.md) 了解完整的使用演示和添加工具的详细教程。
