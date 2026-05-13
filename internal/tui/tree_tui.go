package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"my_tools/internal/storage"
)

// 样式定义
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1).
			MarginBottom(1)

	cursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).
			Bold(true)

	directoryStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFB86C")).
			Bold(true)

	toolStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#50FA7B"))

	selectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#7D56F4")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true)

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6272A4"))

	recentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF79C6"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6272A4")).
			MarginTop(1)
)

// TreeNode 树节点
type TreeNode struct {
	ID          string      // 唯一标识（路径）
	Title       string      // 显示标题
	Description string      // 描述
	Type        NodeType    // 节点类型
	Children    []*TreeNode // 子节点
	Expanded    bool        // 是否展开
	IsRecent    bool        // 是否是最近使用
	LastParams  map[string]string // 上次参数
	Action      func() error // 执行动作
}

// NodeType 节点类型
type NodeType int

const (
	TypeDirectory NodeType = iota
	TypeTool
	TypeRecent
)

// Model TUI 模型
type Model struct {
	tree        []*TreeNode
	cursor      int
	visibleNodes []*TreeNode // 可见的节点列表
	storage     *storage.Storage
	quitting    bool
	message     string // 状态消息
}

// NewModel 创建新模型
func NewModel(tree []*TreeNode, store *storage.Storage) Model {
	m := Model{
		tree:     tree,
		storage:  store,
		cursor:   0,
		message:  "",
	}
	
	// 加载展开状态
	m.loadExpandState()
	
	// 构建可见节点列表
	m.updateVisibleNodes()
	
	return m
}

// loadExpandState 加载展开状态
func (m *Model) loadExpandState() {
	var loadNode func(node *TreeNode)
	loadNode = func(node *TreeNode) {
		if node.Type == TypeDirectory {
			node.Expanded = m.storage.IsExpanded(node.ID)
		}
		for _, child := range node.Children {
			loadNode(child)
		}
	}
	
	for _, node := range m.tree {
		loadNode(node)
	}
}

// updateVisibleNodes 更新可见节点列表
func (m *Model) updateVisibleNodes() {
	m.visibleNodes = nil
	
	var collect func(nodes []*TreeNode)
	collect = func(nodes []*TreeNode) {
		for _, node := range nodes {
			m.visibleNodes = append(m.visibleNodes, node)
			if node.Type == TypeDirectory && node.Expanded {
				collect(node.Children)
			}
		}
	}
	
	collect(m.tree)
}

// Init 初始化
func (m Model) Init() tea.Cmd {
	return nil
}

// Update 更新状态
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
			
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			
		case "down", "j":
			if m.cursor < len(m.visibleNodes)-1 {
				m.cursor++
			}
			
		case "right", "l", "enter":
			if m.cursor >= 0 && m.cursor < len(m.visibleNodes) {
				node := m.visibleNodes[m.cursor]
				if node.Type == TypeDirectory {
					// 展开目录
					node.Expanded = true
					m.storage.Expand(node.ID)
					m.updateVisibleNodes()
					m.message = ""
				} else if node.Action != nil {
					// 执行工具
					return m.executeTool(node)
				}
			}
			
		case "left", "h":
			// 收起当前目录或返回父级
			if m.cursor >= 0 && m.cursor < len(m.visibleNodes) {
				node := m.visibleNodes[m.cursor]
				if node.Type == TypeDirectory && node.Expanded {
					// 收起当前目录
					node.Expanded = false
					m.storage.Collapse(node.ID)
					m.updateVisibleNodes()
					m.message = ""
				} else {
					// 尝试找到并收起父级目录
					m.collapseParent()
				}
			}
		}
		
	case tea.WindowSizeMsg:
		// 处理窗口大小变化
	}
	
	return m, nil
}

// collapseParent 收起父级目录
func (m *Model) collapseParent() {
	// 简化实现：收起所有展开的目录
	for _, node := range m.visibleNodes {
		if node.Type == TypeDirectory && node.Expanded {
			node.Expanded = false
			m.storage.Collapse(node.ID)
		}
	}
	m.updateVisibleNodes()
	if len(m.visibleNodes) > 0 {
		m.cursor = 0
	}
}

// executeTool 执行工具
func (m Model) executeTool(node *TreeNode) (tea.Model, tea.Cmd) {
	return m, tea.Batch(
		tea.ClearScreen,
		func() tea.Msg {
			fmt.Println(strings.Repeat("=", 60))
			fmt.Printf("🚀 启动工具: %s\n", node.Title)
			fmt.Println(strings.Repeat("=", 60))
			
			// 显示上次参数
			if len(node.LastParams) > 0 {
				fmt.Println("\n📝 上次使用的参数:")
				for k, v := range node.LastParams {
					fmt.Printf("   %s: %s\n", k, v)
				}
				fmt.Println()
			}
			
			// 执行工具
			err := node.Action()
			
			// 保存使用记录
			if err == nil {
				m.storage.AddRecentTool(node.Title, node.ID, node.LastParams)
				m.storage.Save()
			}
			
			fmt.Println()
			fmt.Println(strings.Repeat("=", 60))
			fmt.Print("按回车键返回...")
			
			// 等待回车
			buf := make([]byte, 1)
			os.Stdin.Read(buf)
			
			return toolExecutedMsg{err: err}
		},
	)
}

type toolExecutedMsg struct {
	err error
}

// View 渲染视图
func (m Model) View() string {
	if m.quitting {
		return "再见！👋\n"
	}
	
	var b strings.Builder
	
	// 标题
	b.WriteString(titleStyle.Render("🛠️  My Tools - 个人工具箱"))
	b.WriteString("\n\n")
	
	// 显示状态消息
	if m.message != "" {
		b.WriteString(dimStyle.Render(m.message))
		b.WriteString("\n\n")
	}
	
	// 渲染树形结构
	for i, node := range m.visibleNodes {
		cursor := "  "
		if i == m.cursor {
			cursor = cursorStyle.Render("▶ ")
		}
		
		indent := m.getIndent(node)
		
		var icon string
		var style lipgloss.Style
		
		switch node.Type {
		case TypeDirectory:
			if node.Expanded {
				icon = "📂 "
			} else {
				icon = "📁 "
			}
			style = directoryStyle
		case TypeRecent:
			icon = "⏰ "
			style = recentStyle
		default:
			icon = "🔧 "
			style = toolStyle
		}
		
		// 选中样式
		if i == m.cursor {
			style = selectedStyle
		}
		
		line := fmt.Sprintf("%s%s%s %s", cursor, indent, icon, node.Title)
		if node.Description != "" && i != m.cursor {
			line += dimStyle.Render(" - "+node.Description)
		}
		
		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}
	
	// 帮助信息
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/k 上  ↓/j 下  →/l/Enter 展开/执行  ←/h 收起  q 退出"))
	
	return b.String()
}

// getIndent 获取缩进
func (m *Model) getIndent(node *TreeNode) string {
	depth := m.getNodeDepth(node)
	return strings.Repeat("  ", depth)
}

// getNodeDepth 获取节点深度
func (m *Model) getNodeDepth(target *TreeNode) int {
	var find func(nodes []*TreeNode, depth int) int
	find = func(nodes []*TreeNode, depth int) int {
		for _, node := range nodes {
			if node == target {
				return depth
			}
			if d := find(node.Children, depth+1); d != -1 {
				return d
			}
		}
		return -1
	}
	
	return find(m.tree, 0)
}
