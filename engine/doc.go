// Package engine is the zenmd engine: everything that turns a folder of
// markdown into pages and a website, with no window, menu or dialog. It holds
// no code of its own; the packages under it do the work, and this page is the
// map.
//
// Start with session. It opens a folder and renders, saves, builds and prints
// it; most programs need nothing else:
//
//	s := session.New(session.Options{Runtime: dist})
//	defer s.Close()
//	if err := s.Open("docs"); err != nil { ... }
//	page, _ := s.Page(s.Home())
//
// The packages session is built on are public too, for programs that need
// only a part:
//
//   - session: the headless folder session (open, page, preview, save, watch,
//     trust, build, print, site templates, an http.Handler for plugin assets)
//   - workspace: scanning a folder for markdown, and page paths
//   - config: the global and per-folder config files (.zen.yml)
//   - index: titles, headings, links and search over a workspace
//   - render: markdown to sanitised HTML, with frontmatter, directives and plugin blocks
//   - plugins: the plugin API and registry (built-ins are in package builtins)
//   - extplugins: external plugins and the trust store that guards them
//   - scriptplugins: one-file Tengo plugins, run in a helper process
//   - layouts: the page layout API and registry
//   - site: the static site builder
//   - sitetemplates: starter sites ("zenmd init")
//   - lint: the prose linter (Vale)
//   - watch: file change events for a live preview
//
// The default plugins, layouts and site templates live in package builtins.
//
// # Safety
//
// session is the front door: it keeps the sanitiser on, asks a Prompter
// before it runs a folder's own plugins and layouts, and never writes the
// trust store without one. A program that calls render or extplugins
// directly must keep those checks itself; see the docs of those packages.
//
// # Compatibility
//
// The engine follows Semantic Versioning (docs/VERSIONING.md). Before 1.0 a
// breaking change to an exported name bumps the minor version; CI runs
// apidiff against the latest tag and fails an undeclared break.
package engine
