//go:build !(darwin && cgo)

package desktop

import "github.com/wailsapp/wails/v3/pkg/application"

func nativeSavePDF(application.Window, string, string) (int, error) { return 0, ErrPDFUnsupported }

func hasWindowServer() bool { return false }
