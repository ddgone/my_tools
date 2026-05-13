package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"my_tools/internal/menu"
)

func main() {
	// 创建主菜单
	mainMenu := createMainMenu()
	
	// 显示菜单
	if err := mainMenu.Show(); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}

// createMainMenu 创建主菜单
func createMainMenu() *menu.Menu {
	return &menu.Menu{
		Title: "🛠️  My Tools - 个人工具箱",
		Items: []menu.MenuItem{
			menu.NewItem(
				"📝 文本处理",
				"文本转换、格式化等工具",
				[]menu.MenuItem{
					menu.NewToolItem(
						"大小写转换",
						"转换文本为大写或小写",
						"text_case",
						func() error {
							return runTextCaseTool()
						},
					),
					menu.NewToolItem(
						"文本反转",
						"反转字符串内容",
						"text_reverse",
						func() error {
							return runTextReverseTool()
						},
					),
					menu.NewToolItem(
						"去除空格",
						"去除首尾空格",
						"text_trim",
						func() error {
							return runTextTrimTool()
						},
					),
				},
			),
			menu.NewCustomItem(
				"ℹ️  关于",
				"查看工具箱信息",
				func() error {
					showAbout()
					return nil
				},
			),
		},
	}
}

// runTextCaseTool 运行大小写转换工具
func runTextCaseTool() error {
	scanner := bufio.NewScanner(os.Stdin)
	
	fmt.Print("\n请输入文本: ")
	if !scanner.Scan() {
		return fmt.Errorf("读取输入失败")
	}
	text := scanner.Text()
	
	fmt.Println("\n请选择操作:")
	fmt.Println("  1. 转换为大写")
	fmt.Println("  2. 转换为小写")
	fmt.Print("请选择 (1/2): ")
	
	if !scanner.Scan() {
		return fmt.Errorf("读取输入失败")
	}
	choice := strings.TrimSpace(scanner.Text())
	
	var result string
	switch choice {
	case "1":
		result = strings.ToUpper(text)
	case "2":
		result = strings.ToLower(text)
	default:
		return fmt.Errorf("无效选择")
	}
	
	fmt.Printf("\n结果: %s\n", result)
	return nil
}

// runTextReverseTool 运行文本反转工具
func runTextReverseTool() error {
	scanner := bufio.NewScanner(os.Stdin)
	
	fmt.Print("\n请输入文本: ")
	if !scanner.Scan() {
		return fmt.Errorf("读取输入失败")
	}
	text := scanner.Text()
	
	// 反转文本
	runes := []rune(text)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	result := string(runes)
	
	fmt.Printf("\n结果: %s\n", result)
	return nil
}

// runTextTrimTool 运行去除空格工具
func runTextTrimTool() error {
	scanner := bufio.NewScanner(os.Stdin)
	
	fmt.Print("\n请输入文本: ")
	if !scanner.Scan() {
		return fmt.Errorf("读取输入失败")
	}
	text := scanner.Text()
	
	result := strings.TrimSpace(text)
	
	fmt.Printf("\n结果: %s\n", result)
	return nil
}

// showAbout 显示关于信息
func showAbout() {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("  My Tools - 个人工作工具合集")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()
	fmt.Println("  版本: 1.0.0")
	fmt.Println("  描述: 一个简单易用的命令行工具箱")
	fmt.Println("  架构: 插件化设计，易于扩展")
	fmt.Println()
	fmt.Println("  技术栈:")
	fmt.Println("    - Go 语言")
	fmt.Println("    - 纯标准库实现")
	fmt.Println("    - 跨平台支持")
	fmt.Println()
	fmt.Println("  GitHub: (待添加)")
	fmt.Println(strings.Repeat("=", 60))
}
