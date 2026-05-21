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
	bgm        *bgmPlayer
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

	a.syncBGM()

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
					if a.Store.GetConfirmExit() {
						a.showConfirmExitModal()
					} else {
						a.TviewApp.Stop()
					}
					return nil
				}
			}
		}
		// Ctrl+F 收藏/取消收藏当前工具
		if event.Key() == tcell.KeyCtrlF {
			if _, isInput := a.TviewApp.GetFocus().(*tview.InputField); !isInput {
				if _, isTextArea := a.TviewApp.GetFocus().(*tview.TextArea); !isTextArea {
					a.toggleFavorite()
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
	for _, ui := range a.TermUI {
		if ui.Output != nil {
			ui.Output.SetWordWrap(a.Store.GetAutoWordWrap())
		}
		if ui.Usage != nil {
			isPy := strings.Contains(ui.Usage.GetTitle(), "Python专属")
			if isPy {
				ui.Usage.SetTitle(a.getTitleWithShortcut("📖 使用说明 (Python专属)", "Ctrl+E:全屏/还原", true))
			} else {
				ui.Usage.SetTitle(a.getTitleWithShortcut("📖 使用说明", "Ctrl+E:全屏/还原", true))
			}
		}
		if ui.Output != nil {
			ui.Output.SetTitle(a.getTitleWithShortcut("📺 终端输出", "Ctrl+L:清空, Ctrl+U:撤销清空, Ctrl+S:导出, Ctrl+C:取消, Ctrl+E:全屏", false))
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
		SetTitle(a.getTitleWithShortcut("🦎 火蜥蜴工具箱", "←/→/↑/↓:导航, Enter:执行, Ctrl+P:搜索, Ctrl+F:收藏, r:重置, b:折叠, q:退出", true)).
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

// showInModal 将一个 Primitive 包装在带有纯色背景遮罩的居中布局中并显示
func (a *App) showInModal(p tview.Primitive, width, height int, pageID string) {
	// 使用纯背景色的 Grid 覆盖整个屏幕作为遮罩，彻底遮挡底层内容
	// 注意：必须替换 Grid 默认的透明 Box（dontClear=true）为实心背景 Box
	grid := tview.NewGrid()
	grid.Box = tview.NewBox().SetBackgroundColor(colorBg)

	if width > 0 && height > 0 {
		// 如果指定了宽高，则使用 3x3 Grid 进行居中
		inner := tview.NewGrid().
			SetColumns(0, width, 0).
			SetRows(0, height, 0).
			AddItem(p, 1, 1, 1, 1, 0, 0, true)
		grid.AddItem(inner, 0, 0, 1, 1, 0, 0, true)
	} else {
		// 如果未指定宽高（如 tview.Modal），则直接添加，让其自行处理居中
		grid.AddItem(p, 0, 0, 1, 1, 0, 0, true)
	}

	a.Pages.AddAndSwitchToPage(pageID, grid, true)
	a.TviewApp.SetFocus(p)
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

func (a *App) getToolTaskStatus(toolID string) (runningCount int, hasSuccess bool, hasFailed bool, hasUnviewed bool) {
	bar, ok := a.TaskBars[toolID]
	if !ok || len(bar.Tasks) == 0 {
		return 0, false, false, false
	}

	bar.mu.Lock()
	defer bar.mu.Unlock()

	for _, t := range bar.Tasks {
		if t.Status == StatusRunning {
			runningCount++
		} else if t.Status == StatusSuccess {
			hasSuccess = true
		} else if t.Status == StatusFailed {
			hasFailed = true
		}
	}

	hasUnviewed = !bar.ResultsViewed
	return
}

func (a *App) formatToolNode(node *tview.TreeNode, t framework.Tool, baseIcon string, baseColor tcell.Color, isStatusCenter bool) {
	runningCount, hasSuccess, hasFailed, hasUnviewed := a.getToolTaskStatus(t.ID())

	label := fmt.Sprintf(" %s %s", baseIcon, t.Name())
	color := baseColor

	if isStatusCenter {
		// 状态中心模式：整行变色，强调状态
		if runningCount > 0 {
			label = fmt.Sprintf(" %s %s (%d) [运行中]", baseIcon, t.Name(), runningCount)
			color = tcell.ColorYellow
		} else if hasUnviewed {
			if hasFailed {
				label = fmt.Sprintf(" %s %s [失败]", baseIcon, t.Name())
				color = tcell.ColorRed
			} else if hasSuccess {
				label = fmt.Sprintf(" %s %s [完成]", baseIcon, t.Name())
				color = tcell.ColorGreen
			}
		}
	} else {
		// 普通入口模式：保持原色（身份优先），仅添加轻量后缀
		if runningCount > 0 {
			label = fmt.Sprintf(" %s %s (%d) [运行中]", baseIcon, t.Name(), runningCount)
			// 不改颜色，让它保持 Go蓝/Py绿
		} else if hasUnviewed {
			if hasFailed {
				label = fmt.Sprintf(" %s %s ●", baseIcon, t.Name()) // 使用一个小圆点标识有未读错误
			} else if hasSuccess {
				label = fmt.Sprintf(" %s %s ●", baseIcon, t.Name()) // 使用一个小圆点标识有未读成功
			}
		}
	}

	node.SetText(label).SetColor(color).SetReference(t)
}

func (a *App) refreshTree() {
	root := tview.NewTreeNode("🦎 火蜥蜴工具箱").SetColor(tcell.ColorWhite).SetSelectable(false)

	// --- 0. 正在运行 ---
	runningNode := tview.NewTreeNode(" 🏃 运行任务 ").
		SetColor(tcell.ColorYellow).
		SetSelectable(true).
		SetExpanded(a.Store.GetNodeState("🏃 运行任务", true))

	hasAnyTasks := false
	// 我们遍历注册表，寻找所有有任务（运行中或未读）的工具
	for _, t := range framework.Registry {
		runningCount, _, _, hasUnviewed := a.getToolTaskStatus(t.ID())
		if runningCount > 0 || hasUnviewed {
			hasAnyTasks = true
			toolColor := colorGo
			icon := "🔧"
			if strings.Contains(t.Category(), "Python") {
				toolColor = colorPy
				icon = "📄"
			}
			toolNode := tview.NewTreeNode("")
			a.formatToolNode(toolNode, t, icon, toolColor, true) // 状态中心使用 true
			runningNode.AddChild(toolNode)
		}
	}

	if hasAnyTasks {
		root.AddChild(runningNode)
		root.AddChild(tview.NewTreeNode("").SetSelectable(false)) // 空行分隔
	}

	// --- 1. 收藏夹 ---
	favorites := a.Store.GetFavorites()
	favNode := tview.NewTreeNode(" ❤️ 收藏夹 ").
		SetColor(tcell.ColorPink).
		SetSelectable(true).
		SetExpanded(a.Store.GetNodeState("❤️ 收藏夹", true))

	if len(favorites) > 0 {
		for _, favID := range favorites {
			var favTool framework.Tool
			for _, t := range framework.Registry {
				if t.ID() == favID {
					favTool = t
					break
				}
			}
			if favTool != nil {
				toolColor := colorGo
				icon := "🔧"
				if strings.Contains(favTool.Category(), "Python") {
					toolColor = colorPy
					icon = "📄"
				}
				toolNode := tview.NewTreeNode("")
				a.formatToolNode(toolNode, favTool, icon, toolColor, false) // 非状态中心使用 false
				favNode.AddChild(toolNode)
			}
		}
	} else {
		emptyNode := tview.NewTreeNode(" (暂无收藏，Ctrl+F 收藏工具) ").SetColor(tcell.ColorGray).SetSelectable(false)
		favNode.AddChild(emptyNode)
	}
	root.AddChild(favNode)
	root.AddChild(tview.NewTreeNode("").SetSelectable(false)) // 空行分隔

	// --- 2. 最近使用 ---
	recentTools := a.Store.GetRecentTools()
	recentNode := tview.NewTreeNode(" ⭐ 最近使用 ").
		SetColor(tcell.ColorYellow).
		SetSelectable(true).
		SetExpanded(a.Store.GetNodeState("⭐ 最近使用", true))

	recentMaxCount := a.Store.GetRecentToolsCount()
	if len(recentTools) > 0 {
		count := 0
		for _, rt := range recentTools {
			if count >= recentMaxCount {
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
				// 判断工具类型给予不同的颜色
				toolColor := colorGo
				icon := "🔧"
				if strings.Contains(targetTool.Category(), "Python") {
					toolColor = colorPy
					icon = "📄"
				}

				toolNode := tview.NewTreeNode("")
				a.formatToolNode(toolNode, targetTool, icon, toolColor, false) // 非状态中心使用 false

				// 最近使用的特殊逻辑：保留参数后缀
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
				toolNode.SetText(toolNode.GetText() + paramsStr)

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

	// --- 3. 内置原生应用 ---
	nativeNode := tview.NewTreeNode(" 🚀 内置原生应用 ").
		SetColor(tcell.ColorSalmon).
		SetSelectable(true).
		SetExpanded(a.Store.GetNodeState("🚀 内置原生应用", true))

	// --- 4. 扩展兼容脚本 ---
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
		toolNode := tview.NewTreeNode("")
		a.formatToolNode(toolNode, t, icon, color, true) // 原始分类也作为状态中心，方便在长列表中定位

		currentParent.AddChild(toolNode)
	}

	root.AddChild(nativeNode)
	root.AddChild(tview.NewTreeNode("").SetSelectable(false)) // 空行分隔
	root.AddChild(scriptNode)
	root.AddChild(tview.NewTreeNode("").SetSelectable(false)) // 空行分隔

	// --- 5. 终端系统 ---
	sysTermNode := tview.NewTreeNode(" 💻 系统与设置 ").
		SetColor(tcell.ColorSkyblue).
		SetSelectable(true).
		SetExpanded(a.Store.GetNodeState("💻 系统与设置", true))

	termTool := &sysTerminalTool{}
	termItem := tview.NewTreeNode("")
	a.formatToolNode(termItem, termTool, "📟", tcell.ColorLightCyan, false) // 终端系统不作为状态中心
	sysTermNode.AddChild(termItem)
	root.AddChild(sysTermNode)

	root.AddChild(tview.NewTreeNode("").SetSelectable(false)) // 空行分隔

	// --- 6. 系统设置 ---
	setTool := &settingsTool{app: a}
	setNode := tview.NewTreeNode(" ⚙️ 系统首选项 ").
		SetReference(setTool).
		SetColor(tcell.ColorSilver).
		SetSelectable(true)
	sysTermNode.AddChild(setNode) // 添加到 💻 系统与设置 节点下

	a.Tree.SetRoot(root)

	// 尝试恢复之前的选择
	if a.lastToolID != "" {
		if found := a.findNodeByToolID(a.lastToolID); found != nil {
			a.Tree.SetCurrentNode(found)
		} else {
			a.Tree.SetCurrentNode(root)
		}
	} else {
		a.Tree.SetCurrentNode(root)
	}

	if a.Store.GetAutoExpandAll() {
		root.Walk(func(node, parent *tview.TreeNode) bool {
			if len(node.GetChildren()) > 0 {
				node.SetExpanded(true)
			}
			return true
		})
	}
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

	// 生成一个唯一的 modal ID，防止多个弹窗覆盖同一个 page
	modalID := fmt.Sprintf("modal_%d", time.Now().UnixNano())

	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"确定"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			a.Pages.RemovePage(modalID)
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

	a.showInModal(modal, 0, 0, modalID)
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

	a.showInModal(form, 50, 7, "input")
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

	a.showInModal(form, 50, 7, "choice")
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

	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			a.Pages.RemovePage("palette")
			a.Pages.SwitchToPage("main")
			a.TviewApp.SetFocus(a.Tree)
			return nil
		}
		return event
	})

	a.showInModal(flex, 80, 25, "palette")
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

	// 标记结果已读
	if bar, ok := a.TaskBars[tool.ID()]; ok {
		bar.mu.Lock()
		bar.ResultsViewed = true
		bar.mu.Unlock()
		a.refreshTree()
	}

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
		SetScrollable(true).
		SetWordWrap(a.Store.GetAutoWordWrap())
	outputView.SetChangedFunc(func() {
		outputView.ScrollToEnd()
		a.TviewApp.Draw()
	})
	uiState.Output = outputView

	outputView.SetBorder(true).
		SetTitle(a.getTitleWithShortcut("📺 终端输出", "Ctrl+L:清空, Ctrl+U:撤销清空, Ctrl+S:导出, Ctrl+C:取消, Ctrl+E:全屏", false)).
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
			if uiState.TaskList == nil || len(barState.Tasks) == 0 {
				return nil
			}
			if barState.Visible {
				a.hideTaskBar(uiState, barState)
				rebuildFocusables()
				if uiState.TaskList.HasFocus() {
					a.TviewApp.SetFocus(inputField)
				}
			} else {
				a.showTaskBar(uiState, barState)
				rebuildFocusables()
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
			if barState != nil {
				barState.mu.Lock()
				hasRunning := false
				for _, t := range barState.Tasks {
					if t.Status == StatusRunning {
						hasRunning = true
						break
					}
				}
				if hasRunning {
					barState.mu.Unlock()
					return nil // 如果有运行中的任务，则不清理
				}

				if len(barState.Tasks) > 0 {
					barState.UndoTasks = append([]*Task(nil), barState.Tasks...)
					barState.UndoActiveIdx = barState.ActiveIdx
					barState.Tasks = nil
					barState.ActiveIdx = -1
					barState.ResultsViewed = true
				}
				barState.mu.Unlock()

				a.refreshTree() // 清理任务后刷新树

				if len(barState.UndoTasks) > 0 {
					a.hideTaskBar(uiState, barState)
					rebuildFocusables()
					uiState.ShownTask = nil
				}
			} else if uiState.Executing {
				return nil
			}

			uiState.UndoBuffer = outputView.GetText(false)
			uiState.UndoRecords = append([]*ExecRecord(nil), uiState.Records...)
			outputView.Clear()
			uiState.Records = nil
			return nil
		}
		if event.Key() == tcell.KeyCtrlU {
			if barState != nil {
				barState.mu.Lock()
				if len(barState.UndoTasks) > 0 {
					barState.Tasks = append([]*Task(nil), barState.UndoTasks...)
					barState.ActiveIdx = barState.UndoActiveIdx
					barState.UndoTasks = nil
					barState.ResultsViewed = false // 恢复的任务视为未读
					barState.mu.Unlock()

					a.showTaskBar(uiState, barState)
					a.refreshTaskList(uiState, barState)
					rebuildFocusables()
					a.refreshTree()

					if barState.ActiveIdx >= 0 && barState.ActiveIdx < len(barState.Tasks) {
						uiState.ShownTask = barState.Tasks[barState.ActiveIdx]
					}
				} else {
					barState.mu.Unlock()
				}
			}

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
			osc52, err := a.copyToClipboard(cmdText)
			if err == nil {
				if osc52 {
					a.ShowModal("已通过终端协议发送", "已使用 OSC52 协议将命令提交至本地终端\n请确认你的终端软件(如 iTerm2 / WezTerm)已开启剪贴板访问权限")
				} else {
					a.ShowModal("复制成功", "命令已复制到剪贴板！")
				}
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
			barState.ResultsViewed = true // 在工具页面内执行，视为已读
			a.refreshTaskList(uiState, barState)
			visible := barState.Visible
			barState.mu.Unlock()

			a.refreshTree()

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
						outputView.ScrollToEnd()
						a.maybeNotifyTaskComplete(tool.ID(), cmdText, StatusFailed, task)
						return
					}

					finalStatus := parseTaskResult(task, err)
					task.Status = finalStatus
					task.EndedAt = time.Now()

					elapsedStr := fmt.Sprintf("[gray]⏱ 耗时: %s[-]\n", task.EndedAt.Sub(task.CreatedAt).Truncate(time.Millisecond).String())

					if finalStatus == StatusFailed {
						errStr := fmt.Sprintf("\n[red]执行异常: %v[-]\n", err)
						if err == nil {
							errStr = "\n[red]执行失败（无有效结果）[-]\n"
						}
						errStr += elapsedStr
						_, _ = outputView.Write([]byte(errStr))
						task.Output += errStr
						record.Output += errStr
					} else {
						succStr := "\n[green]执行完成[-]\n" + elapsedStr
						_, _ = outputView.Write([]byte(succStr))
						task.Output += succStr
						record.Output += succStr
					}

					barState.mu.Lock()
					frontPage, _ := a.Pages.GetFrontPage()
					isCurrentToolPage := frontPage == "term_"+tool.ID()
					if !isCurrentToolPage {
						barState.ResultsViewed = false
					} else {
						barState.ResultsViewed = true
					}
					a.refreshTaskList(uiState, barState)
					barState.mu.Unlock()

					a.taskFinished(uiState)
					a.refreshTree()
					outputView.ScrollToEnd()
					a.maybeNotifyTaskComplete(tool.ID(), cmdText, finalStatus, task)
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

	// 标记结果已读
	if bar, ok := a.TaskBars[tool.ID()]; ok {
		bar.mu.Lock()
		bar.ResultsViewed = true
		bar.mu.Unlock()
		a.refreshTree()
	}

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
		SetScrollable(true).
		SetWordWrap(a.Store.GetAutoWordWrap())
	outputView.SetChangedFunc(func() {
		outputView.ScrollToEnd()
		a.TviewApp.Draw()
	})
	uiState.Output = outputView

	outputView.SetBorder(true).
		SetTitle(a.getTitleWithShortcut("📺 终端输出", "Ctrl+L:清空, Ctrl+U:撤销清空, Ctrl+S:导出, Ctrl+C:取消, Ctrl+E:全屏", false)).
		SetTitleColor(tcell.ColorYellow).
		SetBorderColor(tcell.ColorDarkGray)

	outputRow := tview.NewFlex().SetDirection(tview.FlexColumn)
	outputRow.AddItem(outputView, 0, 1, false)
	uiState.OutputRow = outputRow

	initialCmd := ""
	initialEnv := a.Store.GetDefaultPythonPath()
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
			if uiState.TaskList == nil || len(barState.Tasks) == 0 {
				return nil
			}
			if barState.Visible {
				a.hideTaskBar(uiState, barState)
				rebuildFocusables()
				if uiState.TaskList.HasFocus() {
					a.TviewApp.SetFocus(inputField)
				}
			} else {
				a.showTaskBar(uiState, barState)
				rebuildFocusables()
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
			if barState != nil {
				barState.mu.Lock()
				hasRunning := false
				for _, t := range barState.Tasks {
					if t.Status == StatusRunning {
						hasRunning = true
						break
					}
				}
				if hasRunning {
					barState.mu.Unlock()
					return nil // 如果有运行中的任务，则不清理
				}

				if len(barState.Tasks) > 0 {
					barState.UndoTasks = append([]*Task(nil), barState.Tasks...)
					barState.UndoActiveIdx = barState.ActiveIdx
					barState.Tasks = nil
					barState.ActiveIdx = -1
					barState.ResultsViewed = true
				}
				barState.mu.Unlock()

				a.refreshTree() // 清理任务后刷新树

				if len(barState.UndoTasks) > 0 {
					a.hideTaskBar(uiState, barState)
					rebuildFocusables()
					uiState.ShownTask = nil
				}
			} else if uiState.Executing {
				return nil
			}

			uiState.UndoBuffer = outputView.GetText(false)
			uiState.UndoRecords = append([]*ExecRecord(nil), uiState.Records...)
			outputView.Clear()
			uiState.Records = nil
			return nil
		}
		if event.Key() == tcell.KeyCtrlU {
			if barState != nil {
				barState.mu.Lock()
				if len(barState.UndoTasks) > 0 {
					barState.Tasks = append([]*Task(nil), barState.UndoTasks...)
					barState.ActiveIdx = barState.UndoActiveIdx
					barState.UndoTasks = nil
					barState.ResultsViewed = false // 恢复的任务视为未读
					barState.mu.Unlock()

					a.showTaskBar(uiState, barState)
					a.refreshTaskList(uiState, barState)
					rebuildFocusables()
					a.refreshTree()

					if barState.ActiveIdx >= 0 && barState.ActiveIdx < len(barState.Tasks) {
						uiState.ShownTask = barState.Tasks[barState.ActiveIdx]
					}
				} else {
					barState.mu.Unlock()
				}
			}

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
			osc52, err := a.copyToClipboard(cmdText)
			if err == nil {
				if osc52 {
					a.ShowModal("已通过终端协议发送", "已使用 OSC52 协议将命令提交至本地终端\n请确认你的终端软件(如 iTerm2 / WezTerm)已开启剪贴板访问权限")
				} else {
					a.ShowModal("复制成功", "命令已复制到剪贴板！")
				}
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
			barState.ResultsViewed = true // 在工具页面内执行，视为已读
			a.refreshTaskList(uiState, barState)
			visible := barState.Visible
			barState.mu.Unlock()

			a.refreshTree()

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
						a.maybeNotifyTaskComplete(tool.ID(), cmdText, StatusFailed, task)
						return
					}

					finalStatus := parseTaskResult(task, err)
					task.Status = finalStatus
					task.EndedAt = time.Now()

					elapsedStr := fmt.Sprintf("[gray]⏱ 耗时: %s[-]\n", task.EndedAt.Sub(task.CreatedAt).Truncate(time.Millisecond).String())

					if finalStatus == StatusFailed {
						errStr := fmt.Sprintf("\n[red]执行异常: %v[-]\n", err)
						if err == nil {
							errStr = "\n[red]执行失败（无有效结果）[-]\n"
						}
						errStr += elapsedStr
						_, _ = outputView.Write([]byte(errStr))
						task.Output += errStr
						record.Output += errStr
					} else {
						succStr := "\n[green]执行完成[-]\n" + elapsedStr
						_, _ = outputView.Write([]byte(succStr))
						task.Output += succStr
						record.Output += succStr
					}

					barState.mu.Lock()
					frontPage, _ := a.Pages.GetFrontPage()
					isCurrentToolPage := frontPage == "term_"+tool.ID()
					if !isCurrentToolPage {
						barState.ResultsViewed = false
					} else {
						barState.ResultsViewed = true
					}
					a.refreshTaskList(uiState, barState)
					barState.mu.Unlock()

					a.taskFinished(uiState)
					a.refreshTree()
					outputView.ScrollToEnd()
					a.maybeNotifyTaskComplete(tool.ID(), cmdText, finalStatus, task)
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

func (a *App) showConfirmExitModal() {
	frontPage, _ := a.Pages.GetFrontPage()
	focus := a.TviewApp.GetFocus()

	modal := tview.NewModal().
		SetText("确定要退出火蜥蜴工具箱吗？").
		AddButtons([]string{"确定退出", "取消"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			a.Pages.RemovePage("confirm_exit_modal")
			if frontPage != "" {
				a.Pages.SwitchToPage(frontPage)
			}
			if focus != nil {
				a.TviewApp.SetFocus(focus)
			}
			if buttonLabel == "确定退出" {
				a.TviewApp.Stop()
			}
		})
	modal.SetBorder(true).
		SetTitle(" 🚪 退出确认 ").
		SetTitleColor(tcell.ColorRed).
		SetBackgroundColor(colorBgLight)

	a.showInModal(modal, 0, 0, "confirm_exit_modal")
}

func (a *App) toggleFavorite() {
	currentNode := a.Tree.GetCurrentNode()
	if currentNode == nil {
		return
	}
	ref := currentNode.GetReference()
	if ref == nil {
		return
	}
	tool, ok := ref.(framework.Tool)
	if !ok {
		return
	}

	toolID := tool.ID()
	if a.Store.IsFavorite(toolID) {
		a.Store.RemoveFavorite(toolID)
	} else {
		a.Store.AddFavorite(toolID)
	}

	a.refreshTree()
	if found := a.findNodeByToolID(toolID); found != nil {
		a.expandParents(found)
		a.Tree.SetCurrentNode(found)
	}
	a.TviewApp.SetFocus(a.Tree)
	go func() {
		time.Sleep(20 * time.Millisecond)
		a.TviewApp.QueueUpdateDraw(func() {
			if found := a.findNodeByToolID(toolID); found != nil {
				a.Tree.SetCurrentNode(found)
			}
			a.TviewApp.SetFocus(a.Tree)
		})
	}()
}

func (a *App) maybeNotifyTaskComplete(toolID, cmdText string, status TaskStatus, task *Task) {
	frontPage, _ := a.Pages.GetFrontPage()
	pageID := "term_" + toolID

	if frontPage != pageID && frontPage != "main" {
		statusText := "✅ 完成"
		if status == StatusFailed {
			statusText = "❌ 失败"
		}
		msg := fmt.Sprintf("后台任务 %s\n\n工具: %s\n参数: %s", statusText, task.ToolName, cmdText)
		a.ShowModal("🔔 任务通知", msg)
	} else if frontPage == "main" {
		statusText := "完成"
		if status == StatusFailed {
			statusText = "失败"
		}
		msg := fmt.Sprintf("后台任务 %s: %s", statusText, task.ToolName)
		a.ShowModal("🔔 任务通知", msg)
	}
}

func (a *App) copyToClipboard(text string) (osc52Used bool, err error) {
	err = clipboard.WriteAll(text)
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "No clipboard") || strings.Contains(errStr, "exit status") {
			b64 := base64.StdEncoding.EncodeToString([]byte(text))
			os.Stdout.WriteString(fmt.Sprintf("\x1b]52;c;%s\x07", b64))
			return true, nil
		}
		return false, err
	}
	return false, nil
}

func (a *App) pasteFromClipboard() (text string, osc52Used bool, err error) {
	text, err = clipboard.ReadAll()
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "No clipboard") || strings.Contains(errStr, "exit status") || strings.Contains(errStr, "exec:") {
			return a.tryOSC52Read()
		}
		return "", false, err
	}
	return text, false, nil
}

func (a *App) tryOSC52Read() (text string, osc52Used bool, err error) {
	os.Stdout.WriteString("\x1b]52;c;?\x07")

	result := make(chan string, 1)
	errCh := make(chan error, 1)

	go func() {
		f, openErr := os.OpenFile("/dev/tty", os.O_RDONLY, 0)
		if openErr != nil {
			f = os.Stdin
		}
		defer f.Close()

		var buf [4096]byte
		n, readErr := f.Read(buf[:])
		if readErr != nil {
			errCh <- readErr
			return
		}

		s := string(buf[:n])
		if idx := strings.Index(s, "\x1b]52;c;"); idx >= 0 {
			rest := s[idx+len("\x1b]52;c;"):]
			if endIdx := strings.IndexByte(rest, '\x07'); endIdx >= 0 {
				result <- rest[:endIdx]
				return
			}
			if endIdx := strings.Index(rest, "\x1b\\"); endIdx >= 0 {
				result <- rest[:endIdx]
				return
			}
		}
		errCh <- fmt.Errorf("未收到有效的 OSC52 响应")
	}()

	select {
	case b64 := <-result:
		decoded, decErr := base64.StdEncoding.DecodeString(b64)
		if decErr != nil {
			return "", true, fmt.Errorf("OSC52 返回数据解码失败: %v\n请将焦点移到配置编辑区后按 Ctrl+V 手动粘贴", decErr)
		}
		return string(decoded), true, nil
	case readErr := <-errCh:
		return "", true, fmt.Errorf("OSC52 查询失败: %v\n请将焦点移到配置编辑区后按 Ctrl+V 手动粘贴", readErr)
	case <-time.After(2 * time.Second):
		return "", true, fmt.Errorf("OSC52 查询超时，终端可能不支持此功能\n请将焦点移到配置编辑区后按 Ctrl+V 手动粘贴")
	}
}

// 导出日志功能
func (a *App) exportLog(toolName, content string) {
	if strings.TrimSpace(content) == "" {
		a.ShowModal("导出提示", "当前终端面板没有任何日志内容可导出。")
		return
	}

	logDir := a.Store.GetLogExportDir()
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

	osc52, err := a.copyToClipboard(content)
	if err != nil {
		a.ShowModal("导出成功 (剪贴板不可用)", fmt.Sprintf("日志已保存至:\n%s\n\n⚠️ 复制失败: %v", exportPath, err))
	} else if osc52 {
		a.ShowModal("导出成功", fmt.Sprintf("日志已保存至:\n%s\n\n✅ 已通过 OSC52 协议发送至本地终端\n请确认终端已开启剪贴板访问权限", exportPath))
	} else {
		a.ShowModal("导出成功", fmt.Sprintf("日志已保存至:\n%s\n\n✅ 已自动复制到剪贴板！", exportPath))
	}
}

func (a *App) syncBGM() {
	enabled := a.Store.GetBGMEnabled()

	if enabled {
		if a.bgm == nil {
			p, err := newBGMPlayer()
			if err != nil {
				return
			}
			a.bgm = p
		}
	} else {
		if a.bgm != nil {
			a.bgm.stop()
			a.bgm = nil
		}
	}
}
