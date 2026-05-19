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

type settingsSnapshot struct {
	ShowVerboseShortcuts bool   `json:"show_verbose_shortcuts"`
	LogExportDir         string `json:"log_export_dir"`
	RecentToolsCount     int    `json:"recent_tools_count"`
	HistoryRetention     int    `json:"history_retention"`
	ConfirmExit          bool   `json:"confirm_exit"`
	DefaultPythonPath    string `json:"default_python_path"`
	AutoWordWrap         bool   `json:"auto_word_wrap"`
	AutoExpandAll        bool   `json:"auto_expand_all"`
}

func readSettingsFromForm(s *settingsTool, form *tview.Form) settingsSnapshot {
	var snap settingsSnapshot

	dd := form.GetFormItemByLabel("快捷键提示模式").(*tview.DropDown)
	idx, _ := dd.GetCurrentOption()
	snap.ShowVerboseShortcuts = idx == 1

	dd = form.GetFormItemByLabel("最近使用显示数量").(*tview.DropDown)
	idx, opt := dd.GetCurrentOption()
	snap.RecentToolsCount = 3
	if idx >= 0 {
		switch opt {
		case "5":
			snap.RecentToolsCount = 5
		case "10":
			snap.RecentToolsCount = 10
		}
	}

	dd = form.GetFormItemByLabel("命令历史保留数量").(*tview.DropDown)
	idx, opt = dd.GetCurrentOption()
	snap.HistoryRetention = 50
	if idx >= 0 {
		switch opt {
		case "20":
			snap.HistoryRetention = 20
		case "100":
			snap.HistoryRetention = 100
		case "200":
			snap.HistoryRetention = 200
		}
	}

	input := form.GetFormItemByLabel("日志导出目录").(*tview.InputField)
	snap.LogExportDir = strings.TrimSpace(input.GetText())

	input = form.GetFormItemByLabel("默认Python解释器").(*tview.InputField)
	snap.DefaultPythonPath = strings.TrimSpace(input.GetText())

	dd = form.GetFormItemByLabel("退出前确认").(*tview.DropDown)
	idx, _ = dd.GetCurrentOption()
	snap.ConfirmExit = idx == 1

	dd = form.GetFormItemByLabel("终端输出自动换行").(*tview.DropDown)
	idx, _ = dd.GetCurrentOption()
	snap.AutoWordWrap = idx == 1

	dd = form.GetFormItemByLabel("启动时展开所有分类").(*tview.DropDown)
	idx, _ = dd.GetCurrentOption()
	snap.AutoExpandAll = idx == 1

	return snap
}

func applySettingsToStore(s *settingsTool, snap settingsSnapshot) {
	s.app.Store.SetShowVerboseShortcuts(snap.ShowVerboseShortcuts)
	s.app.Store.SetLogExportDir(snap.LogExportDir)
	s.app.Store.SetRecentToolsCount(snap.RecentToolsCount)
	s.app.Store.SetHistoryRetention(snap.HistoryRetention)
	s.app.Store.SetConfirmExit(snap.ConfirmExit)
	s.app.Store.SetDefaultPythonPath(snap.DefaultPythonPath)
	s.app.Store.SetAutoWordWrap(snap.AutoWordWrap)
	s.app.Store.SetAutoExpandAll(snap.AutoExpandAll)
}

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

	snap := readSettingsFromForm(s, form)

	b, _ := json.Marshal(snap)
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
		var newSettings settingsSnapshot
		if err := json.Unmarshal(decoded, &newSettings); err != nil {
			s.app.ShowModal("解析错误", "配置字符串内容无效。")
			return
		}

		dd := form.GetFormItemByLabel("快捷键提示模式").(*tview.DropDown)
		if newSettings.ShowVerboseShortcuts {
			dd.SetCurrentOption(1)
		} else {
			dd.SetCurrentOption(0)
		}

		dd = form.GetFormItemByLabel("最近使用显示数量").(*tview.DropDown)
		switch newSettings.RecentToolsCount {
		case 5:
			dd.SetCurrentOption(1)
		case 10:
			dd.SetCurrentOption(2)
		default:
			dd.SetCurrentOption(0)
		}

		dd = form.GetFormItemByLabel("命令历史保留数量").(*tview.DropDown)
		switch newSettings.HistoryRetention {
		case 20:
			dd.SetCurrentOption(0)
		case 100:
			dd.SetCurrentOption(2)
		case 200:
			dd.SetCurrentOption(3)
		default:
			dd.SetCurrentOption(1)
		}

		input := form.GetFormItemByLabel("日志导出目录").(*tview.InputField)
		input.SetText(newSettings.LogExportDir)

		input = form.GetFormItemByLabel("默认Python解释器").(*tview.InputField)
		input.SetText(newSettings.DefaultPythonPath)

		dd = form.GetFormItemByLabel("退出前确认").(*tview.DropDown)
		if newSettings.ConfirmExit {
			dd.SetCurrentOption(1)
		} else {
			dd.SetCurrentOption(0)
		}

		dd = form.GetFormItemByLabel("终端输出自动换行").(*tview.DropDown)
		if newSettings.AutoWordWrap {
			dd.SetCurrentOption(1)
		} else {
			dd.SetCurrentOption(0)
		}

		dd = form.GetFormItemByLabel("启动时展开所有分类").(*tview.DropDown)
		if newSettings.AutoExpandAll {
			dd.SetCurrentOption(1)
		} else {
			dd.SetCurrentOption(0)
		}

		s.app.Pages.RemovePage("import_export_modal")
		s.app.Pages.SwitchToPage(s.ID())
		s.app.TviewApp.SetFocus(form)
	})

	modal.AddButton("取消", func() {
		s.app.Pages.RemovePage("import_export_modal")
		s.app.Pages.SwitchToPage(s.ID())
		s.app.TviewApp.SetFocus(form)
	})

	modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			s.app.Pages.RemovePage("import_export_modal")
			s.app.Pages.SwitchToPage(s.ID())
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
			AddItem(modal, 15, 1, true).
			AddItem(nil, 0, 1, false), 75, 1, true).
		AddItem(nil, 0, 1, false)

	s.app.Pages.AddPage("import_export_modal", flex, true, true)
	s.app.TviewApp.SetFocus(modal)
}

func (s *settingsTool) Execute(ctx framework.AppContext) {
	initial := settingsSnapshot{
		ShowVerboseShortcuts: s.app.Store.GetShowVerboseShortcuts(),
		LogExportDir:         s.app.Store.GetLogExportDir(),
		RecentToolsCount:     s.app.Store.GetRecentToolsCount(),
		HistoryRetention:     s.app.Store.GetHistoryRetention(),
		ConfirmExit:          s.app.Store.GetConfirmExit(),
		DefaultPythonPath:    s.app.Store.GetDefaultPythonPath(),
		AutoWordWrap:         s.app.Store.GetAutoWordWrap(),
		AutoExpandAll:        s.app.Store.GetAutoExpandAll(),
	}

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

	shortcutOptions := []string{
		"精简模式 (推荐，仅按 F1 呼出面板)",
		"详细模式 (每个面板标题栏显示详细快捷键)",
	}
	shortcutIdx := 0
	if initial.ShowVerboseShortcuts {
		shortcutIdx = 1
	}
	form.AddDropDown("快捷键提示模式", shortcutOptions, shortcutIdx, func(option string, optionIndex int) {
		s.app.Store.SetShowVerboseShortcuts(optionIndex == 1)
	})

	countOptions := []string{"3", "5", "10"}
	countIdx := 0
	switch initial.RecentToolsCount {
	case 5:
		countIdx = 1
	case 10:
		countIdx = 2
	}
	form.AddDropDown("最近使用显示数量", countOptions, countIdx, func(option string, optionIndex int) {
		val := 3
		switch option {
		case "5":
			val = 5
		case "10":
			val = 10
		}
		s.app.Store.SetRecentToolsCount(val)
	})

	historyOptions := []string{"20", "50", "100", "200"}
	historyIdx := 1
	switch initial.HistoryRetention {
	case 20:
		historyIdx = 0
	case 100:
		historyIdx = 2
	case 200:
		historyIdx = 3
	}
	form.AddDropDown("命令历史保留数量", historyOptions, historyIdx, func(option string, optionIndex int) {
		val := 50
		switch option {
		case "20":
			val = 20
		case "100":
			val = 100
		case "200":
			val = 200
		}
		s.app.Store.SetHistoryRetention(val)
	})

	form.AddInputField("日志导出目录", initial.LogExportDir, 40, nil, func(text string) {
		trimmed := strings.TrimSpace(text)
		if trimmed != "" {
			s.app.Store.SetLogExportDir(trimmed)
		}
	})

	form.AddInputField("默认Python解释器", initial.DefaultPythonPath, 40, nil, func(text string) {
		trimmed := strings.TrimSpace(text)
		if trimmed != "" {
			s.app.Store.SetDefaultPythonPath(trimmed)
		} else {
			s.app.Store.SetDefaultPythonPath("python")
		}
	})

	onOffOptions := []string{"关闭", "开启"}
	confirmIdx := 0
	if initial.ConfirmExit {
		confirmIdx = 1
	}
	form.AddDropDown("退出前确认", onOffOptions, confirmIdx, func(option string, optionIndex int) {
		s.app.Store.SetConfirmExit(optionIndex == 1)
	})

	wrapIdx := 0
	if initial.AutoWordWrap {
		wrapIdx = 1
	}
	form.AddDropDown("终端输出自动换行", onOffOptions, wrapIdx, func(option string, optionIndex int) {
		s.app.Store.SetAutoWordWrap(optionIndex == 1)
	})

	expandIdx := 0
	if initial.AutoExpandAll {
		expandIdx = 1
	}
	form.AddDropDown("启动时展开所有分类", onOffOptions, expandIdx, func(option string, optionIndex int) {
		s.app.Store.SetAutoExpandAll(optionIndex == 1)
	})

	for _, label := range []string{
		"快捷键提示模式", "最近使用显示数量", "命令历史保留数量",
		"退出前确认", "终端输出自动换行", "启动时展开所有分类",
	} {
		if dd, ok := form.GetFormItemByLabel(label).(*tview.DropDown); ok {
			dd.SetFieldWidth(45)
		}
	}

	form.AddButton("保存并返回", func() {
		s.app.Pages.RemovePage(s.ID())
		s.app.setupUI()
		s.app.UpdateAllPanelTitles()

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
		if _, isList := focus.(*tview.List); isList {
			return event
		}

		if event.Key() == tcell.KeyCtrlE {
			showImportExportModal(s, form)
			return nil
		}

		if event.Key() == tcell.KeyDown {
			return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
		}
		if event.Key() == tcell.KeyUp {
			return tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone)
		}

		if event.Key() == tcell.KeyEscape {
			applySettingsToStore(s, initial)
			s.app.Pages.RemovePage(s.ID())
			s.app.setupUI()
			s.app.UpdateAllPanelTitles()

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

	contentLayout := tview.NewFlex().SetDirection(tview.FlexRow)
	contentLayout.AddItem(form, 22, 1, true)

	mainFlex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(contentLayout, 22, 1, true).
			AddItem(nil, 0, 1, false), 80, 1, true).
		AddItem(nil, 0, 1, false)

	s.app.Pages.AddPage(s.ID(), mainFlex, true, true)
}
