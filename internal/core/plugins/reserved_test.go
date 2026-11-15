package plugins

import "testing"

type reservedPlugin struct{}

func (reservedPlugin) Manifest() Manifest {
	return Manifest{ID: "lint", Version: "0.1.0", APIVersion: APIVersion}
}

func TestReservedIDs(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(reservedPlugin{}); err == nil {
		t.Fatal("registering a reserved id must fail")
	}
	none := func(string) (bool, map[string]any, bool) { return false, nil, false }
	_, warnings := r.Enabled(none, []string{"lint", "nope"})
	if len(warnings) != 1 || warnings[0] != `unknown plugin "nope"` {
		t.Fatalf("warnings = %v", warnings)
	}
}
