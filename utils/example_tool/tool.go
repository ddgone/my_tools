package example_tool

// ExampleTool 示例工具
type ExampleTool struct {
	Name string
}

// NewExampleTool 创建示例工具
func NewExampleTool() *ExampleTool {
	return &ExampleTool{
		Name: "example",
	}
}

// Run 运行工具
func (t *ExampleTool) Run() error {
	// TODO: 实现工具逻辑
	return nil
}
