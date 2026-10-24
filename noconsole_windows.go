//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

// hideOwnConsole closes the extra cmd window that appears when the exe is
// started by double-click (or Explorer) without -H windowsgui.
// A console inherited from an existing terminal is left alone so `go run`
// and `go test` still print.
func init() {
	hideOwnConsole()
}

func hideOwnConsole() {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	u32 := syscall.NewLazyDLL("user32.dll")
	getConsoleWindow := k32.NewProc("GetConsoleWindow")
	getConsoleProcessList := k32.NewProc("GetConsoleProcessList")
	freeConsole := k32.NewProc("FreeConsole")
	showWindow := u32.NewProc("ShowWindow")

	hwnd, _, _ := getConsoleWindow.Call()
	if hwnd == 0 {
		return
	}
	var pids [2]uint32
	n, _, _ := getConsoleProcessList.Call(uintptr(unsafe.Pointer(&pids[0])), 2)
	if n != 1 {
		return
	}
	const swHide = 0
	showWindow.Call(hwnd, swHide)
	freeConsole.Call()
}
