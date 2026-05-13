# My Tools 使用指南

## 项目架构

这是一个基于插件化架构的命令行工具合集，采用纯 Go 实现，支持跨平台运行。

### 核心组件

- **Tool 接口** (`utils/tool/tool.go`): 定义所有工具必须实现的接口
- **Registry** (`utils/registry/registry.go`): 工具注册表，管理所有已注册的工具
- **主程序** (`main.go`): 工具箱外壳，负责解析命令和调用对应工具

### 目录结构

```
my_tools/
├── main.go                    # 主程序入口
├── utils/
│   ├── tool/                  # 工具接口定义
│   ├── registry/              # 工具注册表
│   ├── template/              # 新工具模板
│   ├── text_tool/             # 示例工具：文本处理
│   └── example_tool/          # 旧示例（可删除）
├── config/                    # 配置文件
├── scripts/                   # 脚本文件
└── docs/                      # 文档
```

## 使用方法

### 1. 编译程序

```bash
go build -o my_tools.exe    # Windows
go build -o my_tools        # Linux/Mac
```

### 2. 查看帮助

```bash
# 显示所有可用工具
./my_tools

# 列出工具列表
./my_tools list
# 或
./my_tools ls
```

### 3. 使用具体工具

以文本处理工具为例：

```bash
# 查看工具帮助
./my_tools text -h

# 从标准输入读取并转换为大写
echo "hello world" | ./my_tools text -upper

# 转换文件并输出到新文件
./my_tools text -lower -input input.txt -output output.txt

# 反转文本
echo "abcdefg" | ./my_tools text -reverse

# 去除首尾空格
echo "  hello  " | ./my_tools text -trim

# 组合使用多个选项
echo "  Hello World  " | ./my_tools text -trim -upper
```

## 添加新工具

### 步骤 1: 创建工具目录

复制模板目录：

```bash
cp -r utils/template utils/my_new_tool
```

### 步骤 2: 重命名文件

将 `tool_template.go` 重命名为有意义的名称，如 `my_tool.go`

### 步骤 3: 修改代码

编辑文件，替换占位符并实现逻辑：

```go
package my_new_tool

import (
    "context"
    "flag"
    "fmt"
    
    "my_tools/utils/tool"
)

type MyNewTool struct {
    *tool.BaseTool
    
    // 添加参数
    param1 string
    param2 bool
}

func NewMyNewTool() *MyNewTool {
    return &MyNewTool{
        BaseTool: tool.NewBaseTool("mytool", "我的新工具描述"),
    }
}

func (t *MyNewTool) RegisterFlags() {
    flag.StringVar(&t.param1, "param1", "", "参数1说明")
    flag.BoolVar(&t.param2, "param2", false, "参数2说明")
}

func (t *MyNewTool) Execute(ctx context.Context, args []string) error {
    fs := flag.NewFlagSet(t.Name(), flag.ContinueOnError)
    t.RegisterFlags()
    
    if err := fs.Parse(args); err != nil {
        return fmt.Errorf("解析参数失败: %w", err)
    }
    
    // 实现你的逻辑
    fmt.Println("工具执行中...")
    
    return nil
}
```

### 步骤 4: 注册工具

在 `main.go` 的 `registerTools()` 函数中添加：

```go
import "my_tools/utils/my_new_tool"

func registerTools(reg *registry.Registry) {
    reg.Register(text_tool.NewTextTool())
    reg.Register(my_new_tool.NewMyNewTool())  // 添加这一行
}
```

### 步骤 5: 编译测试

```bash
go build -o my_tools
./my_tools list      # 确认工具已注册
./my_tools mytool -h # 测试工具
```

## 开发规范

### 1. 工具命名

- 使用小写字母和下划线
- 名称应该简洁明了，反映工具功能
- 避免与现有工具重名

### 2. 参数设计

- 使用有意义的参数名
- 提供合理的默认值
- 添加清晰的帮助说明
- 验证参数的合法性

### 3. 错误处理

- 返回详细的错误信息
- 使用 `fmt.Errorf` 包装错误
- 在主程序中统一处理错误

### 4. 输入输出

- 支持标准输入/输出（默认）
- 支持文件输入/输出（可选）
- 保持输出的格式化和可读性

### 5. 跨平台兼容

- 只使用 Go 标准库
- 避免平台特定的系统调用
- 路径分隔符使用 `filepath` 包

## 示例工具详解

### Text Tool (文本处理工具)

位置: `utils/text_tool/`

功能:
- 大小写转换 (`-upper`, `-lower`)
- 文本反转 (`-reverse`)
- 去除空格 (`-trim`)
- 文件读写 (`-input`, `-output`)

用法示例:

```bash
# 管道输入
cat file.txt | ./my_tools text -upper

# 文件处理
./my_tools text -lower -input source.txt -output result.txt

# 组合操作
echo "  Hello World  " | ./my_tools text -trim -upper
```

## 常见问题

### Q: 如何调试工具？

A: 可以在工具中使用 `fmt.Println` 输出调试信息，或使用 Go 的调试工具。

### Q: 工具之间可以共享数据吗？

A: 当前设计是独立的，如需共享，可以通过文件或标准输入输出来传递数据。

### Q: 如何添加工具配置？

A: 可以在 `config/` 目录下添加配置文件，在工具的 `Execute` 方法中读取。

### Q: 支持子命令吗？

A: 当前不支持，每个工具是一个独立命令。如需复杂功能，建议拆分为多个工具。

## 最佳实践

1. **单一职责**: 每个工具只做一件事并做好
2. **组合使用**: 通过管道组合多个工具完成复杂任务
3. **文档完善**: 为每个工具编写 README 和使用示例
4. **测试充分**: 确保工具在各种输入下都能正常工作
5. **错误友好**: 提供清晰的错误提示和解决建议

## 扩展建议

未来可以考虑添加的功能：

- [ ] 工具配置文件支持
- [ ] 日志记录功能
- [ ] 进度条显示
- [ ] 交互式输入
- [ ] 工具执行统计
- [ ] 插件热加载（需要更复杂的架构）
