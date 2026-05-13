package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"my_tools/internal/storage"
	"my_tools/internal/tui"
)

func main() {
	// 加载用户数据
	store := storage.NewStorage()
	if err := store.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "警告: 加载用户数据失败: %v\n", err)
	}
	
	// 构建树形菜单
	tree := buildMenuTree(store)
	
	// 创建 TUI 模型
	model := tui.NewModel(tree, store)
	
	// 运行 TUI
	if _, err := tea.NewProgram(model).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}

// buildMenuTree 构建菜单树
func buildMenuTree(store *storage.Storage) []*tui.TreeNode {
	recentTools := store.GetRecentTools()
	
	var recentNodes []*tui.TreeNode
	for _, item := range recentTools {
		recentNodes = append(recentNodes, &tui.TreeNode{
			ID:          "recent_" + item.ToolPath,
			Title:       item.ToolName,
			Description: "上次: " + formatParams(item.LastParams),
			Type:        tui.TypeRecent,
			IsRecent:    true,
			LastParams:  item.LastParams,
			Action:      getToolAction(item.ToolPath),
		})
	}
	
	return []*tui.TreeNode{
		{
			ID:          "root",
			Title:       "🏠 主页",
			Description: "工具箱主目录",
			Type:        tui.TypeDirectory,
			Children: []*tui.TreeNode{
				{
					ID:          "recent",
					Title:       "⏰ 最近使用",
					Description: fmt.Sprintf("最近使用的工具 (%d)", len(recentNodes)),
					Type:        tui.TypeDirectory,
					Children:    recentNodes,
				},
				{
					ID:          "text",
					Title:       "📝 文本处理",
					Description: "文本转换、格式化等工具",
					Type:        tui.TypeDirectory,
					Children: []*tui.TreeNode{
						{
							ID:          "text_case",
							Title:       "大小写转换",
							Description: "转换文本为大写或小写",
							Type:        tui.TypeTool,
							Action:      runTextCaseTool,
						},
						{
							ID:          "text_reverse",
							Title:       "文本反转",
							Description: "反转字符串内容",
							Type:        tui.TypeTool,
							Action:      runTextReverseTool,
						},
						{
							ID:          "text_trim",
							Title:       "去除空格",
							Description: "去除首尾空格",
							Type:        tui.TypeTool,
							Action:      runTextTrimTool,
						},
					},
				},
				{
					ID:          "about",
					Title:       "ℹ️  关于",
					Description: "查看工具箱信息",
					Type:        tui.TypeTool,
					Action:      showAbout,
				},
			},
		},
	}
}

// getToolAction 根据工具路径获取执行函数
func getToolAction(path string) func() error {
	switch path {
	case "text_case":
		return runTextCaseTool
	case "text_reverse":
		return runTextReverseTool
	case "text_trim":
		return runTextTrimTool
	default:
		return func() error {
			return fmt.Errorf("未知工具: %s", path)
		}
	}
}

// formatParams 格式化参数显示
func formatParams(params map[string]string) string {
	if len(params) == 0 {
		return "无参数"
	}
	var parts []string
	for k, v := range params {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(parts, ", ")
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
	var operation string
	switch choice {
	case "1":
		result = strings.ToUpper(text)
		operation = "大写"
	case "2":
		result = strings.ToLower(text)
		operation = "小写"
	default:
		return fmt.Errorf("无效选择")
	}
	
	fmt.Printf("\n结果: %s\n", result)
	
	// TODO: 保存参数到存储
	_ = operation
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
