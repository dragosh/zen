//go:build !(darwin && cgo)

package app

import "github.com/wailsapp/wails/v3/pkg/application"

// nativePrint asks the window to print (Wails' own print panel).
func nativePrint(w application.Window, _ string) error { return w.Print() }
