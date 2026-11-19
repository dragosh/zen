package addons

import (
	"strings"
	"testing"
)

// reset empties the registry for one test and restores it afterwards.
func reset(t *testing.T) {
	t.Helper()
	mu.Lock()
	saved := registry
	registry = map[string]Addon{}
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		registry = saved
		mu.Unlock()
	})
}

func TestRegisterAndList(t *testing.T) {
	reset(t)
	if got := List(); len(got) != 0 {
		t.Fatalf("empty registry lists %v", got)
	}
	if Has("vale") {
		t.Fatal("Has on an empty registry")
	}
	Register(Addon{ID: "voice", Name: "Voice", Summary: "Dictation."})
	Register(Addon{ID: "vale", Name: "Vale linting", Summary: "Checks."})
	Register(Addon{ID: "a-1", Name: "A", Summary: "a"})
	got := List()
	var ids []string
	for _, a := range got {
		ids = append(ids, a.ID)
	}
	if strings.Join(ids, ",") != "a-1,vale,voice" {
		t.Fatalf("order = %v, want sorted by id", ids)
	}
	if got[1].Name != "Vale linting" || got[1].Summary != "Checks." {
		t.Fatalf("fields lost: %+v", got[1])
	}
	if !Has("vale") || Has("nope") {
		t.Fatal("Has")
	}
	got[0].ID = "changed" // List returns a copy
	if List()[0].ID != "a-1" {
		t.Fatal("List exposes the registry")
	}
}

func TestRegisterPanics(t *testing.T) {
	reset(t)
	Register(Addon{ID: "vale", Name: "Vale", Summary: "x"})
	bad := []string{"", "Vale", "1vale", "va le", "va/le", "-vale", "vale_x", "../x", strings.Repeat("a", 33)}
	for _, id := range bad {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("Register(%q) did not panic", id)
				}
			}()
			Register(Addon{ID: id, Name: "n", Summary: "s"})
		}()
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("duplicate id did not panic")
			}
		}()
		Register(Addon{ID: "vale", Name: "Again", Summary: "s"})
	}()
	Register(Addon{ID: strings.Repeat("a", 32), Name: "long", Summary: "ok"}) // 32 characters is allowed
}

func TestValidID(t *testing.T) {
	for id, want := range map[string]bool{"vale": true, "a": true, "voice-2": true, "": false, "V": false, "a_b": false, "a.b": false} {
		if ValidID(id) != want {
			t.Errorf("ValidID(%q) = %v", id, !want)
		}
	}
}
