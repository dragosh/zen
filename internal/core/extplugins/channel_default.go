//go:build !appstore

package extplugins

// Available reports whether plugins of kind may run in this build. The
// Developer ID build runs everything.
func Available(kind string) error { return nil }
