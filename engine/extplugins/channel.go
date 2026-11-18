package extplugins

import "errors"

// ErrUnavailable is why a feature is switched off in the Mac App Store build
// of zen: the sandbox or App Review does not allow it. Settings and the page
// show it as a note, never as an error; the plugin's status is "unavailable",
// not "blocked" (the user cannot allow it).
var ErrUnavailable = errors.New("not available in the App Store version")
