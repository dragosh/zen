//go:build appstore

package extplugins

import (
	"errors"
	"strings"
	"testing"
)

// TestAppStoreGate: in the App Store build program, tool and sandbox plugins
// are unavailable (with the reason); Tengo scripts, page plugins and the
// built-ins stay.
func TestAppStoreGate(t *testing.T) {
	for _, kind := range []string{"program", "tool", "sandbox"} {
		err := Available(kind)
		if !errors.Is(err, ErrUnavailable) {
			t.Errorf("Available(%q) = %v, want ErrUnavailable", kind, err)
		}
	}
	for _, kind := range []string{"script", "page", "builtin", "layout"} {
		if err := Available(kind); err != nil {
			t.Errorf("Available(%q) = %v, want nil", kind, err)
		}
	}
	if msg := ErrUnavailable.Error(); !strings.Contains(msg, "App Store") {
		t.Errorf("reason %q does not name the App Store version", msg)
	}
}
