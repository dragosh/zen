//go:build !darwin

package session

import "os"

// birthTime: creation time is not portable outside macOS; 0 means unknown.
func birthTime(os.FileInfo) int64 { return 0 }
