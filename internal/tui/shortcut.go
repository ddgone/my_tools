package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (a *App) showShortcutHelp() {
	if a.Pages.HasPage("shortcuts") {
		a.Pages.RemovePage("shortcuts")
		return
	}

	helpText := `[orange::b]🌍 全局 (Global)[-:-:-]
 [gray]Ctrl+P 或 /[-]   呼出全局命令搜索面板
 [gray]Ctrl+C[-]        强制中断当前正在执行的任务/进程
 [gray]Ctrl+F[-]        收藏/取消收藏当前选中的工具
 [gray]F1[-]            显示此快捷键帮助
 [gray]q[-]            退出整个应用程序

[yellow::b]🏠 主页导航 (Home)[-:-:-]
 [gray]↑ / ↓[-]        上下选择工具或目录
 [gray]← / →[-]        折叠/展开目录 (←还可返回上级)
 [gray]Enter[-]        执行选中的工具，或展开目录
 [gray]r[-]            重置并折叠所有目录树状态
 [gray]b[-]            折叠/展开顶部横幅

[green::b]⌨️  命令行输入框 (Input)[-:-:-]
 [gray]Enter[-]        执行当前输入的命令
 [gray]ESC[-]          退出当前工具，返回主页
 [gray]↑ / ↓[-]        翻阅当前工具的历史执行命令
 [gray]Tab[-]          在 输入框、输出框、说明框 间切换焦点
 [gray]Ctrl+E[-]       全屏最大化当前获得焦点的面板
 [gray]Ctrl+N[-]       清空输入框，准备输入新命令
 [gray]Ctrl+A[-]       将当前输入的完整命令复制到系统剪贴板
 [gray]Ctrl+H[-]       打开历史记录与输出预览浮窗

[cyan::b]📺 终端输出面板 (Output)[-:-:-]
 [gray]Ctrl+L[-]       清空当前终端的输出日志和已完成的任务
 [gray]Ctrl+U[-]       撤销上一次的清空操作，恢复日志和任务
 [gray]Ctrl+S[-]       将当前所有输出日志导出为本地文本文件
 [gray]Ctrl+E[-]       全屏最大化输出日志面板 (适合阅读长日志)
 [gray]Ctrl+C[-]       强制取消当前正在执行的任务

[purple::b]📋 任务侧边栏 (Task Panel)[-:-:-]
 [gray]Ctrl+B[-]       显示/隐藏多任务栏 (仅当有多个任务时有效)
 [gray]Enter[-]        切换到选中的任务，查看其日志`

	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetText(helpText).
		SetRegions(true).
		SetWrap(true)

	textView.SetBorder(true).
		SetTitle(" ⌨️ 快捷键速查速记 (按 ESC 退出) ").
		SetTitleColor(tcell.ColorYellow).
		SetBorderColor(tcell.ColorOrange)

	// 设置一个居中的模态框
	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(textView, 32, 1, true).
			AddItem(nil, 0, 1, false), 65, 1, true).
		AddItem(nil, 0, 1, false)

	// 捕获按键退出
	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Key() == tcell.KeyEnter || event.Key() == tcell.KeyF1 || (event.Key() == tcell.KeyRune && event.Rune() == 'q') {
			a.Pages.RemovePage("shortcuts")
			return nil
		}
		return event
	})

	a.Pages.AddPage("shortcuts", flex, true, true)
	a.TviewApp.SetFocus(flex)
}
