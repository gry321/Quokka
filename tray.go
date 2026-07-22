package main

import (
	"os"

	"github.com/energye/systray"
)

// initTray starts the system tray in a background goroutine.
func (a *App) initTray() {
	go systray.Run(a.onTrayReady, a.onTrayExit)
}

func (a *App) onTrayReady() {
	systray.SetIcon(trayIcon)
	systray.SetTitle("Quokka")
	systray.SetTooltip("Quokka — A tool for Everyone")

	mShow := systray.AddMenuItem("显示窗口", "显示 Quokka 主窗口")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("退出", "退出 Quokka")

	mShow.Click(func() {
		a.ShowWindow()
	})
	mQuit.Click(func() {
		systray.Quit()
		os.Exit(0)
	})

	// 显式设置右键菜单（修复 Windows 右键只能触发一次的问题）
	systray.SetOnRClick(func(menu systray.IMenu) {
		menu.ShowMenu()
	})

	// 左键单击 toggle 窗口
	systray.SetOnClick(func(menu systray.IMenu) {
		a.ToggleWindow()
	})

	// 双击显示窗口
	systray.SetOnDClick(func(menu systray.IMenu) {
		a.ShowWindow()
	})
}

func (a *App) onTrayExit() {
	// cleanup if needed
}
