package main

import (
	"runtime"
	"syscall"
	"unsafe"
)

var (
	user32 = syscall.NewLazyDLL("user32.dll")

	procCreateWindowExW   = user32.NewProc("CreateWindowExW")
	procDefWindowProcW    = user32.NewProc("DefWindowProcW")
	procGetMessageW       = user32.NewProc("GetMessageW")
	procRegisterClassW    = user32.NewProc("RegisterClassExW")
	procRegisterHotKey    = user32.NewProc("RegisterHotKey")
	procSetWindowLongPtrW = user32.NewProc("SetWindowLongPtrW")
	procTranslateMessage  = user32.NewProc("TranslateMessage")
	procDispatchMessageW  = user32.NewProc("DispatchMessageW")

	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	procGetModuleHandle = kernel32.NewProc("GetModuleHandleW")
)

// WNDCLASSEXW structure for registering a window class.
type wndClassExW struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     syscall.Handle
	hIcon         syscall.Handle
	hCursor       syscall.Handle
	hbrBackground syscall.Handle
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       syscall.Handle
}

// msg structure for GetMessage.
type msg struct {
	hwnd    syscall.Handle
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	ptX     int32
	ptY     int32
}

const (
	wmHotKey     = 0x0312
	modAlt       = 0x0001
	modNoRepeat  = 0x4000
	vkSpace      = 0x20
	gwlpWndProc  = -4
	cwUseDefault = 0x80000000
)

// hotkeyApp holds the App reference for the WndProc callback.
var hotkeyApp *App

// wndProc handles Windows messages for the hidden hotkey window.
func wndProc(hwnd syscall.Handle, message uint32, wParam, lParam uintptr) uintptr {
	if message == wmHotKey && hotkeyApp != nil {
		hotkeyApp.ToggleWindow()
		return 0
	}
	ret, _, _ := procDefWindowProcW.Call(
		uintptr(hwnd), uintptr(message), wParam, lParam,
	)
	return ret
}

// initHotkey registers Alt+Space as a global hotkey and runs the message loop.
// Must be called as a goroutine (it blocks).
func (a *App) initHotkey() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hotkeyApp = a

	// Store callback at package level to prevent GC.
	wndProcCallback := syscall.NewCallback(wndProc)

	// Get module instance.
	hInst, _, _ := procGetModuleHandle.Call(0)

	className, _ := syscall.UTF16PtrFromString("QuokkaHotkeyWnd")

	wc := wndClassExW{
		cbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		lpfnWndProc:   wndProcCallback,
		hInstance:     syscall.Handle(hInst),
		lpszClassName: className,
	}

	procRegisterClassW.Call(uintptr(unsafe.Pointer(&wc)))

	windowName, _ := syscall.UTF16PtrFromString("QuokkaHotkeyMsgWnd")

	// Create a message-only window (parent = HWND_MESSAGE).
	hwnd, _, _ := procCreateWindowExW.Call(
		0,                                   // dwExStyle
		uintptr(unsafe.Pointer(className)),  // lpClassName
		uintptr(unsafe.Pointer(windowName)), // lpWindowName
		0,                                   // dwStyle
		cwUseDefault, cwUseDefault,          // x, y
		cwUseDefault, cwUseDefault,          // w, h
		uintptr(^uintptr(2)),                // hWndParent = HWND_MESSAGE (-3)
		0,                // hMenu
		uintptr(hInst),   // hInstance
		0,                // lpParam
	)

	// Set window procedure.
	gwlIndex := int32(gwlpWndProc)
	procSetWindowLongPtrW.Call(
		uintptr(hwnd),
		uintptr(gwlIndex),
		wndProcCallback,
	)

	// Register Alt+Space hotkey (ID = 1).
	procRegisterHotKey.Call(
		uintptr(hwnd),
		1,
		uintptr(modAlt|modNoRepeat),
		uintptr(vkSpace),
	)

	// Message loop — blocks until WM_QUIT.
	var m msg
	for {
		ret, _, _ := procGetMessageW.Call(
			uintptr(unsafe.Pointer(&m)),
			0, 0, 0,
		)
		if ret == 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}
