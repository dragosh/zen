/*
* -----------------------------------------------------------
* ☢️ Work in progress Public API
* -----------------------------------------------------------
 */

package api

import (
	"embed"
	"os"

	"github.com/dragosh/zen/internal/logger"
)

const (
	DEV_MODE = false
)

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

var (
	LOG_LEVEL = getEnv("LOG_LEVEL", "warning")
	Log       = logger.Create(LOG_LEVEL)
)

/*
* -----------------------------------------------------------
* Preview api
* -----------------------------------------------------------
 */

const GLOBAL_NAMESPACE = "__zen__"

type EntryKind uint8

const (
	MdEntry     EntryKind = 1
	ConfigEntry EntryKind = 2
)

type StaticAppName string

type DocPreviewEntry struct {
	FileRoot      string
	FileName      string
	FilePath      string
	FileExt       string
	FileSize      string
	FileContents  string
	StaticAppName string
	FileKind      EntryKind // todo
}

type EmbededApp struct {
	StaticAppName StaticAppName
	StaticFiles   embed.FS
	DefaultIndex  embed.FS
	DefaultPath   string
}

func init() {
	// Log.Infof("Dev Mode: %t", DEV_MODE)
	// Log.Infof("Log Level: %s", LOG_LEVEL)
}

// type Options struct {
// 	value string
// }

// func Preview(ops Options) someResult {
// 	return previewImpl(ops)
// }
