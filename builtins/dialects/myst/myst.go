// Package myst is the MyST dialect (specs/engine/dialects).
package myst

import (
	"io/fs"

	"github.com/zenmark-org/zenmd/engine/dialects"
)

type dialect struct{}

// New returns the MyST dialect.
func New() dialects.Dialect { return dialect{} }

func (dialect) ID() string      { return "myst" }
func (dialect) Name() string    { return "MyST" }
func (dialect) Summary() string { return "" }

func (dialect) Scan(src []byte) []dialects.Span { return nil }
func (dialect) Detect(root fs.FS) bool          { return false }
