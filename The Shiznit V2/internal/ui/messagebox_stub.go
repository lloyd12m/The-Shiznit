//go:build !windows

package ui

import "fmt"

// ShowNativeMessageBox is a console fallback for non-Windows builds.
func ShowNativeMessageBox(title, message string) {
	fmt.Printf("[%s] %s\n", title, message)
}
