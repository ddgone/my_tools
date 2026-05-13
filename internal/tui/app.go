package tui

import (
	"fmt"
	"io"
	"strings"

	"my_tools/internal/storage"
	"my_tools/pkg/framework"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type App struct {
	TviewApp *tview.Application
	Pages    *tview.Pages
	Tree     *tview.TreeView
	Store    *storage.Storage

	// context state
	currentTool framework.Tool
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
	a.Tree = tview.NewTreeView()
	a.Tree.SetBorder(true).
		SetTitle(" 🦏 白犀牛工具箱 [gray](↑/↓:移动, Enter:展开/执行, q:退出)[-] ").
		SetTitleColor(tcell.ColorGreen).
		SetBorderColor(tcell.ColorDarkGray)

	a.refreshTree()

	a.Tree.SetSelectedFunc(func(node *tview.TreeNode) {
		ref := node.GetReference()
		if ref == nil {
			node.SetExpanded(!node.IsExpanded())
			return
		}

		tool, ok := ref.(framework.Tool)
		if ok {
			a.executeTool(tool)
		} else {
			// Directory node
			node.SetExpanded(!node.IsExpanded())
		}
	})

	a.Pages.AddAndSwitchToPage("main", a.Tree, true)
}

func (a *App) refreshTree() {
	root := tview.NewTreeNode("所有工具").SetColor(tcell.ColorWhite)

	// --- 添加最近使用目录 ---
	recentTools := a.Store.GetRecentTools()
	if len(recentTools) > 0 {
		recentNode := tview.NewTreeNode(" ⚡ 最近使用 ").
			SetColor(tcell.ColorYellow). // 特殊颜色
			SetSelectable(true).
			SetExpanded(true)

		count := 0
		for _, rt := range recentTools {
			if count >= 3 {
				break
			}

			// 从注册表中查找对应的 Tool
			var targetTool framework.Tool
			for _, t := range framework.Registry {
				if t.ID() == rt.ToolPath {
					targetTool = t
					break
				}
			}

			if targetTool != nil {
				// 显示参数信息
				paramsStr := ""
				if len(rt.LastParams) > 0 {
					var parts []string
					for k, v := range rt.LastParams {
						parts = append(parts, fmt.Sprintf("%s:%s", k, v))
					}
					paramsStr = " [" + strings.Join(parts, ", ") + "]"
				}

				toolNode := tview.NewTreeNode(" 🔧 " + targetTool.Name() + paramsStr).
					SetReference(targetTool).
					SetColor(tcell.ColorPink).
					SetSelectable(true)
				recentNode.AddChild(toolNode)
				count++
			}
		}
		if count > 0 {
			root.AddChild(recentNode)
		}
	}

	// --- 按类别组织工具 ---
	categories := make(map[string]*tview.TreeNode)

	for _, t := range framework.Registry {
		catName := t.Category()
		catNode, exists := categories[catName]
		if !exists {
			catNode = tview.NewTreeNode(" 📁 " + catName).
				SetColor(tcell.ColorTeal).
				SetSelectable(true).
				SetExpanded(true) // 默认展开
			categories[catName] = catNode
			root.AddChild(catNode)
		}

		toolNode := tview.NewTreeNode(" 🔧 " + t.Name()).
			SetReference(t).
			SetColor(tcell.ColorLightGray).
			SetSelectable(true)

		catNode.AddChild(toolNode)
	}

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
		SetBackgroundColor(tcell.ColorDarkSlateGray)
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
	form.SetFieldBackgroundColor(tcell.ColorBlack).
		SetFieldTextColor(tcell.ColorWhite).
		SetLabelColor(tcell.ColorLightBlue).
		SetButtonBackgroundColor(tcell.ColorDarkCyan).
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
		SetBorderColor(tcell.ColorTeal)

	a.Pages.AddAndSwitchToPage("input", form, true)
	a.TviewApp.SetFocus(form)
}

func (a *App) PromptChoice(title, prompt string, options []string, callback func(string)) {
	form := tview.NewForm()
	dropdown := tview.NewDropDown().
		SetLabel(prompt).
		SetOptions(options, nil)

	// 美化表单样式
	form.SetFieldBackgroundColor(tcell.ColorBlack).
		SetFieldTextColor(tcell.ColorWhite).
		SetLabelColor(tcell.ColorLightBlue).
		SetButtonBackgroundColor(tcell.ColorDarkCyan).
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
		SetBorderColor(tcell.ColorTeal)

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
		SetTitleColor(tcell.ColorTeal).
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
		SetTitleColor(tcell.ColorGreen).
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
	inputField.SetBackgroundColor(tcell.ColorBlack)
	inputField.SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorWhite))

	inputField.SetBorder(true).
		SetTitle(" ⌨️ 命令行输入 [gray](输入参数后按 Enter 执行，按 ESC 返回，按 Tab 切换焦点滚动)[-] ").
		SetTitleColor(tcell.ColorYellow).
		SetBorderColor(tcell.ColorTeal)

	flex.AddItem(usageView, 0, 1, false).
		AddItem(outputView, 0, 3, false).
		AddItem(inputField, 5, 1, true) // 给输入框多一点高度 (5行)

	// 焦点高亮效果
	usageView.SetFocusFunc(func() { usageView.SetBorderColor(tcell.ColorYellow) })
	usageView.SetBlurFunc(func() { usageView.SetBorderColor(tcell.ColorDarkGray) })
	outputView.SetFocusFunc(func() { outputView.SetBorderColor(tcell.ColorYellow) })
	outputView.SetBlurFunc(func() { outputView.SetBorderColor(tcell.ColorDarkGray) })
	inputField.SetFocusFunc(func() { inputField.SetBorderColor(tcell.ColorYellow) })
	inputField.SetBlurFunc(func() { inputField.SetBorderColor(tcell.ColorTeal) })

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
