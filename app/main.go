package main

import (
	"embed"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"

	"fire-salamander-desktop/internal/runtimeenv"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

type windowConfig struct {
	Width      int  `json:"width"`
	Height     int  `json:"height"`
	X          int  `json:"x"`
	Y          int  `json:"y"`
	Maximised  bool `json:"maximised"`
	Fullscreen bool `json:"fullscreen"`
}

func loadSavedWindowConfig() windowConfig {
	layout, err := runtimeenv.ResolveLayout()
	if err != nil {
		return windowConfig{Width: 0, Height: 0, X: -1, Y: -1}
	}
	data, err := os.ReadFile(filepath.Join(layout.ConfigDir(), "app.json"))
	if err != nil {
		return windowConfig{Width: 0, Height: 0, X: -1, Y: -1}
	}
	var cfg struct {
		Window windowConfig `json:"window"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return windowConfig{Width: 0, Height: 0, X: -1, Y: -1}
	}
	return cfg.Window
}

func main() {
	app := NewApp()
	isWindows := runtime.GOOS == "windows"

	saved := loadSavedWindowConfig()
	width, height := 1280, 800
	if saved.Width > 0 && saved.Height > 0 {
		width, height = saved.Width, saved.Height
	}

	err := wails.Run(&options.App{
		Title:                    "火蜥蜴工具箱 Desktop",
		Width:                    width,
		Height:                   height,
		MinWidth:                 400,
		MinHeight:                300,
		DisableResize:            false,
		Frameless:                isWindows,
		EnableDefaultContextMenu: false,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		OnDomReady:       app.domReady,
		OnBeforeClose:    app.beforeClose,
		OnShutdown:       app.shutdown,
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
