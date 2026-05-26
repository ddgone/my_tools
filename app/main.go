package main

import (
	"embed"
	"runtime"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()
	isWindows := runtime.GOOS == "windows"

	err := wails.Run(&options.App{
		Title:                    "火蜥蜴工具箱 Desktop",
		Width:                    1440,
		Height:                   960,
		MinWidth:                 1200,
		MinHeight:                760,
		DisableResize:            false,
		Frameless:                isWindows,
		EnableDefaultContextMenu: false,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		Windows: &windows.Options{
			Theme:                             windows.Dark,
			DisableFramelessWindowDecorations: false,
			WebviewIsTransparent:              false,
			WindowIsTranslucent:               false,
			DisableWindowIcon:                 false,
			IsZoomControlEnabled:              true,
		},
		Mac: &mac.Options{
			Appearance: mac.NSAppearanceNameDarkAqua,
			TitleBar:   mac.TitleBarHiddenInset(),
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
