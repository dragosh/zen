// Package dialects holds markdown dialects: other syntaxes for the container
// plugins that `:::name` already reaches (specs/engine/dialects). MyST's
// ```{note} fence is the first. A dialect only finds directives in a page's
// source; engine/render turns what it finds into the same plugin blocks as a
// `:::` container, so a folder renders the same everywhere.
package dialects

import (
	"fmt"
	"io/fs"
	"regexp"

	"github.com/zenmark-org/zenmd/engine/plugins"
)

// Dialect is one alternative syntax.
type Dialect interface {
	ID() string      // "myst"
	Name() string    // "MyST"
	Summary() string // one line for Settings
	// Scan finds the dialect's directives in src (a page body), outside
	// ordinary code fences. Spans are in source order and never overlap.
	Scan(src []byte) []Span
	// Detect reports whether the folder looks like it was written for this
	// dialect. It reads root files only and never runs anything.
	Detect(root fs.FS) bool
}

// Span is one directive. The embedded span is what `:::` containers use: the
// name, the attributes and the raw body.
type Span struct {
	plugins.Span
	Code bool   // a code directive: render as a fence in Lang
	Lang string // the fence language of a code directive
	// Warnings are problems found in the directive (a dropped option); the
	// page shows them.
	Warnings []string
}

var idRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

// ValidID reports whether id may name a dialect (the same shape as an add-on id).
func ValidID(id string) bool { return idRE.MatchString(id) }

// Registry is an ordered set of dialects.
type Registry struct{ list []Dialect }

// NewRegistry returns a registry of ds, in order. An invalid or repeated id is
// an error.
func NewRegistry(ds ...Dialect) (*Registry, error) {
	r := &Registry{}
	seen := map[string]bool{}
	for _, d := range ds {
		id := d.ID()
		if !ValidID(id) {
			return nil, fmt.Errorf("dialects: invalid dialect id %q (want %s)", id, idRE)
		}
		if seen[id] {
			return nil, fmt.Errorf("dialects: dialect %q registered twice", id)
		}
		seen[id] = true
		r.list = append(r.list, d)
	}
	return r, nil
}

// Get returns the dialect with that id, or nil. It is safe on a nil registry.
func (r *Registry) Get(id string) Dialect {
	if r == nil {
		return nil
	}
	for _, d := range r.list {
		if d.ID() == id {
			return d
		}
	}
	return nil
}

// List returns the dialects in registration order.
func (r *Registry) List() []Dialect {
	if r == nil {
		return nil
	}
	return append([]Dialect(nil), r.list...)
}
