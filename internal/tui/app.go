package tui

import (
	"context"
	_ "embed"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"my_tools/internal/storage"
	"my_tools/pkg/framework"

	"github.com/atotto/clipboard"
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

type ExecRecord struct {
	Cmd    string
	Env    string
	Output string
}

type TermUIState struct {
	Flex        *tview.Flex
	OutputRow   *tview.Flex
	Usage       *tview.TextView
	Input       *tview.TextArea
	Env         *tview.InputField
	EnvForm     *tview.Form
	Output      *tview.TextView
	UndoBuffer  string
	Records     []*ExecRecord
	UndoRecords []*ExecRecord
	CancelFunc  context.CancelFunc
	TaskBar     *tview.Flex
	TaskList    *tview.List
	TaskStatus  *tview.TextView
	ShownTask   *Task
	Executing   bool
}

type historyWriter struct {
	target io.Writer
	record *ExecRecord
}

func (hw *historyWriter) Write(p []byte) (n int, err error) {
	hw.record.Output += string(p)
	return hw.target.Write(p)
}

type App struct {
	TviewApp   *tview.Application
	Pages      *tview.Pages
	Tree       *SalamanderTreeView
	Store      *storage.Storage
	TermUI     map[string]*TermUIState
	TaskBars   map[string]*TaskBarState
	lastToolID string
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
	// 动态计算font的最大宽度，确保不重叠
	fontMaxWidth := 0
	for _, line := range b.fontLines {
		runes := []rune(line)
		if len(runes) > fontMaxWidth {
			fontMaxWidth = len(runes)
		}
	}
	descStartX := x + logoWidth + 2 + fontMaxWidth + 2 // font后面留2个空格，往左挪一点

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
				mainc, style, _ := screen.Get(screenX, screenY)
				if mainc == " " {
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

func (c *AppContextImpl) ShowTerminal(title string, usage string, run func(ctx context.Context, args string, out io.Writer) error) {
	c.app.ShowTerminal(c.tool, title, usage, run)
}

func (c *AppContextImpl) ShowPythonTerminal(title string, usage string, run func(ctx context.Context, env string, args string, out io.Writer) error) {
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
		TermUI:   make(map[string]*TermUIState),
		TaskBars: make(map[string]*TaskBarState),
	}

	a.setupUI()

	// 全局按键
	a.TviewApp.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// 拦截全局 Ctrl+C 并克隆事件，防止 tview 内部默认调用 app.Stop()
		// 从而将其传递给具有焦点的 Primitive，使得各面板能自行处理中断逻辑 (如取消任务)
		if event.Key() == tcell.KeyCtrlC {
			return tcell.NewEventKey(event.Key(), event.Rune(), event.Modifiers())
		}

		// 全局快捷键说明页 F1
		if event.Key() == tcell.KeyF1 {
			a.showShortcutHelp()
			return nil
		}

		// q 退出 (仅在没有输入框获得焦点时生效，避免输入内容时误退)
		if event.Key() == tcell.KeyRune && event.Rune() == 'q' {
			if _, isInput := a.TviewApp.GetFocus().(*tview.InputField); !isInput {
				if _, isTextArea := a.TviewApp.GetFocus().(*tview.TextArea); !isTextArea {
					a.TviewApp.Stop()
					return nil
				}
			}
		}
		// Ctrl+P 或 '/' 呼出全局搜索
		if event.Key() == tcell.KeyCtrlP || (event.Key() == tcell.KeyRune && event.Rune() == '/') {
			if _, isInput := a.TviewApp.GetFocus().(*tview.InputField); !isInput {
				if _, isTextArea := a.TviewApp.GetFocus().(*tview.TextArea); !isTextArea {
					a.showCommandPalette()
					return nil
				}
			}
		}
		return event
	})

	return a
}

func (a *App) Run() error {
	// 移除 EnableMouse(true)，因为启用鼠标会接管终端的鼠标事件，导致用户无法通过鼠标原生框选复制文本。
	return a.TviewApp.SetRoot(a.Pages, true).Run()
}

func (a *App) getTitleWithShortcut(baseTitle string, verboseShortcuts string, isMainHint bool) string {
	if a.Store.GetShowVerboseShortcuts() {
		return " " + baseTitle + " [gray](" + verboseShortcuts + ")[-] "
	}
	if isMainHint {
		return " " + baseTitle + " [gray](F1:快捷键)[-] "
	}
	return " " + baseTitle + " "
}

func (a *App) UpdateAllPanelTitles() {
	// We can't easily update banner here without re-creating it, but setupUI() already recreates it.
	// We just need to update the cached TermUI states.
	for _, ui := range a.TermUI {
		if ui.Usage != nil {
			isPy := strings.Contains(ui.Usage.GetTitle(), "Python专属")
			if isPy {
				ui.Usage.SetTitle(a.getTitleWithShortcut("📖 使用说明 (Python专属)", "Ctrl+E:全屏/还原", true))
			} else {
				ui.Usage.SetTitle(a.getTitleWithShortcut("📖 使用说明", "Ctrl+E:全屏/还原", true))
			}
		}
		if ui.Output != nil {
			ui.Output.SetTitle(a.getTitleWithShortcut("📺 终端输出", "Ctrl+L:清空, Ctrl+U:撤销清空, Ctrl+S:导出, Ctrl+E:全屏", false))
		}
		if ui.Input != nil {
			ui.Input.SetTitle(a.getTitleWithShortcut("⌨️ 命令行输入", "Enter:执行, ESC:返回, ↑/↓:历史, Tab:切换, Ctrl+E:全屏, Ctrl+N:新命令, Ctrl+B:任务, Ctrl+A:复制", false))
		}
		if ui.EnvForm != nil {
			ui.EnvForm.SetTitle(a.getTitleWithShortcut("⚙️ Python 环境配置", "Tab:切换, Ctrl+E:全屏", false))
		}
		if ui.TaskBar != nil {
			ui.TaskBar.SetTitle(a.getTitleWithShortcut("📋 任务", "Ctrl+B:隐藏, Ctrl+D:清理, Ctrl+C:取消", false))
		}
	}
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
		SetTitle(a.getTitleWithShortcut("🦎 火蜥蜴工具箱", "←/→/↑/↓:导航, Enter:执行, Ctrl+P:搜索, r:重置, b:折叠, q:退出", true)).
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
				_ = a.Store.Save()
				updateBannerHeight()
				return nil
			}
		}
		return event
	})

	a.Pages.AddAndSwitchToPage("main", mainLayout, true)
}

func (a *App) expandParents(target *tview.TreeNode) {
	p := a.findParent(a.Tree.GetRoot(), target)
	for p != nil && p != a.Tree.GetRoot() {
		p.SetExpanded(true)
		if nodeID := getNodeID(p); nodeID != "" {
			a.Store.SetNodeState(nodeID, true)
		}
		p = a.findParent(a.Tree.GetRoot(), p)
	}
}

func (a *App) findNodeByToolID(toolID string) *tview.TreeNode {
	if a.Tree == nil || a.Tree.GetRoot() == nil {
		return nil
	}
	var found *tview.TreeNode
	a.Tree.GetRoot().Walk(func(node, parent *tview.TreeNode) bool {
		// 跳过“最近使用”分支，确保光标回到工具的真实分类目录下
		if node.GetText() == " ⭐ 最近使用 " {
			return false // 不进入该分支
		}
		if t, ok := node.GetReference().(framework.Tool); ok && t.ID() == toolID {
			found = node
			return false
		}
		return true
	})
	return found
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

type sysTerminalTool struct{}

func (s *sysTerminalTool) ID() string       { return "sys_terminal" }
func (s *sysTerminalTool) Name() string     { return "命令行 Shell" }
func (s *sysTerminalTool) Category() string { return "💻 系统与设置" }
func (s *sysTerminalTool) Execute(ctx framework.AppContext) {
	pwd, _ := os.Getwd()
	cmdPrompt := "dir"
	if runtime.GOOS != "windows" {
		cmdPrompt = "ls"
	}
	usage := fmt.Sprintf("直接输入系统命令 (如 %s, ping, echo) 并回车执行。\n[red](注: 按 Ctrl+C 可以随时中断正在执行的命令)[-]\n当前工作目录: %s", cmdPrompt, pwd)

	ctx.ShowTerminal("系统终端", usage, func(runCtx context.Context, args string, out io.Writer) error {
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.Command("cmd", "/c", args)
		} else {
			cmd = exec.CommandContext(runCtx, "sh", "-c", args)
		}
		cmd.Stdout = out
		cmd.Stderr = out

		if err := cmd.Start(); err != nil {
			return err
		}

		done := make(chan struct{})
		defer close(done)

		if runtime.GOOS == "windows" {
			go func() {
				select {
				case <-runCtx.Done():
					if cmd.Process != nil {
						_ = exec.Command("taskkill", "/T", "/F", "/PID", fmt.Sprint(cmd.Process.Pid)).Run()
					}
				case <-done:
				}
			}()
		}

		return cmd.Wait()
	})
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
				toolColor := colorGo
				icon := "🔧"
				if strings.Contains(targetTool.Category(), "Python") {
					toolColor = colorPy
					icon = "📄"
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

		// 解析多级分类 (如 "KD测试工具 > 点云处理工具")
		parts := strings.Split(catName, " > ")
		currentParent := parentNode
		for i, part := range parts {
			partKey := strings.TrimSpace(part)
			fullPath := strings.Join(parts[:i+1], " > ")

			catNode, exists := catMap[fullPath]
			if !exists {
				icon := "📁"
				color := tcell.ColorLightSalmon
				if strings.Contains(partKey, "Python") {
					icon = "🐍"
					color = tcell.ColorPaleGoldenrod
				}
				catNodeText := fmt.Sprintf(" %s %s", icon, partKey)
				catNode = tview.NewTreeNode(catNodeText).
					SetColor(color).
					SetSelectable(true).
					SetExpanded(a.Store.GetNodeState(strings.TrimSpace(catNodeText), true))
				catMap[fullPath] = catNode
				currentParent.AddChild(catNode)
			}
			currentParent = catNode
		}

		icon := "🔧"
		color := colorGo
		if strings.Contains(catName, "Python") {
			icon = "📄"
			color = colorPy
		}
		toolNode := tview.NewTreeNode(fmt.Sprintf(" %s %s", icon, t.Name())).
			SetReference(t).
			SetColor(color).
			SetSelectable(true)

		currentParent.AddChild(toolNode)
	}

	root.AddChild(nativeNode)
	root.AddChild(tview.NewTreeNode("").SetSelectable(false)) // 空行分隔
	root.AddChild(scriptNode)
	root.AddChild(tview.NewTreeNode("").SetSelectable(false)) // 空行分隔

	// --- 4. 终端系统 ---
	sysTermNode := tview.NewTreeNode(" 💻 系统与设置 ").
		SetColor(tcell.ColorSkyblue).
		SetSelectable(true).
		SetExpanded(a.Store.GetNodeState("💻 系统与设置", true))

	termTool := &sysTerminalTool{}
	termItem := tview.NewTreeNode(fmt.Sprintf(" 📟 %s", termTool.Name())).
		SetReference(termTool).
		SetColor(tcell.ColorLightCyan).
		SetSelectable(true)
	sysTermNode.AddChild(termItem)
	root.AddChild(sysTermNode)

	root.AddChild(tview.NewTreeNode("").SetSelectable(false)) // 空行分隔

	// --- 5. 系统设置 ---
	setTool := &settingsTool{app: a}
	setNode := tview.NewTreeNode(" ⚙️ 系统首选项 ").
		SetReference(setTool).
		SetColor(tcell.ColorSilver).
		SetSelectable(true)
	sysTermNode.AddChild(setNode) // 添加到 💻 系统与设置 节点下

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

func (a *App) ShowModal(title, message string) {
	frontPage, _ := a.Pages.GetFrontPage()
	focus := a.TviewApp.GetFocus()

	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"确定"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			a.Pages.RemovePage("modal")
			if frontPage != "" {
				a.Pages.SwitchToPage(frontPage)
			}
			if focus != nil {
				a.TviewApp.SetFocus(focus)
			}
		})
	modal.SetBorder(true).
		SetTitle(" " + title + " ").
		SetTitleColor(tcell.ColorWhite).
		SetBackgroundColor(colorBgLight)
	a.Pages.AddAndSwitchToPage("modal", modal, true)
	a.TviewApp.SetFocus(modal)
}

func (a *App) PromptInput(title, prompt, defaultValue string, callback func(string)) {
	frontPage, _ := a.Pages.GetFrontPage()
	focus := a.TviewApp.GetFocus()

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
		if frontPage != "" {
			a.Pages.SwitchToPage(frontPage)
		}
		if focus != nil {
			a.TviewApp.SetFocus(focus)
		}
		callback(text)
	})
	form.AddButton("取消", func() {
		a.Pages.RemovePage("input")
		if frontPage != "" {
			a.Pages.SwitchToPage(frontPage)
		}
		if focus != nil {
			a.TviewApp.SetFocus(focus)
		}
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

func (a *App) showCommandPalette() {
	flex := tview.NewFlex().SetDirection(tview.FlexRow)

	tree := tview.NewTreeView().SetGraphics(true)
	tree.SetBorder(true).
		SetBorderColor(tcell.ColorDarkGray).
		SetTitle(" 🔍 搜索结果 ").
		SetTitleColor(tcell.ColorGray)

	input := tview.NewInputField().
		SetLabel(" 🔎 搜索工具/目录: ").
		SetLabelColor(tcell.ColorOrange).
		SetFieldBackgroundColor(colorBgDark).
		SetFieldTextColor(tcell.ColorWhite)

	input.SetBorder(true).
		SetBorderColor(tcell.ColorYellow).
		SetTitle(a.getTitleWithShortcut("⚡ 快捷命令面板", "ESC:退出", true)).
		SetTitleColor(tcell.ColorYellow)

	// 更新树
	updateTree := func(query string) {
		query = strings.ToLower(query)
		root := tview.NewTreeNode("搜索结果").SetSelectable(false)

		nativeNode := tview.NewTreeNode(" 🚀 内置原生应用 ").SetColor(tcell.ColorSalmon).SetSelectable(true)
		scriptNode := tview.NewTreeNode(" 📜 扩展兼容脚本 ").SetColor(tcell.ColorKhaki).SetSelectable(true)

		nativeCategories := make(map[string]*tview.TreeNode)
		scriptCategories := make(map[string]*tview.TreeNode)

		hasNative := false
		hasScript := false

		for _, t := range framework.Registry {
			name := strings.ToLower(t.Name())
			cat := strings.ToLower(t.Category())

			matchTool := strings.Contains(name, query)
			matchCat := strings.Contains(cat, query)

			if query == "" || matchTool || matchCat {
				var parentNode *tview.TreeNode
				var catMap map[string]*tview.TreeNode
				isScript := strings.Contains(t.Category(), "脚本")

				if isScript {
					parentNode = scriptNode
					catMap = scriptCategories
					hasScript = true
				} else {
					parentNode = nativeNode
					catMap = nativeCategories
					hasNative = true
				}

				// 解析多级分类
				parts := strings.Split(t.Category(), " > ")
				currentParent := parentNode
				for i, part := range parts {
					partKey := strings.TrimSpace(part)
					fullPath := strings.Join(parts[:i+1], " > ")

					catNode, exists := catMap[fullPath]
					if !exists {
						icon := "📁"
						color := tcell.ColorLightSalmon
						if strings.Contains(partKey, "Python") {
							icon = "🐍"
							color = tcell.ColorPaleGoldenrod
						}
						catNode = tview.NewTreeNode(fmt.Sprintf(" %s %s", icon, partKey)).
							SetColor(color).
							SetSelectable(true).
							SetExpanded(true)
						catMap[fullPath] = catNode
						currentParent.AddChild(catNode)
					}
					currentParent = catNode
				}

				icon := "🔧"
				color := colorGo
				if strings.Contains(t.Category(), "Python") {
					icon = "📄"
					color = colorPy
				}
				toolNode := tview.NewTreeNode(fmt.Sprintf(" %s %s", icon, t.Name())).
					SetReference(t).
					SetColor(color).
					SetSelectable(true)
				currentParent.AddChild(toolNode)
			}
		}

		if hasNative {
			root.AddChild(nativeNode)
			nativeNode.SetExpanded(true)
		}
		if hasScript {
			root.AddChild(scriptNode)
			scriptNode.SetExpanded(true)
		}
		if !hasNative && !hasScript {
			root.AddChild(tview.NewTreeNode(" (无匹配结果) ").SetColor(tcell.ColorGray).SetSelectable(false))
		}

		tree.SetRoot(root)
		tree.SetCurrentNode(root)
		if len(root.GetChildren()) > 0 {
			tree.SetCurrentNode(root.GetChildren()[0])
		}
	}

	// 初始显示全部
	updateTree("")

	input.SetChangedFunc(func(text string) {
		updateTree(text)
	})

	tree.SetSelectedFunc(func(node *tview.TreeNode) {
		ref := node.GetReference()
		if ref == nil {
			node.SetExpanded(!node.IsExpanded())
			return
		}
		if tool, ok := ref.(framework.Tool); ok {
			a.Pages.RemovePage("palette")
			a.Pages.SwitchToPage("main")
			a.executeTool(tool)
		}
	})

	focusables := []tview.Primitive{input, tree}
	currentFocus := 0

	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
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

	input.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyDown {
			a.TviewApp.SetFocus(tree)
			currentFocus = 1
			return nil
		}
		return event
	})

	tree.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyRight:
			node := tree.GetCurrentNode()
			if node != nil {
				if len(node.GetChildren()) > 0 && !node.IsExpanded() {
					node.SetExpanded(true)
				} else if len(node.GetChildren()) > 0 && node.IsExpanded() {
					tree.SetCurrentNode(node.GetChildren()[0])
				} else if ref := node.GetReference(); ref != nil {
					if tool, ok := ref.(framework.Tool); ok {
						a.Pages.RemovePage("palette")
						a.Pages.SwitchToPage("main")
						a.executeTool(tool)
					}
				}
			}
			return nil
		case tcell.KeyLeft:
			node := tree.GetCurrentNode()
			if node != nil {
				if parent := a.findParent(tree.GetRoot(), node); parent != nil && parent != tree.GetRoot() {
					tree.SetCurrentNode(parent)
				}
			}
			return nil
		}
		return event
	})

	flex.AddItem(input, 3, 1, true).
		AddItem(tree, 0, 1, false)

	// 使用一个空 Box 来做背景遮罩，使模态框居中
	modalLayout := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(flex, 20, 1, true).              // 高度 20
			AddItem(nil, 0, 1, false), 60, 1, true). // 宽度 60
		AddItem(nil, 0, 1, false)

	modalLayout.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			a.Pages.RemovePage("palette")
			a.Pages.SwitchToPage("main")
			a.TviewApp.SetFocus(a.Tree)
			return nil
		}
		return event
	})

	a.Pages.AddAndSwitchToPage("palette", modalLayout, true)
	a.TviewApp.SetFocus(input)
}

func (a *App) RecordToolUsage(name, toolID string, params map[string]string) {
	a.Store.AddRecentTool(name, toolID, params)
	_ = a.Store.Save()
	// 不在这里立即刷新树，而是延迟到退出 (ESC) 时刷新
}

func (a *App) ShowTerminal(tool framework.Tool, title string, usage string, run func(ctx context.Context, args string, out io.Writer) error) {
	pageID := "term_" + tool.ID()
	a.lastToolID = tool.ID()
	if ui, ok := a.TermUI[tool.ID()]; ok {
		a.Pages.SwitchToPage(pageID)
		a.TviewApp.SetFocus(ui.Input)
		return
	}

	uiState := &TermUIState{}
	a.TermUI[tool.ID()] = uiState

	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	uiState.Flex = flex

	barState, _ := a.ensureTaskBar(uiState, tool.ID())

	usageView := tview.NewTextView().
		SetDynamicColors(true).
		SetText(usage).
		SetScrollable(true)
	usageView.SetBorder(true).
		SetTitle(a.getTitleWithShortcut("📖 使用说明", "Ctrl+E:全屏/还原", true)).
		SetTitleColor(colorGo).
		SetBorderColor(tcell.ColorDarkGray)
	uiState.Usage = usageView

	outputView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	outputView.SetChangedFunc(func() {
		outputView.ScrollToEnd()
		a.TviewApp.Draw()
	})
	uiState.Output = outputView

	outputView.SetBorder(true).
		SetTitle(a.getTitleWithShortcut("📺 终端输出", "Ctrl+L:清空, Ctrl+U:撤销清空, Ctrl+S:导出, Ctrl+E:全屏", false)).
		SetTitleColor(tcell.ColorYellow).
		SetBorderColor(tcell.ColorDarkGray)

	outputRow := tview.NewFlex().SetDirection(tview.FlexColumn)
	outputRow.AddItem(outputView, 0, 1, false)
	uiState.OutputRow = outputRow

	initialCmd := ""
	history := a.Store.GetToolHistory(tool.ID())
	if len(history) > 0 {
		if cmd, ok := history[0]["cmd"]; ok {
			initialCmd = cmd
		}
	}
	historyIndex := -1

	inputField := tview.NewTextArea().
		SetLabel(" ❯ ").
		SetText(initialCmd, true)
	inputField.SetBackgroundColor(colorBgDark)
	inputField.SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorWhite))
	uiState.Input = inputField

	inputField.SetBorder(true).
		SetTitle(a.getTitleWithShortcut("⌨️ 命令行输入", "Enter:执行, ESC:返回, ↑/↓:历史, Tab:切换, Ctrl+E:全屏, Ctrl+N:新命令, Ctrl+B:任务, Ctrl+A:复制", false)).
		SetTitleColor(tcell.ColorOrange).
		SetBorderColor(tcell.ColorDarkGray)

	flex.AddItem(usageView, 0, 1, false).
		AddItem(outputRow, 0, 3, false).
		AddItem(inputField, 5, 1, true)

	type layoutItem struct {
		p     tview.Primitive
		fixed int
		prop  int
	}
	layoutItems := []layoutItem{
		{usageView, 0, 1},
		{outputRow, 0, 3},
		{inputField, 5, 1},
	}
	isMaximized := false

	outputRowFocused := func() bool {
		return outputView.HasFocus() || (uiState.TaskList != nil && uiState.TaskList.HasFocus())
	}

	toggleMaximize := func() {
		if isMaximized {
			for _, item := range layoutItems {
				flex.ResizeItem(item.p, item.fixed, item.prop)
			}
			isMaximized = false
		} else {
			var target tview.Primitive
			for _, item := range layoutItems {
				if item.p == outputRow && outputRowFocused() {
					target = outputRow
					break
				}
				if item.p.HasFocus() {
					target = item.p
					break
				}
			}
			if target != nil {
				for _, item := range layoutItems {
					if item.p == target {
						flex.ResizeItem(item.p, 0, 1)
					} else {
						flex.ResizeItem(item.p, 0, 0)
					}
				}
				isMaximized = true
			}
		}
	}

	usageView.SetFocusFunc(func() { usageView.SetBorderColor(colorGo) })
	usageView.SetBlurFunc(func() { usageView.SetBorderColor(tcell.ColorDarkGray) })
	outputView.SetFocusFunc(func() { outputView.SetBorderColor(tcell.ColorYellow) })
	outputView.SetBlurFunc(func() { outputView.SetBorderColor(tcell.ColorDarkGray) })
	inputField.SetFocusFunc(func() { inputField.SetBorderColor(tcell.ColorOrange) })
	inputField.SetBlurFunc(func() { inputField.SetBorderColor(tcell.ColorDarkGray) })

	focusables := []tview.Primitive{usageView, outputView, inputField}
	currentFocus := 2

	rebuildFocusables := func() {
		focusables = focusables[:0]
		focusables = append(focusables, usageView, outputView)
		if uiState.TaskList != nil && barState.Visible {
			focusables = append(focusables, uiState.TaskList)
		}
		focusables = append(focusables, inputField)
	}

	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlC {
			if uiState.ShownTask != nil && uiState.ShownTask.Cancel != nil && uiState.ShownTask.Status == StatusRunning {
				_, _ = outputView.Write([]byte("\n[red]正在取消任务...[-]\n"))
				uiState.ShownTask.Cancel()
			} else if uiState.CancelFunc != nil {
				_, _ = outputView.Write([]byte("\n[red]正在取消任务 (旧版)...[-]\n"))
				uiState.CancelFunc()
			}
			return nil
		}
		if event.Key() == tcell.KeyEscape {
			if isMaximized {
				toggleMaximize()
				return nil
			}
			a.refreshTree() // 退出时刷新树，更新最近使用
			a.Pages.SwitchToPage("main")
			if a.lastToolID != "" {
				if found := a.findNodeByToolID(a.lastToolID); found != nil {
					a.expandParents(found)
					a.Tree.SetCurrentNode(found)
				}
			}
			a.TviewApp.SetFocus(a.Tree)

			// 解决 tview 树组件在替换根节点后首次 Draw 无法正确定位光标的 Bug
			go func() {
				time.Sleep(20 * time.Millisecond)
				a.TviewApp.QueueUpdateDraw(func() {
					if a.lastToolID != "" {
						if found := a.findNodeByToolID(a.lastToolID); found != nil {
							a.Tree.SetCurrentNode(found)
						}
					}
					a.TviewApp.SetFocus(a.Tree)
				})
			}()

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
		if event.Key() == tcell.KeyCtrlE {
			toggleMaximize()
			return nil
		}
		if event.Key() == tcell.KeyCtrlB {
			if uiState.TaskList == nil {
				return nil
			}
			if !barState.Visible {
				a.showTaskBar(uiState, barState)
				rebuildFocusables()
				a.TviewApp.SetFocus(uiState.TaskList)
				return nil
			}
			if uiState.TaskList.HasFocus() {
				a.TviewApp.SetFocus(inputField)
				if uiState.ShownTask != nil && uiState.ShownTask.Status != StatusRunning {
					uiState.Executing = false
				} else if uiState.ShownTask == nil {
					uiState.Executing = false
				}
				inputField.SetDisabled(uiState.Executing)
			} else {
				a.TviewApp.SetFocus(uiState.TaskList)
			}
			return nil
		}
		if event.Key() == tcell.KeyCtrlN {
			outputView.Clear()
			uiState.Executing = false
			inputField.SetDisabled(false)
			uiState.ShownTask = nil
			a.TviewApp.SetFocus(inputField)
			return nil
		}
		return event
	})

	outputView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlC {
			if uiState.ShownTask != nil && uiState.ShownTask.Cancel != nil && uiState.ShownTask.Status == StatusRunning {
				_, _ = outputView.Write([]byte("\n[red]正在取消任务...[-]\n"))
				uiState.ShownTask.Cancel()
			} else if uiState.CancelFunc != nil {
				_, _ = outputView.Write([]byte("\n[red]正在取消任务 (旧版)...[-]\n"))
				uiState.CancelFunc()
			}
			return nil
		}
		if event.Key() == tcell.KeyCtrlS {
			a.exportLog(tool.Name(), outputView.GetText(true))
			return nil
		}
		if event.Key() == tcell.KeyCtrlL {
			uiState.UndoBuffer = outputView.GetText(false)
			uiState.UndoRecords = append([]*ExecRecord(nil), uiState.Records...)
			outputView.Clear()
			uiState.Records = nil
			return nil
		}
		if event.Key() == tcell.KeyCtrlU {
			if uiState.UndoBuffer != "" {
				outputView.SetText(uiState.UndoBuffer)
				uiState.Records = uiState.UndoRecords
				uiState.UndoBuffer = ""
				uiState.UndoRecords = nil
			}
			return nil
		}
		return event
	})

	inputField.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlC {
			if uiState.ShownTask != nil && uiState.ShownTask.Cancel != nil && uiState.ShownTask.Status == StatusRunning {
				_, _ = outputView.Write([]byte("\n[red]正在取消任务...[-]\n"))
				uiState.ShownTask.Cancel()
			} else if uiState.CancelFunc != nil {
				_, _ = outputView.Write([]byte("\n[red]正在取消任务 (旧版)...[-]\n"))
				uiState.CancelFunc()
			}
			return nil
		}
		if event.Key() == tcell.KeyCtrlA {
			cmdText := inputField.GetText()
			if err := a.copyToClipboard(cmdText); err == nil {
				a.ShowModal("复制成功", "命令已复制到剪贴板！")
			} else {
				a.ShowModal("复制失败", err.Error())
			}
			return nil
		}
		if event.Key() == tcell.KeyUp {
			if len(history) > 0 && historyIndex < len(history)-1 {
				historyIndex++
				if cmd, ok := history[historyIndex]["cmd"]; ok {
					inputField.SetText(cmd, true)
				}
			}
			return nil
		}
		if event.Key() == tcell.KeyDown {
			if historyIndex > 0 {
				historyIndex--
				if cmd, ok := history[historyIndex]["cmd"]; ok {
					inputField.SetText(cmd, true)
				}
			} else if historyIndex == 0 {
				historyIndex = -1
				inputField.SetText("", true)
			}
			return nil
		}
		if event.Key() == tcell.KeyEnter && event.Modifiers() == tcell.ModNone {
			if uiState.Executing {
				return nil
			}
			cmdText := strings.TrimSpace(inputField.GetText())

			if tool.ID() == "sys_terminal" {
				inputField.SetText("", true)
			}

			a.RecordToolUsage(tool.Name(), tool.ID(), map[string]string{"cmd": cmdText})
			history = a.Store.GetToolHistory(tool.ID())
			historyIndex = -1

			uiState.UndoBuffer = ""
			uiState.UndoRecords = nil

			record := &ExecRecord{Cmd: cmdText}
			uiState.Records = append(uiState.Records, record)

			runCtx, cancel := context.WithCancel(context.Background())
			task := &Task{
				ID:        fmt.Sprintf("task_%s_%d", tool.ID(), time.Now().UnixMilli()),
				ToolID:    tool.ID(),
				ToolName:  tool.Name(),
				Cmd:       cmdText,
				Status:    StatusRunning,
				CreatedAt: time.Now(),
				Cancel:    cancel,
			}

			barState.mu.Lock()
			barState.Tasks = append(barState.Tasks, task)
			barState.ActiveIdx = len(barState.Tasks) - 1
			a.refreshTaskList(uiState, barState)
			visible := barState.Visible
			barState.mu.Unlock()

			if !visible {
				a.showTaskBar(uiState, barState)
				rebuildFocusables()
			}

			uiState.ShownTask = task

			outputView.Clear()
			prefix := fmt.Sprintf("[yellow]❯ 执行命令: %s[-]\n", cmdText)
			task.Output += prefix
			record.Output += prefix

			a.taskStarted(uiState)
			_, _ = outputView.Write([]byte(prefix))

			go func() {
				defer func() {
					if r := recover(); r != nil {
						a.TviewApp.QueueUpdateDraw(func() {
							task.Status = StatusFailed
							task.EndedAt = time.Now()
							barState.mu.Lock()
							a.refreshTaskList(uiState, barState)
							barState.mu.Unlock()
							a.taskFinished(uiState)
							_, _ = outputView.Write([]byte(fmt.Sprintf("\n[red]任务异常崩溃: %v[-]\n", r)))
							outputView.ScrollToEnd()
						})
					}
				}()
				ot := &outputTracker{
					Writer:    tview.ANSIWriter(outputView),
					Task:      task,
					ShownTask: &uiState.ShownTask,
				}

				err := run(runCtx, cmdText, ot)

				a.TviewApp.QueueUpdateDraw(func() {
					if err != nil && !ot.wroteBytes {
						barState.mu.Lock()
						a.removeTask(barState, task)
						taskCount := len(barState.Tasks)
						if taskCount > 0 {
							a.refreshTaskList(uiState, barState)
						}
						barState.mu.Unlock()

						if taskCount == 0 {
							a.hideTaskBar(uiState, barState)
							rebuildFocusables()
						}

						errStr := fmt.Sprintf("\n[red]执行出错: %v[-]\n", err)
						_, _ = outputView.Write([]byte(errStr))
						record.Output += errStr
						uiState.ShownTask = nil
						a.taskFinished(uiState)
						a.TviewApp.SetFocus(inputField)
						outputView.ScrollToEnd()
						return
					}

					finalStatus := parseTaskResult(task, err)
					task.Status = finalStatus
					task.EndedAt = time.Now()

					if finalStatus == StatusFailed {
						errStr := fmt.Sprintf("\n[red]执行异常: %v[-]\n", err)
						if err == nil {
							errStr = "\n[red]执行失败（无有效结果）[-]\n"
						}
						_, _ = outputView.Write([]byte(errStr))
						task.Output += errStr
						record.Output += errStr
					} else {
						succStr := "\n[green]执行完成[-]\n"
						_, _ = outputView.Write([]byte(succStr))
						task.Output += succStr
						record.Output += succStr
					}

					barState.mu.Lock()
					a.refreshTaskList(uiState, barState)
					barState.mu.Unlock()

					a.taskFinished(uiState)
					a.TviewApp.SetFocus(inputField)
					outputView.ScrollToEnd()
				})
			}()
			return nil
		}
		if event.Key() == tcell.KeyEnter && event.Modifiers()&tcell.ModShift != 0 {
			return event
		}
		return event
	})

	a.Pages.AddAndSwitchToPage(pageID, flex, true)
	a.TviewApp.SetFocus(inputField)
}

func (a *App) ShowPythonTerminal(tool framework.Tool, title string, usage string, run func(ctx context.Context, env string, args string, out io.Writer) error) {
	pageID := "term_" + tool.ID()
	a.lastToolID = tool.ID()
	if ui, ok := a.TermUI[tool.ID()]; ok {
		a.Pages.SwitchToPage(pageID)
		a.TviewApp.SetFocus(ui.Input)
		return
	}

	uiState := &TermUIState{}
	a.TermUI[tool.ID()] = uiState

	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	uiState.Flex = flex

	barState, _ := a.ensureTaskBar(uiState, tool.ID())

	usageView := tview.NewTextView().
		SetDynamicColors(true).
		SetText(usage).
		SetScrollable(true)
	usageView.SetBorder(true).
		SetTitle(a.getTitleWithShortcut("📖 使用说明 (Python专属)", "Ctrl+E:全屏/还原", true)).
		SetTitleColor(colorPy).
		SetBorderColor(tcell.ColorDarkGray)
	uiState.Usage = usageView

	outputView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	outputView.SetChangedFunc(func() {
		outputView.ScrollToEnd()
		a.TviewApp.Draw()
	})
	uiState.Output = outputView

	outputView.SetBorder(true).
		SetTitle(a.getTitleWithShortcut("📺 终端输出", "Ctrl+L:清空, Ctrl+U:撤销清空, Ctrl+S:导出, Ctrl+E:全屏", false)).
		SetTitleColor(tcell.ColorYellow).
		SetBorderColor(tcell.ColorDarkGray)

	outputRow := tview.NewFlex().SetDirection(tview.FlexColumn)
	outputRow.AddItem(outputView, 0, 1, false)
	uiState.OutputRow = outputRow

	initialCmd := ""
	initialEnv := "python"
	history := a.Store.GetToolHistory(tool.ID())
	if len(history) > 0 {
		if cmd, ok := history[0]["cmd"]; ok {
			initialCmd = cmd
		}
		if env, ok := history[0]["env"]; ok {
			initialEnv = env
		}
	}
	historyIndex := -1

	envForm := tview.NewForm().
		SetFieldBackgroundColor(colorBgDark).
		SetFieldTextColor(tcell.ColorWhite).
		SetLabelColor(colorPy).
		SetButtonBackgroundColor(tcell.ColorDarkGreen).
		SetButtonTextColor(tcell.ColorWhite)

	envForm.SetBorder(true).
		SetTitle(a.getTitleWithShortcut("⚙️ Python 环境配置", "Tab:切换, Ctrl+E:全屏", false)).
		SetTitleColor(colorPy).
		SetBorderColor(tcell.ColorDarkGray)
	uiState.EnvForm = envForm

	envInput := tview.NewInputField().
		SetLabel("解释器路径: ").
		SetText(initialEnv).
		SetFieldWidth(30)
	envForm.AddFormItem(envInput)
	uiState.Env = envInput

	envInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlC {
			if uiState.ShownTask != nil && uiState.ShownTask.Cancel != nil && uiState.ShownTask.Status == StatusRunning {
				_, _ = outputView.Write([]byte("\n[red]正在取消任务...[-]\n"))
				uiState.ShownTask.Cancel()
			} else if uiState.CancelFunc != nil {
				_, _ = outputView.Write([]byte("\n[red]正在取消任务 (旧版)...[-]\n"))
				uiState.CancelFunc()
			}
			return nil
		}
		return event
	})

	envForm.AddButton("📦 安装 pip 依赖", func() {
		envPath := strings.TrimSpace(envInput.GetText())
		if envPath == "" {
			envPath = "python"
		}

		a.PromptInput("安装 pip 依赖", "输入要安装的包名 (如 requests pandas):", "", func(pkgName string) {
			if pkgName == "" {
				return
			}

			uiState.UndoBuffer = ""
			uiState.UndoRecords = nil

			record := &ExecRecord{Cmd: "!pip " + pkgName, Env: envPath}
			uiState.Records = append(uiState.Records, record)

			prefix := fmt.Sprintf("\n[yellow]❯ 正在使用 %s 安装依赖: %s[-]\n", envPath, pkgName)
			_, _ = outputView.Write([]byte(prefix))
			record.Output += prefix

			go func() {
				defer func() {
					if r := recover(); r != nil {
						a.TviewApp.QueueUpdateDraw(func() {
							_, _ = outputView.Write([]byte(fmt.Sprintf("\n[red]安装崩溃: %v[-]\n", r)))
							outputView.ScrollToEnd()
						})
					}
				}()
				runCtx, cancel := context.WithCancel(context.Background())
				uiState.CancelFunc = cancel

				hw := &historyWriter{target: tview.ANSIWriter(outputView), record: record}

				var cmd *exec.Cmd
				if runtime.GOOS == "windows" {
					cmd = exec.Command("cmd", "/c", envPath, "-m", "pip", "install", pkgName)
				} else {
					cmd = exec.CommandContext(runCtx, envPath, "-m", "pip", "install", pkgName)
				}
				cmd.Stdout = hw
				cmd.Stderr = hw

				var err error
				if startErr := cmd.Start(); startErr != nil {
					err = startErr
				} else {
					done := make(chan struct{})

					if runtime.GOOS == "windows" {
						go func() {
							select {
							case <-runCtx.Done():
								if cmd.Process != nil {
									_ = exec.Command("taskkill", "/T", "/F", "/PID", fmt.Sprint(cmd.Process.Pid)).Run()
								}
							case <-done:
							}
						}()
					}
					err = cmd.Wait()
					close(done)
				}

				a.TviewApp.QueueUpdateDraw(func() {
					uiState.CancelFunc = nil
					if err != nil {
						errStr := fmt.Sprintf("\n[red]安装出错: %v[-]\n", err)
						_, _ = outputView.Write([]byte(errStr))
						record.Output += errStr
					} else {
						succStr := "\n[green]安装完成！[-]\n"
						_, _ = outputView.Write([]byte(succStr))
						record.Output += succStr
					}
					outputView.ScrollToEnd()
				})
			}()
		})
	})

	inputField := tview.NewTextArea().
		SetLabel(" ❯ ").
		SetText(initialCmd, true)
	inputField.SetBackgroundColor(colorBgDark)
	inputField.SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorWhite))
	uiState.Input = inputField

	inputField.SetBorder(true).
		SetTitle(a.getTitleWithShortcut("⌨️ 命令行输入", "Enter:执行, ESC:返回, ↑/↓:历史, Tab:切换, Ctrl+E:全屏, Ctrl+N:新命令, Ctrl+B:任务, Ctrl+A:复制", false)).
		SetTitleColor(tcell.ColorOrange).
		SetBorderColor(tcell.ColorDarkGray)

	flex.AddItem(usageView, 0, 1, false).
		AddItem(outputRow, 0, 3, false).
		AddItem(envForm, 5, 1, false).
		AddItem(inputField, 5, 1, true)

	type layoutItem struct {
		p     tview.Primitive
		fixed int
		prop  int
	}
	layoutItems := []layoutItem{
		{usageView, 0, 1},
		{outputRow, 0, 3},
		{envForm, 5, 1},
		{inputField, 5, 1},
	}
	isMaximized := false

	outputRowFocused := func() bool {
		return outputView.HasFocus() || (uiState.TaskList != nil && uiState.TaskList.HasFocus())
	}

	toggleMaximize := func() {
		if isMaximized {
			for _, item := range layoutItems {
				flex.ResizeItem(item.p, item.fixed, item.prop)
			}
			isMaximized = false
		} else {
			var target tview.Primitive
			for _, item := range layoutItems {
				if item.p == outputRow && outputRowFocused() {
					target = outputRow
					break
				}
				if item.p.HasFocus() {
					target = item.p
					break
				}
			}
			if target != nil {
				for _, item := range layoutItems {
					if item.p == target {
						flex.ResizeItem(item.p, 0, 1)
					} else {
						flex.ResizeItem(item.p, 0, 0)
					}
				}
				isMaximized = true
			}
		}
	}

	usageView.SetFocusFunc(func() { usageView.SetBorderColor(colorPy) })
	usageView.SetBlurFunc(func() { usageView.SetBorderColor(tcell.ColorDarkGray) })
	outputView.SetFocusFunc(func() { outputView.SetBorderColor(tcell.ColorYellow) })
	outputView.SetBlurFunc(func() { outputView.SetBorderColor(tcell.ColorDarkGray) })
	envForm.SetFocusFunc(func() { envForm.SetBorderColor(colorPy) })
	envForm.SetBlurFunc(func() { envForm.SetBorderColor(tcell.ColorDarkGray) })
	inputField.SetFocusFunc(func() { inputField.SetBorderColor(tcell.ColorOrange) })
	inputField.SetBlurFunc(func() { inputField.SetBorderColor(tcell.ColorDarkGray) })

	focusables := []tview.Primitive{usageView, outputView, envForm, inputField}
	currentFocus := 3

	rebuildFocusables := func() {
		focusables = focusables[:0]
		focusables = append(focusables, usageView, outputView)
		if uiState.TaskList != nil && barState.Visible {
			focusables = append(focusables, uiState.TaskList)
		}
		focusables = append(focusables, envForm, inputField)
	}

	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlC {
			if uiState.ShownTask != nil && uiState.ShownTask.Cancel != nil && uiState.ShownTask.Status == StatusRunning {
				_, _ = outputView.Write([]byte("\n[red]正在取消任务...[-]\n"))
				uiState.ShownTask.Cancel()
			} else if uiState.CancelFunc != nil {
				_, _ = outputView.Write([]byte("\n[red]正在取消任务 (旧版)...[-]\n"))
				uiState.CancelFunc()
			}
			return nil
		}
		if event.Key() == tcell.KeyEscape {
			if isMaximized {
				toggleMaximize()
				return nil
			}
			a.refreshTree() // 退出时刷新树，更新最近使用
			a.Pages.SwitchToPage("main")
			if a.lastToolID != "" {
				if found := a.findNodeByToolID(a.lastToolID); found != nil {
					a.expandParents(found)
					a.Tree.SetCurrentNode(found)
				}
			}
			a.TviewApp.SetFocus(a.Tree)

			go func() {
				time.Sleep(20 * time.Millisecond)
				a.TviewApp.QueueUpdateDraw(func() {
					if a.lastToolID != "" {
						if found := a.findNodeByToolID(a.lastToolID); found != nil {
							a.Tree.SetCurrentNode(found)
						}
					}
					a.TviewApp.SetFocus(a.Tree)
				})
			}()

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
		if event.Key() == tcell.KeyCtrlE {
			toggleMaximize()
			return nil
		}
		if event.Key() == tcell.KeyCtrlB {
			if uiState.TaskList == nil {
				return nil
			}
			if !barState.Visible {
				a.showTaskBar(uiState, barState)
				rebuildFocusables()
				a.TviewApp.SetFocus(uiState.TaskList)
				return nil
			}
			if uiState.TaskList.HasFocus() {
				a.TviewApp.SetFocus(inputField)
				if uiState.ShownTask != nil && uiState.ShownTask.Status != StatusRunning {
					uiState.Executing = false
				} else if uiState.ShownTask == nil {
					uiState.Executing = false
				}
				inputField.SetDisabled(uiState.Executing)
			} else {
				a.TviewApp.SetFocus(uiState.TaskList)
			}
			return nil
		}
		if event.Key() == tcell.KeyCtrlN {
			outputView.Clear()
			uiState.Executing = false
			inputField.SetDisabled(false)
			uiState.ShownTask = nil
			a.TviewApp.SetFocus(inputField)
			return nil
		}
		return event
	})

	outputView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlC {
			if uiState.ShownTask != nil && uiState.ShownTask.Cancel != nil && uiState.ShownTask.Status == StatusRunning {
				_, _ = outputView.Write([]byte("\n[red]正在取消任务...[-]\n"))
				uiState.ShownTask.Cancel()
			} else if uiState.CancelFunc != nil {
				_, _ = outputView.Write([]byte("\n[red]正在取消任务 (旧版)...[-]\n"))
				uiState.CancelFunc()
			}
			return nil
		}
		if event.Key() == tcell.KeyCtrlS {
			a.exportLog(tool.Name(), outputView.GetText(true))
			return nil
		}
		if event.Key() == tcell.KeyCtrlL {
			uiState.UndoBuffer = outputView.GetText(false)
			uiState.UndoRecords = append([]*ExecRecord(nil), uiState.Records...)
			outputView.Clear()
			uiState.Records = nil
			return nil
		}
		if event.Key() == tcell.KeyCtrlU {
			if uiState.UndoBuffer != "" {
				outputView.SetText(uiState.UndoBuffer)
				uiState.Records = uiState.UndoRecords
				uiState.UndoBuffer = ""
				uiState.UndoRecords = nil
			}
			return nil
		}
		return event
	})

	inputField.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlC {
			if uiState.ShownTask != nil && uiState.ShownTask.Cancel != nil && uiState.ShownTask.Status == StatusRunning {
				_, _ = outputView.Write([]byte("\n[red]正在取消任务...[-]\n"))
				uiState.ShownTask.Cancel()
			} else if uiState.CancelFunc != nil {
				_, _ = outputView.Write([]byte("\n[red]正在取消任务 (旧版)...[-]\n"))
				uiState.CancelFunc()
			}
			return nil
		}
		if event.Key() == tcell.KeyCtrlA {
			cmdText := inputField.GetText()
			if err := a.copyToClipboard(cmdText); err == nil {
				a.ShowModal("复制成功", "命令已复制到剪贴板！")
			} else {
				a.ShowModal("复制失败", err.Error())
			}
			return nil
		}
		if event.Key() == tcell.KeyUp {
			if len(history) > 0 && historyIndex < len(history)-1 {
				historyIndex++
				if cmd, ok := history[historyIndex]["cmd"]; ok {
					inputField.SetText(cmd, true)
				}
				if env, ok := history[historyIndex]["env"]; ok {
					envInput.SetText(env)
				}
			}
			return nil
		}
		if event.Key() == tcell.KeyDown {
			if historyIndex > 0 {
				historyIndex--
				if cmd, ok := history[historyIndex]["cmd"]; ok {
					inputField.SetText(cmd, true)
				}
				if env, ok := history[historyIndex]["env"]; ok {
					envInput.SetText(env)
				}
			} else if historyIndex == 0 {
				historyIndex = -1
				inputField.SetText("", true)
			}
			return nil
		}
		if event.Key() == tcell.KeyEnter && event.Modifiers() == tcell.ModNone {
			if uiState.Executing {
				return nil
			}
			cmdText := strings.TrimSpace(inputField.GetText())
			envPath := strings.TrimSpace(envInput.GetText())
			if envPath == "" {
				envPath = "python"
			}

			a.RecordToolUsage(tool.Name(), tool.ID(), map[string]string{
				"cmd": cmdText,
				"env": envPath,
			})
			history = a.Store.GetToolHistory(tool.ID())
			historyIndex = -1

			uiState.UndoBuffer = ""
			uiState.UndoRecords = nil

			record := &ExecRecord{Cmd: cmdText, Env: envPath}
			uiState.Records = append(uiState.Records, record)

			runCtx, cancel := context.WithCancel(context.Background())
			task := &Task{
				ID:        fmt.Sprintf("task_%s_%d", tool.ID(), time.Now().UnixMilli()),
				ToolID:    tool.ID(),
				ToolName:  tool.Name(),
				Cmd:       cmdText,
				Env:       envPath,
				Status:    StatusRunning,
				CreatedAt: time.Now(),
				Cancel:    cancel,
			}

			barState.mu.Lock()
			barState.Tasks = append(barState.Tasks, task)
			barState.ActiveIdx = len(barState.Tasks) - 1
			a.refreshTaskList(uiState, barState)
			visible := barState.Visible
			barState.mu.Unlock()

			if !visible {
				a.showTaskBar(uiState, barState)
				rebuildFocusables()
			}

			uiState.ShownTask = task

			outputView.Clear()
			prefix := fmt.Sprintf("[yellow]❯ 执行环境: %s | 执行参数: %s[-]\n", envPath, cmdText)
			task.Output += prefix
			record.Output += prefix

			a.taskStarted(uiState)
			_, _ = outputView.Write([]byte(prefix))

			go func() {
				defer func() {
					if r := recover(); r != nil {
						a.TviewApp.QueueUpdateDraw(func() {
							task.Status = StatusFailed
							task.EndedAt = time.Now()
							barState.mu.Lock()
							a.refreshTaskList(uiState, barState)
							barState.mu.Unlock()
							a.taskFinished(uiState)
							_, _ = outputView.Write([]byte(fmt.Sprintf("\n[red]任务异常崩溃: %v[-]\n", r)))
							outputView.ScrollToEnd()
						})
					}
				}()
				ot := &outputTracker{
					Writer:    tview.ANSIWriter(outputView),
					Task:      task,
					ShownTask: &uiState.ShownTask,
				}

				err := run(runCtx, envPath, cmdText, ot)

				a.TviewApp.QueueUpdateDraw(func() {
					if err != nil && !ot.wroteBytes {
						barState.mu.Lock()
						a.removeTask(barState, task)
						taskCount := len(barState.Tasks)
						if taskCount > 0 {
							a.refreshTaskList(uiState, barState)
						}
						barState.mu.Unlock()

						if taskCount == 0 {
							a.hideTaskBar(uiState, barState)
							rebuildFocusables()
						}

						errStr := fmt.Sprintf("\n[red]执行出错: %v[-]\n", err)
						_, _ = outputView.Write([]byte(errStr))
						record.Output += errStr
						uiState.ShownTask = nil
						a.taskFinished(uiState)
						a.TviewApp.SetFocus(inputField)
						outputView.ScrollToEnd()
						return
					}

					finalStatus := parseTaskResult(task, err)
					task.Status = finalStatus
					task.EndedAt = time.Now()

					if finalStatus == StatusFailed {
						errStr := fmt.Sprintf("\n[red]执行异常: %v[-]\n", err)
						if err == nil {
							errStr = "\n[red]执行失败（无有效结果）[-]\n"
						}
						_, _ = outputView.Write([]byte(errStr))
						task.Output += errStr
						record.Output += errStr
					} else {
						succStr := "\n[green]执行完成[-]\n"
						_, _ = outputView.Write([]byte(succStr))
						task.Output += succStr
						record.Output += succStr
					}

					barState.mu.Lock()
					a.refreshTaskList(uiState, barState)
					barState.mu.Unlock()

					a.taskFinished(uiState)
					a.TviewApp.SetFocus(inputField)
					outputView.ScrollToEnd()
				})
			}()
			return nil
		}
		if event.Key() == tcell.KeyEnter && event.Modifiers()&tcell.ModShift != 0 {
			return event
		}
		return event
	})

	a.Pages.AddAndSwitchToPage(pageID, flex, true)
	a.TviewApp.SetFocus(inputField)
}

func (a *App) copyToClipboard(text string) error {
	err := clipboard.WriteAll(text)
	if err != nil {
		errStr := err.Error()
		if runtime.GOOS == "linux" && (strings.Contains(errStr, "No clipboard") || strings.Contains(errStr, "exit status")) {
			// 尝试使用 OSC 52 终端转义序列通过 SSH 直接向本地客户端发送剪贴板内容
			b64 := base64.StdEncoding.EncodeToString([]byte(text))
			os.Stdout.WriteString(fmt.Sprintf("\x1b]52;c;%s\x07", b64))
			return fmt.Errorf("服务器无剪贴板。\n已尝试使用 OSC52 终端指令发送至本地剪贴板。\n💡 注: 需确保你的终端软件(如iTerm/Windows Terminal)已开启'允许终端访问剪贴板'功能。")
		}
		return err
	}
	return nil
}

// 导出日志功能
func (a *App) exportLog(toolName, content string) {
	if strings.TrimSpace(content) == "" {
		a.ShowModal("导出提示", "当前终端面板没有任何日志内容可导出。")
		return
	}

	logDir := "my_tools_logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		a.ShowModal("导出错误", "无法创建日志目录: "+err.Error())
		return
	}

	fileName := fmt.Sprintf("%s_%d.log",
		strings.ReplaceAll(toolName, " ", "_"),
		time.Now().UnixMilli())

	exportPath := filepath.Join(logDir, fileName)

	err := os.WriteFile(exportPath, []byte(content), 0644)
	if err != nil {
		a.ShowModal("导出失败", fmt.Sprintf("写入文件失败:\n%v", err))
		return
	}

	if err := a.copyToClipboard(content); err != nil {
		a.ShowModal("导出成功 (剪贴板不可用)", fmt.Sprintf("日志已保存至:\n%s\n\n⚠️ 注意: %v", exportPath, err))
	} else {
		a.ShowModal("导出成功", fmt.Sprintf("日志已保存至:\n%s\n\n✅ 已自动复制到剪贴板！", exportPath))
	}
}
