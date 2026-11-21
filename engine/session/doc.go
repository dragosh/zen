// Package session is the headless folder session of zenmd: everything that
// turns a folder of markdown into pages, with no window, menu or dialog.
//
// A Session opens a folder (with progress, cancel, a timeout and rollback),
// keeps its index and sidebar tree, renders pages and previews, saves and
// creates pages, runs external and script plugins and layouts (asking the
// user through a Prompter before it trusts a folder's own), serves the files
// of the open folder and of plugins and layouts as an http.Handler, builds a
// static site, makes the print document and lists site templates. It reports
// changes as events through an Events sink.
//
// The Zenmark app (apps/zenmark) is a thin adapter over it: it binds the
// session's methods to its frontend, forwards the events to Wails and
// implements Prompter with native dialogs. A test, the command line or
// another app can use the same session without any GUI.
//
//	s := session.New(session.Options{Runtime: dist})
//	defer s.Close()
//	if err := s.Open("examples/showcase"); err != nil { ... }
//	page, _ := s.Page(s.Home())
//	res, _ := s.Build(ctx, session.BuildOptions{Out: "site", Portable: true}, nil)
//
// # Events and questions
//
// An Events sink and a Prompter are both optional. Without a sink nothing is
// reported; without a prompter the session never asks and never decides: a
// folder's plugins and layouts stay undecided, and are not run, until a trust
// decision is recorded.
//
// # Several sessions
//
// Sessions share no package-level state of their own, so a process may hold
// several, one folder each. What is shared on purpose or by the packages the
// session builds on: the Vale linter process limit (engine/lint), the
// script plugin helper pool and its crash counts (engine/scriptplugins:
// closing a session or switching its folder retires the idle helpers of all
// sessions), and the per-file storage locks.
//
// # Compatibility
//
// The JSON types (Workspace, Page, PluginInfo, LayoutInfo, BuildOptions, ...)
// and the event names are the contract with the Zenmark frontend; they do not
// change without a frontend change.
package session
