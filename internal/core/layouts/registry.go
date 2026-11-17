package layouts

import (
	"fmt"
	"io/fs"
	"sync/atomic"
)

// Registry holds the layouts a folder can use. Build it, then share it:
// lookups are safe for concurrent use as long as nothing registers any more.
type Registry struct {
	items    []*Layout
	byID     map[string]*Layout
	denied   map[string]bool
	problems map[string]string
	assetVer atomic.Pointer[string]
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{byID: map[string]*Layout{}, denied: map[string]bool{}, problems: map[string]string{}}
}

// SetAssetVersion sets the ?v= of built-in layouts' asset URLs (the app version).
func (r *Registry) SetAssetVersion(v string) { r.assetVer.Store(&v) }

// AssetVersion returns what SetAssetVersion set.
func (r *Registry) AssetVersion() string {
	if p := r.assetVer.Load(); p != nil {
		return *p
	}
	return ""
}

// Register adds a layout; ids are unique.
func (r *Registry) Register(l *Layout) error {
	id := l.man.ID
	if _, dup := r.byID[id]; dup {
		return fmt.Errorf("layout %q is already registered", id)
	}
	r.byID[id] = l
	r.items = append(r.items, l)
	return nil
}

// Get returns the registered layout id, or nil.
func (r *Registry) Get(id string) *Layout { return r.byID[id] }

// All returns the registered layouts in registration order.
func (r *Registry) All() []*Layout { return append([]*Layout(nil), r.items...) }

// Deny records a layout that exists but may not be used (blocked or not yet
// allowed), so a page that names it says so.
func (r *Registry) Deny(id string) { r.denied[id] = true }

// Denied reports whether id was recorded by Deny.
func (r *Registry) Denied(id string) bool { return r.denied[id] }

// Invalidate records a layout that exists but could not be loaded, with the
// reason, so a page that names it says why it is not used.
func (r *Registry) Invalidate(id, problem string) { r.problems[id] = problem }

// Problem returns the reason recorded by Invalidate, "" for none.
func (r *Registry) Problem(id string) string { return r.problems[id] }

// Clone returns a registry with the same layouts, denials and asset version;
// the app starts from the built-ins and adds the folder's.
func (r *Registry) Clone() *Registry {
	c := NewRegistry()
	for _, l := range r.items {
		_ = c.Register(l)
	}
	for id := range r.denied {
		c.denied[id] = true
	}
	for id, p := range r.problems {
		c.problems[id] = p
	}
	if v := r.AssetVersion(); v != "" {
		c.SetAssetVersion(v)
	}
	return c
}

// AssetFS returns the files of layout id that may be served, false when the
// layout is unknown or ships none.
func (r *Registry) AssetFS(id string) (fs.FS, bool) {
	l := r.byID[id]
	if l == nil {
		return nil, false
	}
	f := l.Assets()
	return f, f != nil
}
