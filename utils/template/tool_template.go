package {{TOOL_NAME}}

import (
	"context"
	"flag"
	"fmt"

	"my_tools/utils/tool"
)

// {{TOOL_NAME|title}}Tool 你的工具描述
type {{TOOL_NAME|title}}Tool struct {
	*tool.BaseTool
	
	// TODO: 添加配置参数
	// exampleParam string
}

// New{{TOOL_NAME|title}}Tool 创建工具实例
func New{{TOOL_NAME|title}}Tool() *{{TOOL_NAME|title}}Tool {
	return &{{TOOL_NAME|title}}Tool{
		BaseTool: tool.NewBaseTool("{{TOOL_NAME}}", "TODO: 工具描述"),
	}
}

// RegisterFlags 注册命令行参数（空实现）
func (t *{{TOOL_NAME|title}}Tool) RegisterFlags() {
	// 注意：实际使用时会在 Execute 中创建新的 FlagSet
}

// registerFlagsToSet 将参数注册到指定的 FlagSet
func (t *{{TOOL_NAME|title}}Tool) registerFlagsToSet(fs *flag.FlagSet) {
	// TODO: 注册参数
	// fs.StringVar(&t.exampleParam, "example", "", "示例参数")
}

// Execute 执行工具逻辑
func (t *{{TOOL_NAME|title}}Tool) Execute(ctx context.Context, args []string) error {
	// 解析参数
	fs := flag.NewFlagSet(t.Name(), flag.ExitOnError)
	t.registerFlagsToSet(fs)
	
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("解析参数失败: %w", err)
	}
	
	// TODO: 实现工具逻辑
	fmt.Println("TODO: 实现工具功能")
	
	return nil
}
