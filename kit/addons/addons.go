// Package addons is the registry of an app's add-ons: optional capabilities
// (a helper program, the Go code that drives it and the UI it adds) that an
// app includes only when it is built with them (specs/kit/addons).
//
// An add-on is a Go package addons/<id> whose files carry the build tag
// addon_<id>. Its init calls Register and wires itself in through the kit's
// hooks (for example engine/session.SetLinter); the app imports it, under the
// same tag, in apps/<app>/addons_<id>.go. Without the tag none of its Go code
// is linked and the registry stays empty.
package addons

import (
	"fmt"
	"regexp"
	"sort"
	"sync"
)

// Addon says what an add-on is, for people: Settings › Add-ons lists these.
type Addon struct{ ID, Name, Summary string }

var idRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

// ValidID reports whether id may name an add-on: a build tag and a path
// segment, so it is a short lowercase word.
func ValidID(id string) bool { return idRE.MatchString(id) }

var (
	mu       sync.Mutex
	registry = map[string]Addon{}
)

// Register adds an add-on. It panics on an invalid or duplicate id: both are
// mistakes of the build, not of the user.
func Register(a Addon) {
	if !ValidID(a.ID) {
		panic(fmt.Sprintf("addons: invalid add-on id %q (want %s)", a.ID, idRE))
	}
	mu.Lock()
	defer mu.Unlock()
	if _, dup := registry[a.ID]; dup {
		panic(fmt.Sprintf("addons: add-on %q registered twice", a.ID))
	}
	registry[a.ID] = a
}

// List returns the registered add-ons sorted by id.
func List() []Addon {
	mu.Lock()
	defer mu.Unlock()
	out := make([]Addon, 0, len(registry))
	for _, a := range registry {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Has reports whether the add-on is registered.
func Has(id string) bool {
	mu.Lock()
	defer mu.Unlock()
	_, ok := registry[id]
	return ok
}
