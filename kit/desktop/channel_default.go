//go:build !appstore

package desktop

// appStore is true in the Mac App Store build (go build -tags appstore).
const appStore = false

// bookmarks is the folder access of this build: none. Outside the sandbox zen
// reads any folder directly.
func bookmarks() folderAccess { return openAccess{} }

type openAccess struct{}

func (openAccess) Remember(string) error          { return nil }
func (openAccess) Resolve(string) (func(), error) { return noop, nil }
