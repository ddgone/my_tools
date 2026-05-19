package tui

import (
	"my_tools/pkg/framework"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type settingsTool struct {
	app *App
}

func (s *settingsTool) ID() string       { return "sys_settings" }
func (s *settingsTool) Name() string     { return "系统设置" }
func (s *settingsTool) Category() string { return "⚙️ 系统配置" }

func (s *settingsTool) Execute(ctx framework.AppContext) {
	// 记录进入设置前的初始状态，用于 ESC 取消时恢复
	initialMode := s.app.Store.GetShowVerboseShortcuts()

	form := tview.NewForm().
		SetFieldBackgroundColor(colorBgDark).
		SetFieldTextColor(tcell.ColorWhite).
		SetLabelColor(tcell.ColorOrange).
		SetButtonBackgroundColor(tcell.ColorDarkGreen).
		SetButtonTextColor(tcell.ColorWhite)

	form.SetBorder(true).
		SetTitle(" ⚙️ 全局系统首选项 [gray](ESC: 取消并返回)[-] ").
		SetTitleColor(tcell.ColorSilver).
		SetBorderColor(tcell.ColorDarkGray)

	options := []string{
		"精简模式 (推荐，仅按 F1 呼出面板)",
		"详细模式 (每个面板标题栏显示详细快捷键)",
	}

	initialIndex := 0
	if initialMode {
		initialIndex = 1
	}

	form.AddDropDown("快捷键提示模式", options, initialIndex, func(option string, optionIndex int) {
		if optionIndex == 0 {
			s.app.Store.SetShowVerboseShortcuts(false)
		} else {
			s.app.Store.SetShowVerboseShortcuts(true)
		}
	})

	// 限制下拉框的宽度，防止全屏时被无限拉伸
	if dd, ok := form.GetFormItemByLabel("快捷键提示模式").(*tview.DropDown); ok {
		dd.SetFieldWidth(45)
	}

	form.AddButton("保存并返回", func() {
		// 返回主页并刷新整个界面以应用更改
		s.app.Pages.RemovePage(s.ID())
		s.app.setupUI()
		s.app.UpdateAllPanelTitles() // 刷新那些已经被缓存起来的面板的标题

		// 恢复光标到系统设置节点
		if found := s.app.findNodeByToolID(s.ID()); found != nil {
			s.app.expandParents(found)
			s.app.Tree.SetCurrentNode(found)
		}
		s.app.TviewApp.SetFocus(s.app.Tree)

		go func() {
			time.Sleep(20 * time.Millisecond)
			s.app.TviewApp.QueueUpdateDraw(func() {
				if found := s.app.findNodeByToolID(s.ID()); found != nil {
					s.app.Tree.SetCurrentNode(found)
				}
				s.app.TviewApp.SetFocus(s.app.Tree)
			})
		}()
	})

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		focus := s.app.TviewApp.GetFocus()
		// 如果当前焦点是 List，说明 DropDown 正在展开状态！
		// 此时把控制权完全交给 tview 默认行为（上下切换选项、Enter确认、ESC取消展开）
		if _, isList := focus.(*tview.List); isList {
			return event
		}

		// --- 下面的逻辑仅在 DropDown 收起，或者焦点在其他表单项时生效 ---

		// 拦截上下键，将其转化为 Tab/Shift+Tab 用于在设置项和按钮之间穿梭
		if event.Key() == tcell.KeyDown {
			return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
		}
		if event.Key() == tcell.KeyUp {
			return tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone)
		}

		// ESC 彻底取消修改，恢复初始状态并退出设置页
		if event.Key() == tcell.KeyEscape {
			s.app.Store.SetShowVerboseShortcuts(initialMode)
			s.app.Pages.RemovePage(s.ID())
			s.app.setupUI()
			s.app.UpdateAllPanelTitles() // 取消时也可能需要刷新标题（虽然理论上不需要，但安全起见）

			// 恢复光标到系统设置节点
			if found := s.app.findNodeByToolID(s.ID()); found != nil {
				s.app.expandParents(found)
				s.app.Tree.SetCurrentNode(found)
			}
			s.app.TviewApp.SetFocus(s.app.Tree)

			go func() {
				time.Sleep(20 * time.Millisecond)
				s.app.TviewApp.QueueUpdateDraw(func() {
					if found := s.app.findNodeByToolID(s.ID()); found != nil {
						s.app.Tree.SetCurrentNode(found)
					}
					s.app.TviewApp.SetFocus(s.app.Tree)
				})
			}()

			return nil
		}
		return event
	})

	// 采用全屏直接显示，不再嵌套多余的空白 Flex
	s.app.Pages.AddAndSwitchToPage(s.ID(), form, true)
}
