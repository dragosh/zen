package app

import "os/exec"

// macOpen is the one place the app hands a path to macOS: Show in Finder for
// the templates folder and a built website, and the plain-text fallback for
// the settings files. In the App Store build it is only given paths the user
// chose or the app's own container; before submission it should become an
// NSWorkspace call (docs/DISTRIBUTION.md). Tests replace it.
var macOpen = func(args ...string) error { return exec.Command("open", args...).Run() }
