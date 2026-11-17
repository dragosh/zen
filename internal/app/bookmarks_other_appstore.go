//go:build appstore && !darwin

package app

// cocoaBookmarks has no meaning off macOS; the App Store build is macOS only.
// This keeps `go vet -tags appstore ./...` working on other hosts.
type cocoaBookmarks struct{}

func (cocoaBookmarks) Create(string) ([]byte, error) { return nil, ErrNoBookmark }
func (cocoaBookmarks) Resolve([]byte) (string, bool, func(), error) {
	return "", false, nil, ErrNoBookmark
}
