//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

const mutexName = "Global\\QuokkaWails_SingleInstance_Mutex"

// ensureSingleInstance checks if another Quokka instance is running.
// If yes, it activates the existing window and returns false (caller should exit).
// If no, it returns true (caller should continue running).
func ensureSingleInstance() bool {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	procCreateMutex := kernel32.NewProc("CreateMutexW")
	procGetLastError := kernel32.NewProc("GetLastError")

	namePtr, _ := syscall.UTF16PtrFromString(mutexName)
	handle, _, _ := procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(namePtr)))
	if handle == 0 {
		// CreateMutex failed, assume we can continue
		return true
	}

	lastErr, _, _ := procGetLastError.Call()
	const ERROR_ALREADY_EXISTS = 183

	if lastErr == ERROR_ALREADY_EXISTS {
		// Another instance is running — try to bring its window to front
		syscall.CloseHandle(syscall.Handle(handle))
		activateExistingWindow()
		return false
	}

	// We own the mutex, keep it alive for the process lifetime
	// (it will be released automatically when the process exits)
	return true
}

// activateExistingWindow finds the Quokka window and brings it to foreground.
func activateExistingWindow() {
	user32 := syscall.NewLazyDLL("user32.dll")
	procFindWindow := user32.NewProc("FindWindowW")
	procShowWindow := user32.NewProc("ShowWindow")
	procSetForeground := user32.NewProc("SetForegroundWindow")
	procSetWindowPos := user32.NewProc("SetWindowPos")

	const (
		SW_RESTORE = 9
		SW_SHOW    = 5
	)

	titlePtr, _ := syscall.UTF16PtrFromString("Quokka")

	// Try to find the main Quokka window
	hwnd, _, _ := procFindWindow.Call(0, uintptr(unsafe.Pointer(titlePtr)))
	if hwnd == 0 {
		// Also try the hotkey message window class
		className, _ := syscall.UTF16PtrFromString("QuokkaHotkeyWnd")
		hwnd, _, _ = procFindWindow.Call(uintptr(unsafe.Pointer(className)), 0)
	}

	if hwnd != 0 {
		// Restore if minimized
		procShowWindow.Call(hwnd, SW_RESTORE)
		procShowWindow.Call(hwnd, SW_SHOW)
		// Bring to foreground
		procSetForeground.Call(hwnd)
		// SWP_NOMOVE | SWP_NOSIZE | SWP_SHOWWINDOW to force show
		const SWP_NOMOVE = 0x0002
		const SWP_NOSIZE = 0x0001
		const SWP_SHOWWINDOW = 0x0040
		procSetWindowPos.Call(hwnd, 0, 0, 0, 0, 0, SWP_NOMOVE|SWP_NOSIZE|SWP_SHOWWINDOW)
	}
}
