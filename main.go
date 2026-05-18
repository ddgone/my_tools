package main

import (
	"fmt"
	"os"

	"my_tools/internal/storage"
	"my_tools/internal/tui"

	// 注册所有的工具
	_ "my_tools/tools/pos2gis"
	_ "my_tools/tools/python_tools"
	_ "my_tools/tools/text"
	_ "my_tools/tools/utm_geojson"
)

func main() {
	// 加载用户数据
	store := storage.NewStorage()
	if err := store.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "警告: 加载用户数据失败: %v\n", err)
	}

	// 初始化并运行新版 TUI (基于 tview)
	app := tui.NewApp(store)
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "启动失败: %v\n", err)
		os.Exit(1)
	}
}
