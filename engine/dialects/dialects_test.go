package dialects

import (
	"io/fs"
	"testing"
)

type fake string

func (f fake) ID() string         { return string(f) }
func (f fake) Name() string       { return string(f) }
func (f fake) Summary() string    { return "" }
func (f fake) Scan([]byte) []Span { return nil }
func (f fake) Detect(fs.FS) bool  { return false }

func TestRegistry(t *testing.T) {
	r, err := NewRegistry(fake("a"), fake("b"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Get("b") == nil || r.Get("c") != nil || len(r.List()) != 2 || r.List()[0].ID() != "a" {
		t.Errorf("registry: %v", r.List())
	}
	if _, err := NewRegistry(fake("a"), fake("a")); err == nil {
		t.Error("repeated id accepted")
	}
	if _, err := NewRegistry(fake("Bad id")); err == nil {
		t.Error("invalid id accepted")
	}
	var nilReg *Registry
	if nilReg.Get("a") != nil || nilReg.List() != nil {
		t.Error("nil registry")
	}
}
