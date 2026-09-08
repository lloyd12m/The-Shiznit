//go:build windows

package ui

import (
	"syscall"
	"unsafe"
)

var (
	user32      = syscall.NewLazyDLL("user32.dll")
	messageBoxW = user32.NewProc("MessageBoxW")
)

const mbOK = 0x00000000
const mbIconInformation = 0x00000040

// ShowNativeMessageBox displays a native Windows MessageBox.
func ShowNativeMessageBox(title, message string) {
	textPtr, _ := syscall.UTF16PtrFromString(message)
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	_, _, _ = messageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(textPtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		mbOK|mbIconInformation,
	)
}
