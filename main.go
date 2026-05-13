package main

import (
	"context"
	"fmt"
	"os"

	"my_tools/utils/registry"
	"my_tools/utils/text_tool"
)

func main() {
	// 创建工具注册表
	reg := registry.NewRegistry()
	
	// 注册所有工具
	registerTools(reg)
	
	// 如果没有参数，显示帮助信息
	if len(os.Args) < 2 {
		printHelp(reg)
		os.Exit(0)
	}
	
	// 获取工具名称
	toolName := os.Args[1]
	
	// 特殊命令：列出所有工具
	if toolName == "list" || toolName == "ls" {
		printToolList(reg)
		return
	}
	
	// 查找工具
	tool, exists := reg.Get(toolName)
	if !exists {
		fmt.Fprintf(os.Stderr, "错误: 未找到工具 '%s'\n\n", toolName)
		printToolList(reg)
		os.Exit(1)
	}
	
	// 执行工具
	ctx := context.Background()
	if err := tool.Execute(ctx, os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "执行失败: %v\n", err)
		os.Exit(1)
	}
}

// registerTools 注册所有工具
func registerTools(reg *registry.Registry) {
	// 注册文本处理工具
	reg.Register(text_tool.NewTextTool())
	
	// TODO: 在这里添加新工具
	// reg.Register(your_tool.NewYourTool())
}

// printHelp 打印帮助信息
func printHelp(reg *registry.Registry) {
	fmt.Println("My Tools - 个人工作工具合集")
	fmt.Println()
	fmt.Println("使用方法:")
	fmt.Println("  my_tools <tool_name> [options]")
	fmt.Println()
	fmt.Println("可用命令:")
	fmt.Println("  list, ls    列出所有可用工具")
	fmt.Println()
	fmt.Println("已注册的工具:")
	for _, name := range reg.Names() {
		if tool, ok := reg.Get(name); ok {
			fmt.Printf("  %-15s %s\n", name, tool.Description())
		}
	}
	fmt.Println()
	fmt.Println("使用 'my_tools <tool_name> -h' 查看具体工具的用法")
}

// printToolList 打印工具列表
func printToolList(reg *registry.Registry) {
	fmt.Println("可用工具列表:")
	fmt.Println()
	for _, name := range reg.Names() {
		if tool, ok := reg.Get(name); ok {
			fmt.Printf("  %-15s %s\n", name, tool.Description())
		}
	}
}
