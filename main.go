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
	// Single instance: if another Quokka is running, activate it and exit.
	if !ensureSingleInstance() {
		return
	}

	app := NewApp()

	err := wails.Run(&options.App{
		Title:            "Quokka",
		Width:            620,
		Height:           110,
		MinWidth:         400,
		MinHeight:        80,
		DisableResize:    false,
		AlwaysOnTop:      true,
		Frameless:        true,
		StartHidden:      false,
		HideWindowOnClose: true,

		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
