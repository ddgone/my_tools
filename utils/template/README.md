# 创建新工具指南

## 快速开始

1. 复制 `utils/template` 目录为你的工具目录（例如：`utils/my_new_tool`）
2. 重命名文件为合适的名称（例如：`my_tool.go`）
3. 修改代码中的占位符：
   - `{{TOOL_NAME}}` 替换为你的工具名称（小写，如：`mytool`）
   - `{{TOOL_NAME|title}}` 替换为工具名称的大写形式（如：`Mytool`）
   - 实现具体的业务逻辑

4. 在 `main.go` 的 `registerTools()` 函数中注册你的工具：
   ```go
   reg.Register(my_new_tool.NewMytoolTool())
   ```

## 工具接口说明

每个工具必须实现以下接口：

```go
type Tool interface {
    Name() string                        // 工具唯一标识
    Description() string                 // 工具描述
    RegisterFlags()                      // 注册命令行参数
    Execute(ctx context.Context, args []string) error  // 执行逻辑
}
```

## 示例

参考 `utils/text_tool` 的实现，它展示了：
- 参数配置（布尔值、字符串等）
- 文件输入/输出
- 标准输入/输出
- 错误处理

## 最佳实践

1. **单一职责**：每个工具只做一件事
2. **清晰的参数**：使用有意义的参数名和默认值
3. **友好的提示**：提供清晰的帮助信息和错误提示
4. **跨平台**：避免使用平台特定的API
5. **文档完善**：在工具目录下添加 README.md 说明用法
