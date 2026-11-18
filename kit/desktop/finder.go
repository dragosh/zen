package desktop

import "os/exec"

// macOpen is the one place an app hands a path to macOS: Show in Finder for
// the templates folder and a built website, and the plain-text fallback for
// the settings files. In the App Store build it is only given paths the user
// chose or the app's own container; before submission it should become an
// NSWorkspace call (docs/DISTRIBUTION.md). Tests replace it.
var macOpen = func(args ...string) error { return exec.Command("open", args...).Run() }

// Reveal shows a file or folder in Finder.
func Reveal(path string) error { return macOpen(path) }

// OpenFile runs /usr/bin/open with args, for example "-t", path to open a file
// in the default text editor.
func OpenFile(args ...string) error { return macOpen(args...) }
