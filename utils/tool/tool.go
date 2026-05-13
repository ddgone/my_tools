package tool

import (
	"context"
)

// Tool 定义所有工具必须实现的接口
type Tool interface {
	// Name 返回工具名称（唯一标识）
	Name() string
	
	// Description 返回工具描述
	Description() string
	
	// RegisterFlags 注册命令行参数
	RegisterFlags()
	
	// Execute 执行工具逻辑
	Execute(ctx context.Context, args []string) error
}

// BaseTool 提供基础工具实现，可选嵌入
type BaseTool struct {
	name        string
	description string
}

func NewBaseTool(name, description string) *BaseTool {
	return &BaseTool{
		name:        name,
		description: description,
	}
}

func (b *BaseTool) Name() string {
	return b.name
}

func (b *BaseTool) Description() string {
	return b.description
}

func (b *BaseTool) RegisterFlags() {
	// 默认空实现，子类可覆盖
}
