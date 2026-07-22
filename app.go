package main

import (
	"context"
	"os/exec"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/energye/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var (
	user32Focus        = syscall.NewLazyDLL("user32.dll")
	procGetForeground   = user32Focus.NewProc("GetForegroundWindow")
	procFindWindowW     = user32Focus.NewProc("FindWindowW")
)

// App holds the application context.
type App struct {
	ctx     context.Context
	visible bool
	mu      sync.Mutex
	launcher *LauncherIndex
	plugins  *PluginManager
}

// NewApp creates a new App instance.
func NewApp() *App {
	return &App{visible: true}
}

// startup is called when the app starts up.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.plugins = NewPluginManager()
	a.initTray()
	go a.initHotkey()
	go func() {
		a.launcher = NewLauncherIndex()
	}()
	go a.focusWatcher()
}

// shutdown is called when the app is closing.
func (a *App) shutdown(ctx context.Context) {
	systray.Quit()
}

// focusWatcher hides the window when it loses focus to another app.
func (a *App) focusWatcher() {
	time.Sleep(2 * time.Second) // wait for Wails window to be created

	titlePtr, _ := syscall.UTF16PtrFromString("Quokka")
	var ourHWND uintptr
	for {
		// Find our window handle (retry until found)
		for ourHWND == 0 {
			h, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(titlePtr)))
			if h != 0 {
				ourHWND = h
				break
			}
			time.Sleep(500 * time.Millisecond)
		}

		// Wait for window to become visible
		for !a.isVisible() {
			time.Sleep(100 * time.Millisecond)
		}

		// Grace period after showing (avoid hiding before gaining focus)
		time.Sleep(600 * time.Millisecond)

		// Monitor: hide when another window takes focus
		for a.isVisible() {
			fg, _, _ := procGetForeground.Call()
			if fg != 0 && fg != ourHWND {
				a.hideAndNotify()
				break
			}
			time.Sleep(80 * time.Millisecond)
		}
	}
}

func (a *App) isVisible() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.visible
}

func (a *App) hideAndNotify() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.visible {
		runtime.WindowHide(a.ctx)
		a.visible = false
		runtime.EventsEmit(a.ctx, "windowHidden")
	}
}

// ResizeWindow dynamically resizes the window to fit the content.
func (a *App) ResizeWindow(width, height int) {
	runtime.WindowSetSize(a.ctx, width, height)
}

// ToggleWindow shows or hides the window.
func (a *App) ToggleWindow() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.visible {
		runtime.WindowHide(a.ctx)
		a.visible = false
		runtime.EventsEmit(a.ctx, "windowHidden")
	} else {
		runtime.WindowShow(a.ctx)
		runtime.WindowCenter(a.ctx)
		a.visible = true
		runtime.EventsEmit(a.ctx, "windowShown")
	}
}

// ShowWindow shows and centres the application window.
func (a *App) ShowWindow() {
	a.mu.Lock()
	defer a.mu.Unlock()
	runtime.WindowShow(a.ctx)
	runtime.WindowCenter(a.ctx)
	a.visible = true
	runtime.EventsEmit(a.ctx, "windowShown")
}

// HideWindow hides the application window (minimize to tray).
func (a *App) HideWindow() {
	a.mu.Lock()
	defer a.mu.Unlock()
	runtime.WindowHide(a.ctx)
	a.visible = false
	runtime.EventsEmit(a.ctx, "windowHidden")
}

// Shutdown gracefully quits the application.
func (a *App) Shutdown() {
	runtime.Quit(a.ctx)
}

// SearchApps performs a fuzzy/pinyin search on indexed applications.
func (a *App) SearchApps(query string) []AppEntry {
	if a.launcher == nil {
		return nil
	}
	return a.launcher.Search(query)
}

// LaunchApp launches an application by its path.
func (a *App) LaunchApp(path string) string {
	cmd := exec.Command("cmd", "/c", "start", "", path)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}
	if err := cmd.Run(); err != nil {
		return err.Error()
	}
	return ""
}

// GetAppIcon returns a base64-encoded PNG data URI for the given file's icon.
func (a *App) GetAppIcon(path string) string {
	return GetFileIcon(path)
}

// GetForegroundHWND returns the handle of the current foreground window.
func (a *App) GetForegroundHWND() uintptr {
	hwnd, _, _ := procGetForeground.Call()
	return hwnd
}

// ============================================================
// Plugin API (bound to frontend)
// ============================================================

// AddPlugin registers a DLL plugin.
func (a *App) AddPlugin(name, dllPath string) string {
	if a.plugins == nil {
		return "Plugin system not initialized"
	}
	return a.plugins.AddPlugin(name, dllPath)
}

// RemovePlugin removes a plugin by index.
func (a *App) RemovePlugin(index int) string {
	if a.plugins == nil {
		return "Plugin system not initialized"
	}
	return a.plugins.RemovePlugin(index)
}

// TogglePlugin enables/disables a plugin by index.
func (a *App) TogglePlugin(index int) string {
	if a.plugins == nil {
		return "Plugin system not initialized"
	}
	return a.plugins.TogglePlugin(index)
}

// ListPlugins returns all registered plugins.
func (a *App) ListPlugins() []PluginInfo {
	if a.plugins == nil {
		return nil
	}
	return a.plugins.ListPlugins()
}

// RunPlugins executes all enabled plugins and returns entries.
func (a *App) RunPlugins(query string) []PluginEntry {
	if a.plugins == nil {
		return nil
	}
	hwnd, _, _ := procGetForeground.Call()
	return a.plugins.RunPlugins(query, hwnd)
}
