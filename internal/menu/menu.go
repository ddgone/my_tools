package menu

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// MenuItem 菜单项
type MenuItem struct {
	Title       string     // 显示标题
	Description string     // 描述
	Action      MenuAction // 动作
	SubItems    []MenuItem // 子菜单
}

// MenuAction 菜单动作类型
type MenuAction struct {
	Type    ActionType
	Command string      // 工具命令
	Func    func() error // 自定义函数
}

// ActionType 动作类型
type ActionType int

const (
	ActionSubmenu ActionType = iota // 进入子菜单
	ActionTool                      // 执行工具
	ActionCustom                    // 自定义动作
	ActionExit                      // 退出
)

// Menu 菜单
type Menu struct {
	Title string
	Items []MenuItem
}

// Show 显示菜单
func (m *Menu) Show() error {
	scanner := bufio.NewScanner(os.Stdin)
	
	for {
		// 清屏
		fmt.Print("\033[H\033[2J")
		
		// 显示标题
		fmt.Println(strings.Repeat("=", 60))
		fmt.Printf("  %s\n", m.Title)
		fmt.Println(strings.Repeat("=", 60))
		fmt.Println()
		
		// 显示菜单项
		for i, item := range m.Items {
			fmt.Printf("  %d. %-30s %s\n", i+1, item.Title, item.Description)
		}
		
		fmt.Println()
		fmt.Println(strings.Repeat("=", 60))
		fmt.Print("请选择 (输入数字，0退出): ")
		
		// 读取输入
		if !scanner.Scan() {
			break
		}
		
		input := strings.TrimSpace(scanner.Text())
		
		// 处理退出
		if input == "0" {
			fmt.Println("\n再见！👋")
			break
		}
		
		// 解析选择
		choice, err := strconv.Atoi(input)
		if err != nil || choice < 1 || choice > len(m.Items) {
			fmt.Println("\n❌ 无效选择，请重试")
			waitForEnter()
			continue
		}
		
		// 执行动作
		selected := m.Items[choice-1]
		if err := m.executeItem(selected); err != nil {
			fmt.Printf("\n❌ 错误: %v\n", err)
			waitForEnter()
		}
	}
	
	return scanner.Err()
}

// executeItem 执行菜单项
func (m *Menu) executeItem(item MenuItem) error {
	switch item.Action.Type {
	case ActionSubmenu:
		// 显示子菜单
		submenu := &Menu{
			Title: item.Title,
			Items: item.SubItems,
		}
		return submenu.Show()
		
	case ActionTool:
		// 执行工具
		fmt.Printf("\n🚀 启动工具: %s\n", item.Title)
		fmt.Println(strings.Repeat("-", 60))
		
		// 这里调用实际的工具执行逻辑
		if item.Action.Func != nil {
			return item.Action.Func()
		}
		
	case ActionCustom:
		// 自定义动作
		if item.Action.Func != nil {
			return item.Action.Func()
		}
		
	case ActionExit:
		os.Exit(0)
	}
	
	return nil
}

// waitForEnter 等待用户按回车
func waitForEnter() {
	fmt.Print("\n按回车键继续...")
	bufio.NewReader(os.Stdin).ReadBytes('\n')
}

// NewItem 创建菜单项（子菜单）
func NewItem(title, description string, subItems []MenuItem) MenuItem {
	return MenuItem{
		Title:       title,
		Description: description,
		Action: MenuAction{
			Type: ActionSubmenu,
		},
		SubItems: subItems,
	}
}

// NewToolItem 创建工具菜单项
func NewToolItem(title, description, command string, execute func() error) MenuItem {
	return MenuItem{
		Title:       title,
		Description: description,
		Action: MenuAction{
			Type:    ActionTool,
			Command: command,
			Func:    execute,
		},
	}
}

// NewCustomItem 创建自定义菜单项
func NewCustomItem(title, description string, action func() error) MenuItem {
	return MenuItem{
		Title:       title,
		Description: description,
		Action: MenuAction{
			Type: ActionCustom,
			Func: action,
		},
	}
}
