//go:build appstore

package app

import (
	"os"
	"path/filepath"
)

// appStore is true in the Mac App Store build (go build -tags appstore).
const appStore = true

// bookmarks keeps security-scoped bookmarks in the app container, next to the
// trust file (the sandbox remaps os.UserConfigDir() there).
func bookmarks() folderAccess {
	dir, err := os.UserConfigDir()
	if err != nil {
		return noAccess{err}
	}
	return newStoreAccess(filepath.Join(dir, "zen", "bookmarks.json"), cocoaBookmarks{})
}

// noAccess is used when there is no settings folder: nothing can be remembered.
type noAccess struct{ err error }

func (n noAccess) Remember(string) error          { return n.err }
func (n noAccess) Resolve(string) (func(), error) { return nil, ErrNoBookmark }
