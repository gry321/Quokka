package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:         "Quokka",
		Width:         600, // 初始尺寸，之后会被调整
		Height:        120,
		DisableResize: false, // 必须允许程序调整
		AlwaysOnTop:   true,
		Frameless:     true,
		//MinSize: &options.Size{      // 锁定尺寸（让用户无法手动调整）
		//	Width:  600,
		//	Height: 120,
		//},
		//MaxSize: &options.Size{
		//	Width:  600,
		//	Height: 120,
		//},
		MaxWidth:  600,
		MaxHeight: 120,
		MinWidth:  600,
		MinHeight: 120,

		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
