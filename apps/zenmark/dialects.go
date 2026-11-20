package app

import "github.com/zenmark-org/zenmd/engine/session"

// DialectInfo is one markdown dialect of this build for Settings › Markdown:
// its id, name, one-line summary and whether the open folder uses it.
type DialectInfo = session.DialectInfo

// Dialects lists the markdown dialects of this build with the open folder's choice.
func (z *Zen) Dialects() []DialectInfo { return z.s.Dialects() }

// SetDialect turns a dialect on or off for the open folder (its .zenmd.yml).
func (z *Zen) SetDialect(id string, on bool) error { return z.s.SetDialect(id, on) }

// DismissDialectHint hides the "turn on MyST?" hint for the open folder; the
// choice is kept in the app's settings, not in the folder.
func (z *Zen) DismissDialectHint(id string) error { return z.s.DismissDialectHint(id) }

// DialectHint is the dialect the open folder looks written for (a myst.yml, a
// Jupyter Book _config.yml, a Sphinx conf.py) and has not turned on or
// dismissed; "" for none, and always "" in a read-only app. Startup.DialectHint
// says the same when a folder opens.
func (z *Zen) DialectHint() string { return z.s.DialectHint() }
