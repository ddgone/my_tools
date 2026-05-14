package tui

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"my_tools/internal/storage"
	"my_tools/pkg/framework"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

//go:embed logo_200.txt
var salamanderArt string

//go:embed logo_new_40.txt
var logoArt string

//go:embed font_new.txt
var fontArt string

var (
	colorGo      = tcell.NewHexColor(0x68f9ff)
	colorPy      = tcell.NewHexColor(0x68ff79)
	colorBg      = tcell.NewHexColor(0x282a36) // Dracula 背景色
	colorBgDark  = tcell.NewHexColor(0x1e1f29) // 更深的背景色，用于输入框
	colorBgLight = tcell.NewHexColor(0x44475a) // 浅背景色，用于弹窗等
)

func init() {
	// 强制终端启用真彩色(True Color)支持
	// Linux 下许多终端模拟器实际上支持 24 位色，但没默认设置 COLORTERM 环境变量
	// 这会导致 tcell 自动降级到 256 色，从而使 Dracula 的特定 Hex 颜色就近匹配成了黑灰色
	if os.Getenv("COLORTERM") == "" {
		os.Setenv("COLORTERM", "truecolor")
	}

	tview.Styles.PrimitiveBackgroundColor = colorBg
	tview.Styles.ContrastBackgroundColor = colorBgLight
	tview.Styles.MoreContrastBackgroundColor = colorBgLight
}

type App struct {
	TviewApp *tview.Application
	Pages    *tview.Pages
	Tree     *SalamanderTreeView // 改用自定义带背景的树
	Store    *storage.Storage

	// context state
	currentTool framework.Tool
}

// 删掉之前的硬编码字符串

type SalamanderTreeView struct {
	*tview.TreeView
	bgArt []string
}

type BannerBox struct {
	*tview.Box
	logoLines []string
	fontLines []string
	descLines []string
}

func NewBannerBox(logo, font, desc string) *BannerBox {
	return &BannerBox{
		Box:       tview.NewBox(),
		logoLines: strings.Split(strings.TrimRight(logo, "\n"), "\n"),
		fontLines: strings.Split(strings.TrimRight(font, "\n"), "\n"),
		descLines: strings.Split(strings.TrimRight(desc, "\n"), "\n"),
	}
}

func (b *BannerBox) Draw(screen tcell.Screen) {
	// 临时将边框切换为双线（焦点样式），仅对 Banner 生效
	oldBorders := tview.Borders
	tview.Borders.Horizontal = tview.Borders.HorizontalFocus
	tview.Borders.Vertical = tview.Borders.VerticalFocus
	tview.Borders.TopLeft = tview.Borders.TopLeftFocus
	tview.Borders.TopRight = tview.Borders.TopRightFocus
	tview.Borders.BottomLeft = tview.Borders.BottomLeftFocus
	tview.Borders.BottomRight = tview.Borders.BottomRightFocus

	b.Box.DrawForSubclass(screen, b)

	// 恢复全局边框样式，不影响下面工具树的单线边框
	tview.Borders = oldBorders

	x, y, width, height := b.GetInnerRect()
	if width <= 0 || height <= 0 {
		return
	}

	logoStyle := tcell.StyleDefault.Foreground(tcell.ColorRed).Background(colorBg).Bold(true)
	fontStyle := tcell.StyleDefault.Foreground(tcell.ColorYellow).Background(colorBg).Bold(true)

	// Draw logo
	logoWidth := 0
	for r, line := range b.logoLines {
		runes := []rune(line)
		if len(runes) > logoWidth {
			logoWidth = len(runes)
		}
		screenY := y + r
		if screenY >= y+height {
			break
		}
		for c, ch := range runes { // use []rune to handle potential wide chars correctly though ascii art should be plain ascii
			screenX := x + c
			if screenX >= x+width {
				break
			}
			if ch != ' ' && ch != '\n' && ch != '\r' {
				screen.SetContent(screenX, screenY, ch, nil, logoStyle)
			}
		}
	}

	// Draw font next to logo
	fontStartX := x + logoWidth + 2
	fontStartY := y // 恢复上去
	for r, line := range b.fontLines {
		screenY := fontStartY + r
		if screenY >= y+height {
			break
		}
		for c, ch := range []rune(line) {
			screenX := fontStartX + c
			if screenX >= x+width {
				break
			}
			if ch != ' ' && ch != '\n' && ch != '\r' {
				screen.SetContent(screenX, screenY, ch, nil, fontStyle)
			}
		}
	}

	// Draw desc next to font
	descStartX := x + logoWidth + 2 + 50 // 预留50个字符宽度给font
	// 动态计算font的最大宽度，确保不重叠
	fontMaxWidth := 0
	for _, line := range b.fontLines {
		runes := []rune(line)
		if len(runes) > fontMaxWidth {
			fontMaxWidth = len(runes)
		}
	}
	descStartX = x + logoWidth + 2 + fontMaxWidth + 2 // font后面留2个空格，往左挪一点

	descStartY := y + 3 // 整体往下一行
	for r, line := range b.descLines {
		screenY := descStartY + r
		if screenY >= y+height {
			break
		}
		tview.Print(screen, line, descStartX, screenY, width-(descStartX-x), tview.AlignLeft, tcell.ColorDarkGray)
	}
}

func (s *SalamanderTreeView) Draw(screen tcell.Screen) {
	s.TreeView.Draw(screen)

	x, y, width, height := s.GetInnerRect()

	// --- 1. 渲染背景火蜥蜴 (bgArt) ---
	bgWidth := 0
	for _, line := range s.bgArt {
		if len(line) > bgWidth {
			bgWidth = len(line)
		}
	}

	// 往中间挪一点
	bgStartX := x + width - bgWidth - 30
	bgStartY := y + height - len(s.bgArt) - 5

	if bgStartX < x {
		bgStartX = x
	}
	if bgStartY < y {
		bgStartY = y
	}

	// 调整蜥蜴水印颜色，使其在 Dracula 背景下更协调
	bgStyle := tcell.StyleDefault.Foreground(tcell.NewHexColor(0x383a4a))

	for r, line := range s.bgArt {
		screenY := bgStartY + r
		if screenY >= y+height {
			break
		}
		for c, ch := range line {
			screenX := bgStartX + c
			if screenX >= x+width {
				break
			}
			if ch != ' ' && ch != '\n' && ch != '\r' {
				mainc, _, style, _ := screen.GetContent(screenX, screenY)
				if mainc == ' ' {
					_, bg, _ := style.Decompose()
					screen.SetContent(screenX, screenY, ch, nil, bgStyle.Background(bg))
				}
			}
		}
	}
}

// AppContextImpl 实现了 framework.AppContext 接口
type AppContextImpl struct {
	app  *App
	tool framework.Tool
}

func (c *AppContextImpl) ShowModal(title, message string) {
	c.app.ShowModal(title, message)
}

func (c *AppContextImpl) PromptInput(title, prompt, defaultValue string, callback func(string)) {
	c.app.PromptInput(title, prompt, defaultValue, callback)
}

func (c *AppContextImpl) PromptChoice(title, prompt string, options []string, callback func(string)) {
	c.app.PromptChoice(title, prompt, options, callback)
}

func (c *AppContextImpl) ShowTerminal(title string, usage string, run func(args string, out io.Writer) error) {
	c.app.ShowTerminal(c.tool, title, usage, run)
}

func (c *AppContextImpl) ShowPythonTerminal(title string, usage string, run func(env string, args string, out io.Writer) error) {
	c.app.ShowPythonTerminal(c.tool, title, usage, run)
}

func (c *AppContextImpl) GetLastParam(key string) string {
	recentTools := c.app.Store.GetRecentTools()
	for _, rt := range recentTools {
		if rt.ToolPath == c.tool.ID() {
			return rt.LastParams[key]
		}
	}
	return ""
}

func (c *AppContextImpl) RecordUsage(params map[string]string) {
	c.app.RecordToolUsage(c.tool.Name(), c.tool.ID(), params)
}

func NewApp(store *storage.Storage) *App {
	a := &App{
		TviewApp: tview.NewApplication(),
		Pages:    tview.NewPages(),
		Store:    store,
	}

	a.setupUI()

	// 全局按键
	a.TviewApp.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// q 退出
		if event.Key() == tcell.KeyRune && event.Rune() == 'q' {
			a.TviewApp.Stop()
			return nil
		}
		return event
	})

	return a
}

func (a *App) Run() error {
	return a.TviewApp.SetRoot(a.Pages, true).Run()
}

func (a *App) setupUI() {
	// --- 工具树面板 ---
	baseTree := tview.NewTreeView()
	baseTree.SetGraphics(true) // 恢复连线
	baseTree.SetTopLevel(1)    // 隐藏根节点，让三大板块贴边

	a.Tree = &SalamanderTreeView{
		TreeView: baseTree,
		bgArt:    strings.Split(strings.TrimRight(salamanderArt, "\n"), "\n"),
	}

	a.Tree.SetBorder(true).
		SetBorderColor(tcell.ColorDarkGray)

	a.refreshTree()

	a.Tree.SetSelectedFunc(func(node *tview.TreeNode) {
		ref := node.GetReference()
		if ref == nil {
			node.SetExpanded(!node.IsExpanded())
			if nodeID := getNodeID(node); nodeID != "" {
				a.Store.SetNodeState(nodeID, node.IsExpanded())
			}
			return
		}

		tool, ok := ref.(framework.Tool)
		if ok {
			a.executeTool(tool)
		} else {
			// Directory node
			node.SetExpanded(!node.IsExpanded())
			if nodeID := getNodeID(node); nodeID != "" {
				a.Store.SetNodeState(nodeID, node.IsExpanded())
			}
		}
	})

	descText := "[yellow::b]Fire Salamander Tools[-:-:-]\n" +
		"[white]打造最强跨平台全能工具箱[-]\n" +
		"[gray]---------------------------[-]\n" +
		"[orange]• 极速原生启动[-]\n" +
		"[orange]• 无缝兼容 Python 脚本[-]\n" +
		"[orange]• 一键全平台编译分发[-]"

	banner := NewBannerBox(logoArt, fontArt, descText)
	banner.SetBorder(true).
		SetTitle(" 🦎 火蜥蜴工具箱 [gray](←/→/↑/↓:导航, Enter:展开/执行, r:重置, b:折叠横幅, q:退出)[-] ").
		SetTitleColor(tcell.ColorRed).
		SetBorderColor(tcell.ColorDarkGray)

	bannerExpanded := a.Store.GetNodeState("BannerExpanded", true)

	mainLayout := tview.NewFlex().SetDirection(tview.FlexRow)

	updateBannerHeight := func() {
		if bannerExpanded {
			mainLayout.ResizeItem(banner, 13, 1)
		} else {
			mainLayout.ResizeItem(banner, 2, 1) // 2 刚好只显示带标题的上下边框，内部隐藏
		}
	}

	mainLayout.AddItem(banner, 13, 1, false).
		AddItem(a.Tree, 0, 1, true)

	updateBannerHeight()

	// 树的自定义键盘导航
	a.Tree.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		node := a.Tree.GetCurrentNode()
		if node == nil {
			return event
		}

		switch event.Key() {
		case tcell.KeyRight:
			// 如果是折叠的目录，则展开
			if len(node.GetChildren()) > 0 && !node.IsExpanded() {
				node.SetExpanded(true)
				if nodeID := getNodeID(node); nodeID != "" {
					a.Store.SetNodeState(nodeID, true)
				}
			} else if len(node.GetChildren()) > 0 && node.IsExpanded() {
				// 如果已展开，则移动到第一个子节点
				a.Tree.SetCurrentNode(node.GetChildren()[0])
			} else if ref := node.GetReference(); ref != nil {
				// 如果是工具节点，则执行
				if tool, ok := ref.(framework.Tool); ok {
					a.executeTool(tool)
				}
			}
			return nil
		case tcell.KeyLeft:
			// 回退到父节点，但不收回展开
			if parent := a.findParent(a.Tree.GetRoot(), node); parent != nil && parent != a.Tree.GetRoot() {
				a.Tree.SetCurrentNode(parent)
			}
			return nil
		case tcell.KeyRune:
			if event.Rune() == 'r' || event.Rune() == 'R' {
				a.Store.ClearNodeStates()
				a.refreshTree()
				return nil
			}
			if event.Rune() == 'b' || event.Rune() == 'B' {
				bannerExpanded = !bannerExpanded
				a.Store.SetNodeState("BannerExpanded", bannerExpanded)
				a.Store.Save()
				updateBannerHeight()
				return nil
			}
		}
		return event
	})

	a.Pages.AddAndSwitchToPage("main", mainLayout, true)
}

func (a *App) findParent(root, target *tview.TreeNode) *tview.TreeNode {
	if root == target {
		return nil
	}
	for _, child := range root.GetChildren() {
		if child == target {
			return root
		}
		if p := a.findParent(child, target); p != nil {
			return p
		}
	}
	return nil
}

func getNodeID(node *tview.TreeNode) string {
	// 从节点文本中提取唯一标识
	text := node.GetText()
	// 简单过滤掉图标和空格作为 ID
	return strings.TrimSpace(text)
}

func (a *App) refreshTree() {
	root := tview.NewTreeNode("🦎 火蜥蜴工具箱").SetColor(tcell.ColorWhite).SetSelectable(false)

	// --- 1. 最近使用 ---
	recentTools := a.Store.GetRecentTools()
	recentNode := tview.NewTreeNode(" ⭐ 最近使用 ").
		SetColor(tcell.ColorYellow).
		SetSelectable(true).
		SetExpanded(a.Store.GetNodeState("⭐ 最近使用", true))

	if len(recentTools) > 0 {
		count := 0
		for _, rt := range recentTools {
			if count >= 3 {
				break
			}
			var targetTool framework.Tool
			for _, t := range framework.Registry {
				if t.ID() == rt.ToolPath {
					targetTool = t
					break
				}
			}

			if targetTool != nil {
				paramsStr := ""
				if len(rt.LastParams) > 0 {
					var keys []string
					for k := range rt.LastParams {
						keys = append(keys, k)
					}
					sort.Strings(keys)
					var parts []string
					for _, k := range keys {
						parts = append(parts, fmt.Sprintf("%s:%s", k, rt.LastParams[k]))
					}
					paramsStr = " [" + strings.Join(parts, ", ") + "]"
				}

				// 判断工具类型给予不同的颜色
				toolColor := tcell.ColorWhite
				icon := "🔧"
				if strings.Contains(targetTool.Category(), "Python") {
					toolColor = colorPy
					icon = "📄"
				} else {
					toolColor = colorGo
				}

				toolNode := tview.NewTreeNode(fmt.Sprintf(" %s %s%s", icon, targetTool.Name(), paramsStr)).
					SetReference(targetTool).
					SetColor(toolColor).
					SetSelectable(true)
				recentNode.AddChild(toolNode)
				count++
			}
		}
	} else {
		emptyNode := tview.NewTreeNode(" (暂无) ").SetColor(tcell.ColorGray).SetSelectable(false)
		recentNode.AddChild(emptyNode)
	}
	root.AddChild(recentNode)

	// 添加一个空行节点作为视觉分隔
	root.AddChild(tview.NewTreeNode("").SetSelectable(false))

	// --- 2. 内置原生应用 ---
	nativeNode := tview.NewTreeNode(" 🚀 内置原生应用 ").
		SetColor(tcell.ColorSalmon).
		SetSelectable(true).
		SetExpanded(a.Store.GetNodeState("🚀 内置原生应用", true))

	// --- 3. 扩展兼容脚本 ---
	scriptNode := tview.NewTreeNode(" 📜 扩展兼容脚本 ").
		SetColor(tcell.ColorKhaki).
		SetSelectable(true).
		SetExpanded(a.Store.GetNodeState("📜 扩展兼容脚本", true))

	nativeCategories := make(map[string]*tview.TreeNode)
	scriptCategories := make(map[string]*tview.TreeNode)

	for _, t := range framework.Registry {
		catName := t.Category()

		var parentNode *tview.TreeNode
		var catMap map[string]*tview.TreeNode

		// 判断是否是脚本类工具
		if strings.Contains(catName, "脚本") {
			parentNode = scriptNode
			catMap = scriptCategories
		} else {
			parentNode = nativeNode
			catMap = nativeCategories
		}

		catNode, exists := catMap[catName]
		if !exists {
			icon := "📁"
			color := tcell.ColorLightSalmon
			if strings.Contains(catName, "Python") {
				icon = "🐍"
				color = tcell.ColorPaleGoldenrod
			}
			catNodeText := fmt.Sprintf(" %s %s", icon, catName)
			catNode = tview.NewTreeNode(catNodeText).
				SetColor(color).
				SetSelectable(true).
				SetExpanded(a.Store.GetNodeState(strings.TrimSpace(catNodeText), true)) // 次级目录默认展开
			catMap[catName] = catNode
			parentNode.AddChild(catNode)
		}

		icon := "🔧"
		color := colorGo // 原生应用工具使用自定义亮蓝色
		if strings.Contains(catName, "Python") {
			icon = "📄"
			color = colorPy // Python脚本工具使用自定义亮绿色
		}
		toolNode := tview.NewTreeNode(fmt.Sprintf(" %s %s", icon, t.Name())).
			SetReference(t).
			SetColor(color).
			SetSelectable(true)

		catNode.AddChild(toolNode)
	}

	root.AddChild(nativeNode)
	root.AddChild(tview.NewTreeNode("").SetSelectable(false)) // 空行分隔
	root.AddChild(scriptNode)

	a.Tree.SetRoot(root)
	a.Tree.SetCurrentNode(root)
}

func (a *App) executeTool(t framework.Tool) {
	ctx := &AppContextImpl{
		app:  a,
		tool: t,
	}
	t.Execute(ctx)
}

// --- UI 辅助组件 ---

func (a *App) ShowModal(title, message string) {
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"确定"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			a.Pages.RemovePage("modal")
			a.Pages.SwitchToPage("main")
			a.TviewApp.SetFocus(a.Tree)
		})
	modal.SetBorder(true).
		SetTitle(" " + title + " ").
		SetTitleColor(tcell.ColorWhite).
		SetBackgroundColor(colorBgLight)
	a.Pages.AddAndSwitchToPage("modal", modal, true)
	a.TviewApp.SetFocus(modal)
}

func (a *App) PromptInput(title, prompt, defaultValue string, callback func(string)) {
	form := tview.NewForm()
	inputField := tview.NewInputField().
		SetLabel(prompt).
		SetText(defaultValue).
		SetFieldWidth(40)

	// 美化表单样式
	form.SetFieldBackgroundColor(colorBgDark).
		SetFieldTextColor(tcell.ColorWhite).
		SetLabelColor(tcell.ColorOrange).
		SetButtonBackgroundColor(tcell.NewHexColor(0xff5555)).
		SetButtonTextColor(tcell.ColorWhite)

	form.AddFormItem(inputField)
	form.AddButton("确定", func() {
		text := inputField.GetText()
		a.Pages.RemovePage("input")
		a.Pages.SwitchToPage("main")
		a.TviewApp.SetFocus(a.Tree)
		callback(text)
	})
	form.AddButton("取消", func() {
		a.Pages.RemovePage("input")
		a.Pages.SwitchToPage("main")
		a.TviewApp.SetFocus(a.Tree)
	})
	form.SetBorder(true).
		SetTitle(" " + title + " ").
		SetTitleColor(tcell.ColorYellow).
		SetBorderColor(tcell.ColorDarkGray)

	a.Pages.AddAndSwitchToPage("input", form, true)
	a.TviewApp.SetFocus(form)
}

func (a *App) PromptChoice(title, prompt string, options []string, callback func(string)) {
	form := tview.NewForm()
	dropdown := tview.NewDropDown().
		SetLabel(prompt).
		SetOptions(options, nil)

	// 美化表单样式
	form.SetFieldBackgroundColor(colorBgDark).
		SetFieldTextColor(tcell.ColorWhite).
		SetLabelColor(tcell.ColorOrange).
		SetButtonBackgroundColor(tcell.NewHexColor(0xff5555)).
		SetButtonTextColor(tcell.ColorWhite)

	form.AddFormItem(dropdown)
	form.AddButton("确定", func() {
		_, text := dropdown.GetCurrentOption()
		a.Pages.RemovePage("choice")
		a.Pages.SwitchToPage("main")
		a.TviewApp.SetFocus(a.Tree)
		callback(text)
	})
	form.AddButton("取消", func() {
		a.Pages.RemovePage("choice")
		a.Pages.SwitchToPage("main")
		a.TviewApp.SetFocus(a.Tree)
	})
	form.SetBorder(true).
		SetTitle(" " + title + " ").
		SetTitleColor(tcell.ColorYellow).
		SetBorderColor(tcell.ColorDarkGray)

	a.Pages.AddAndSwitchToPage("choice", form, true)
	a.TviewApp.SetFocus(form)
}

func (a *App) RecordToolUsage(name, toolID string, params map[string]string) {
	a.Store.AddRecentTool(name, toolID, params)
	a.Store.Save()
	a.refreshTree()
}

func (a *App) ShowTerminal(tool framework.Tool, title string, usage string, run func(args string, out io.Writer) error) {
	flex := tview.NewFlex().SetDirection(tview.FlexRow)

	// 上半部分：使用说明
	usageView := tview.NewTextView().
		SetDynamicColors(true).
		SetText(usage).
		SetScrollable(true)
	usageView.SetBorder(true).
		SetTitle(" 📖 使用说明 ").
		SetTitleColor(colorGo).
		SetBorderColor(tcell.ColorDarkGray)

	// 中半部分：执行输出
	outputView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetChangedFunc(func() {
			a.TviewApp.Draw()
		})
	outputView.SetBorder(true).
		SetTitle(" 📺 终端输出 ").
		SetTitleColor(tcell.ColorYellow).
		SetBorderColor(tcell.ColorDarkGray)

	// 查找最近一次的参数
	initialCmd := ""
	recentTools := a.Store.GetRecentTools()
	for _, rt := range recentTools {
		if rt.ToolPath == tool.ID() {
			if cmd, ok := rt.LastParams["cmd"]; ok {
				initialCmd = cmd
			}
			break
		}
	}

	// 下半部分：输入区域 (多行文本框更适合较长的命令参数)
	inputField := tview.NewTextArea().
		SetLabel(" ❯ ").
		SetText(initialCmd, true)
	inputField.SetBackgroundColor(colorBgDark)
	inputField.SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorWhite))

	inputField.SetBorder(true).
		SetTitle(" ⌨️ 命令行输入 [gray](输入参数后按 Enter 执行，按 ESC 返回，按 Tab 切换焦点滚动)[-] ").
		SetTitleColor(tcell.ColorOrange).
		SetBorderColor(tcell.ColorDarkGray)

	flex.AddItem(usageView, 0, 1, false).
		AddItem(outputView, 0, 3, false).
		AddItem(inputField, 5, 1, true) // 给输入框多一点高度 (5行)

	// 焦点高亮效果
	usageView.SetFocusFunc(func() { usageView.SetBorderColor(colorGo) })
	usageView.SetBlurFunc(func() { usageView.SetBorderColor(tcell.ColorDarkGray) })
	outputView.SetFocusFunc(func() { outputView.SetBorderColor(tcell.ColorYellow) })
	outputView.SetBlurFunc(func() { outputView.SetBorderColor(tcell.ColorDarkGray) })
	inputField.SetFocusFunc(func() { inputField.SetBorderColor(tcell.ColorOrange) })
	inputField.SetBlurFunc(func() { inputField.SetBorderColor(tcell.ColorDarkGray) })

	focusables := []tview.Primitive{inputField, outputView, usageView}
	currentFocus := 0

	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			a.Pages.RemovePage("terminal")
			a.Pages.SwitchToPage("main")
			a.TviewApp.SetFocus(a.Tree)
			return nil
		}
		if event.Key() == tcell.KeyTab {
			currentFocus = (currentFocus + 1) % len(focusables)
			a.TviewApp.SetFocus(focusables[currentFocus])
			return nil
		}
		if event.Key() == tcell.KeyBacktab {
			currentFocus = (currentFocus - 1 + len(focusables)) % len(focusables)
			a.TviewApp.SetFocus(focusables[currentFocus])
			return nil
		}
		return event
	})

	inputField.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEnter && event.Modifiers() == tcell.ModNone {
			cmdText := strings.TrimSpace(inputField.GetText())
			outputView.Clear()

			// 记录使用
			a.RecordToolUsage(tool.Name(), tool.ID(), map[string]string{"cmd": cmdText})

			// 开始执行
			go func() {
				a.TviewApp.QueueUpdateDraw(func() {
					inputField.SetDisabled(true)
					outputView.Write([]byte(fmt.Sprintf("[yellow]执行命令: %s[-]\n", cmdText)))
				})

				err := run(cmdText, outputView)

				a.TviewApp.QueueUpdateDraw(func() {
					if err != nil {
						outputView.Write([]byte(fmt.Sprintf("\n[red]执行出错: %v[-]\n", err)))
					} else {
						outputView.Write([]byte("\n[green]执行完成[-]\n"))
					}
					inputField.SetDisabled(false)
					a.TviewApp.SetFocus(inputField)
				})
			}()
			return nil // 拦截按键，防止输入换行
		}
		// 允许 Shift+Enter 换行 (如果需要的话)
		if event.Key() == tcell.KeyEnter && event.Modifiers()&tcell.ModShift != 0 {
			return event
		}
		return event
	})

	a.Pages.AddAndSwitchToPage("terminal", flex, true)
	a.TviewApp.SetFocus(inputField)
}

func (a *App) ShowPythonTerminal(tool framework.Tool, title string, usage string, run func(env string, args string, out io.Writer) error) {
	flex := tview.NewFlex().SetDirection(tview.FlexRow)

	// 上半部分：使用说明
	usageView := tview.NewTextView().
		SetDynamicColors(true).
		SetText(usage).
		SetScrollable(true)
	usageView.SetBorder(true).
		SetTitle(" 📖 使用说明 (Python专属) ").
		SetTitleColor(colorPy).
		SetBorderColor(tcell.ColorDarkGray)

	// 中半部分：执行输出
	outputView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetChangedFunc(func() {
			a.TviewApp.Draw()
		})
	outputView.SetBorder(true).
		SetTitle(" 📺 终端输出 ").
		SetTitleColor(tcell.ColorYellow).
		SetBorderColor(tcell.ColorDarkGray)

	// 查找最近一次的参数
	initialCmd := ""
	initialEnv := "python"
	recentTools := a.Store.GetRecentTools()
	for _, rt := range recentTools {
		if rt.ToolPath == tool.ID() {
			if cmd, ok := rt.LastParams["cmd"]; ok {
				initialCmd = cmd
			}
			if env, ok := rt.LastParams["env"]; ok {
				initialEnv = env
			}
			break
		}
	}

	// Python 环境设置栏
	envForm := tview.NewForm().
		SetFieldBackgroundColor(colorBgDark).
		SetFieldTextColor(tcell.ColorWhite).
		SetLabelColor(colorPy).
		SetButtonBackgroundColor(tcell.ColorDarkGreen).
		SetButtonTextColor(tcell.ColorWhite)

	envForm.SetBorder(true).
		SetTitle(" ⚙️ Python 环境配置 [gray](Tab 切换焦点)[-] ").
		SetTitleColor(colorPy).
		SetBorderColor(tcell.ColorDarkGray)

	envInput := tview.NewInputField().
		SetLabel("解释器路径: ").
		SetText(initialEnv).
		SetFieldWidth(30)
	envForm.AddFormItem(envInput)

	// 安装依赖的按钮
	envForm.AddButton("📦 安装 pip 依赖", func() {
		envPath := strings.TrimSpace(envInput.GetText())
		if envPath == "" {
			envPath = "python"
		}

		a.PromptInput("安装 pip 依赖", "输入要安装的包名 (如 requests pandas):", "", func(pkgName string) {
			if pkgName == "" {
				return
			}
			outputView.Clear()
			outputView.Write([]byte(fmt.Sprintf("[yellow]正在使用 %s 安装依赖: %s[-]\n", envPath, pkgName)))

			go func() {
				// We pass a special args flag "!pip " to let the adapter know we want to run pip
				err := run(envPath, "!pip "+pkgName, outputView)
				a.TviewApp.QueueUpdateDraw(func() {
					if err != nil {
						outputView.Write([]byte(fmt.Sprintf("\n[red]安装出错: %v[-]\n", err)))
					} else {
						outputView.Write([]byte("\n[green]安装完成！[-]\n"))
					}
				})
			}()
		})
	})

	// 下半部分：输入区域
	inputField := tview.NewTextArea().
		SetLabel(" ❯ ").
		SetText(initialCmd, true)
	inputField.SetBackgroundColor(colorBgDark)
	inputField.SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorWhite))

	inputField.SetBorder(true).
		SetTitle(" ⌨️ 命令行输入 [gray](输入参数后按 Enter 执行，按 ESC 返回)[-] ").
		SetTitleColor(tcell.ColorOrange).
		SetBorderColor(tcell.ColorDarkGray)

	flex.AddItem(usageView, 0, 1, false).
		AddItem(outputView, 0, 3, false).
		AddItem(envForm, 5, 1, false).
		AddItem(inputField, 5, 1, true)

	// 焦点高亮效果
	usageView.SetFocusFunc(func() { usageView.SetBorderColor(colorPy) })
	usageView.SetBlurFunc(func() { usageView.SetBorderColor(tcell.ColorDarkGray) })
	outputView.SetFocusFunc(func() { outputView.SetBorderColor(tcell.ColorYellow) })
	outputView.SetBlurFunc(func() { outputView.SetBorderColor(tcell.ColorDarkGray) })
	envForm.SetFocusFunc(func() { envForm.SetBorderColor(colorPy) })
	envForm.SetBlurFunc(func() { envForm.SetBorderColor(tcell.ColorDarkGray) })
	inputField.SetFocusFunc(func() { inputField.SetBorderColor(tcell.ColorOrange) })
	inputField.SetBlurFunc(func() { inputField.SetBorderColor(tcell.ColorDarkGray) })

	focusables := []tview.Primitive{inputField, envForm, outputView, usageView}
	currentFocus := 0

	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			a.Pages.RemovePage("py_terminal")
			a.Pages.SwitchToPage("main")
			a.TviewApp.SetFocus(a.Tree)
			return nil
		}
		if event.Key() == tcell.KeyTab {
			currentFocus = (currentFocus + 1) % len(focusables)
			a.TviewApp.SetFocus(focusables[currentFocus])
			return nil
		}
		if event.Key() == tcell.KeyBacktab {
			currentFocus = (currentFocus - 1 + len(focusables)) % len(focusables)
			a.TviewApp.SetFocus(focusables[currentFocus])
			return nil
		}
		return event
	})

	inputField.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEnter && event.Modifiers() == tcell.ModNone {
			cmdText := strings.TrimSpace(inputField.GetText())
			envPath := strings.TrimSpace(envInput.GetText())
			if envPath == "" {
				envPath = "python"
			}

			outputView.Clear()

			// 记录使用
			a.RecordToolUsage(tool.Name(), tool.ID(), map[string]string{
				"cmd": cmdText,
				"env": envPath,
			})

			// 开始执行
			go func() {
				a.TviewApp.QueueUpdateDraw(func() {
					inputField.SetDisabled(true)
					outputView.Write([]byte(fmt.Sprintf("[yellow]执行环境: %s\n执行参数: %s[-]\n", envPath, cmdText)))
				})

				err := run(envPath, cmdText, outputView)

				a.TviewApp.QueueUpdateDraw(func() {
					if err != nil {
						outputView.Write([]byte(fmt.Sprintf("\n[red]执行出错: %v[-]\n", err)))
					} else {
						outputView.Write([]byte("\n[green]执行完成[-]\n"))
					}
					inputField.SetDisabled(false)
					a.TviewApp.SetFocus(inputField)
				})
			}()
			return nil
		}
		if event.Key() == tcell.KeyEnter && event.Modifiers()&tcell.ModShift != 0 {
			return event
		}
		return event
	})

	a.Pages.AddAndSwitchToPage("py_terminal", flex, true)
	a.TviewApp.SetFocus(inputField)
}
