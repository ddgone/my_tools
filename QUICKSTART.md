# 快速开始

## 编译项目

```bash
go build -o my_tools.exe    # Windows
go build -o my_tools        # Linux/Mac
```

## 基本使用

### 1. 查看所有工具

```bash
./my_tools
# 或
./my_tools list
```

### 2. 查看工具帮助

```bash
./my_tools text -h
```

### 3. 使用文本处理工具

```bash
# 转换为大写
echo "hello" | ./my_tools text -upper

# 转换为小写
echo "HELLO" | ./my_tools text -lower

# 反转文本
echo "abcdefg" | ./my_tools text -reverse

# 去除空格并转大写
echo "  hello world  " | ./my_tools text -trim -upper

# 从文件读取并输出到新文件
./my_tools text -lower -input input.txt -output output.txt
```

## 添加新工具

### 1. 复制模板

```bash
# PowerShell
Copy-Item -Recurse utils/template utils/my_new_tool

# Linux/Mac
cp -r utils/template utils/my_new_tool
```

### 2. 重命名文件

将 `utils/my_new_tool/tool_template.go` 重命名为 `my_tool.go`

### 3. 编辑代码

打开文件，替换占位符：
- `{{TOOL_NAME}}` → 你的工具名（如：`mytool`）
- `{{TOOL_NAME|title}}` → 工具名大写形式（如：`Mytool`）

实现你的业务逻辑。

### 4. 注册工具

在 `main.go` 中添加：

```go
import "my_tools/utils/my_new_tool"

func registerTools(reg *registry.Registry) {
    reg.Register(text_tool.NewTextTool())
    reg.Register(my_new_tool.NewMytoolTool())  // 添加这行
}
```

### 5. 编译测试

```bash
go build -o my_tools.exe
./my_tools list
./my_tools mytool -h
```

## 更多示例

查看 `utils/text_tool/text_tool.go` 了解完整的工具实现。

详细文档请参考 [docs/USAGE.md](docs/USAGE.md)
