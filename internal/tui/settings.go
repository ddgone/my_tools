package tui

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"my_tools/pkg/framework"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type settingsTool struct {
	app *App
}

func (s *settingsTool) ID() string       { return "sys_settings" }
func (s *settingsTool) Name() string     { return "系统设置" }
func (s *settingsTool) Category() string { return "⚙️ 系统配置" }

func showImportExportModal(s *settingsTool, form *tview.Form) {
	modal := tview.NewForm().
		SetFieldBackgroundColor(colorBgDark).
		SetFieldTextColor(tcell.ColorWhite).
		SetLabelColor(tcell.ColorOrange).
		SetButtonBackgroundColor(tcell.ColorDarkGreen).
		SetButtonTextColor(tcell.ColorWhite)

	modal.SetBorder(true).
		SetTitle(" 📦 导入/导出配置 [gray](ESC: 取消)[-] ").
		SetTitleColor(tcell.ColorSilver).
		SetBorderColor(tcell.ColorDarkGray)

	dd := form.GetFormItemByLabel("快捷键提示模式").(*tview.DropDown)
	currentIndex, _ := dd.GetCurrentOption()
	currentModeFromForm := currentIndex == 1

	settings := struct {
		ShowVerboseShortcuts bool `json:"show_verbose_shortcuts"`
	}{
		ShowVerboseShortcuts: currentModeFromForm,
	}

	b, _ := json.Marshal(settings)
	encoded := base64.StdEncoding.EncodeToString(b)

	inputField := tview.NewInputField().
		SetLabel("配置数据: ").
		SetText(encoded).
		SetFieldWidth(50)

	modal.AddFormItem(inputField)

	modal.AddButton("复制到剪贴板", func() {
		text := inputField.GetText()
		if err := s.app.copyToClipboard(text); err == nil {
			s.app.ShowModal("复制成功", "配置已复制到剪贴板！\n你可以将它发送给其他人。")
		} else {
			s.app.ShowModal("复制失败", err.Error())
		}
	})

	modal.AddButton("保存并应用", func() {
		text := strings.TrimSpace(inputField.GetText())
		decoded, err := base64.StdEncoding.DecodeString(text)
		if err != nil {
			s.app.ShowModal("格式错误", "无法解析配置字符串，请确保粘贴了正确的内容。")
			return
		}
		var newSettings struct {
			ShowVerboseShortcuts bool `json:"show_verbose_shortcuts"`
		}
		if err := json.Unmarshal(decoded, &newSettings); err != nil {
			s.app.ShowModal("解析错误", "配置字符串内容无效。")
			return
		}

		if newSettings.ShowVerboseShortcuts {
			dd.SetCurrentOption(1)
		} else {
			dd.SetCurrentOption(0)
		}

		s.app.Pages.RemovePage("import_export_modal")
		s.app.Pages.SwitchToPage(s.ID()) // Ensure settings page becomes visible again
		s.app.TviewApp.SetFocus(form)
	})

	modal.AddButton("取消", func() {
		s.app.Pages.RemovePage("import_export_modal")
		s.app.Pages.SwitchToPage(s.ID()) // Ensure settings page becomes visible again
		s.app.TviewApp.SetFocus(form)
	})

	modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			s.app.Pages.RemovePage("import_export_modal")
			s.app.Pages.SwitchToPage(s.ID()) // Ensure settings page becomes visible again
			s.app.TviewApp.SetFocus(form)
			return nil
		}
		if event.Key() == tcell.KeyDown {
			return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
		}
		if event.Key() == tcell.KeyUp {
			return tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone)
		}
		return event
	})

	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(modal, 13, 1, true).
			AddItem(nil, 0, 1, false), 75, 1, true).
		AddItem(nil, 0, 1, false)

	// AddPage with visible=true acts as an overlay, keeping sys_settings visible in the background
	s.app.Pages.AddPage("import_export_modal", flex, true, true)
	s.app.TviewApp.SetFocus(modal)
}

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
		SetTitle(" ⚙️ 全局系统首选项 [gray](Ctrl+E: 导入/导出配置, ESC: 取消并返回)[-] ").
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

		if event.Key() == tcell.KeyCtrlE {
			showImportExportModal(s, form)
			return nil
		}

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
