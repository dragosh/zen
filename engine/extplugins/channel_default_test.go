//go:build !appstore

package extplugins

import "testing"

// TestDevIDGate: the Developer ID build keeps every kind of plugin.
func TestDevIDGate(t *testing.T) {
	for _, kind := range []string{"program", "tool", "sandbox", "script", "page", "builtin"} {
		if err := Available(kind); err != nil {
			t.Errorf("Available(%q) = %v, want nil", kind, err)
		}
	}
}
