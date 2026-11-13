# core/index

| | |
|---|---|
| Status | done |
| Target version | 0.1.0-alpha.1 |
| Branch | `feature/core-index` |
| Wave | 2 |
| Depends on | core/workspace |
| Owns | `internal/core/index/` |

## Purpose

Builds the sidebar navigation tree and breadcrumbs from the links between the
markdown files of a workspace.

## Behaviour

1. `Build(files, home, links, titles)` returns an `Index`. `links(path)` gives the internal link targets of a page in document order; `titles(path)` its title.
2. The tree is a breadth-first spanning tree of the link graph starting at `home`: a page becomes the child of the first page (in BFS order) that links to it; children keep document order. Each file appears exactly once. Self links, duplicate links and links to already placed pages add no node.
3. With no `home`, top-level nodes are the files of the root directory sorted by path, each expanded the same way.
4. Files not reachable are placed under one trailing group node `{kind: "group", title: "Other pages"}`, as a tree of folder groups and pages, sorted by path; titles there are the relative path without extension.
5. `Breadcrumb(path)` returns the tree path from the top-level node to the page (inclusive). Pages under "Other pages" get only themselves. Unknown path → empty.
6. Callers rebuild with `Build` and compare with `Equal` (deep comparison) to know whether the tree changed.
7. Paths are matched case-insensitively against `files` (macOS file systems are case-insensitive): a link to `readme.md` is the node for `README.md`, and the node uses the spelling from `files`.
8. Building 5 000 files with 50 000 links takes under 200 ms.

## Architecture

Pure Go, no parsing: link extraction is injected (app/services passes
`render.Renderer.Links`). Types mirror `NavNode` / `Crumb` in PLAN.md.

## Interfaces

```go
package index

type Node  struct{ Title, Path, Kind string; Children []Node } // Kind: "page" | "group"
type Crumb struct{ Title, Path string }

type Index struct{ /* unexported */ }
func Build(files []string, home string, links func(path string) []string, titles func(path string) string) *Index
func (i *Index) Tree() []Node
func (i *Index) Breadcrumb(path string) []Crumb
func (i *Index) Equal(other *Index) bool
```

## Tests

| # | Behaviour | Test (`internal/core/index/index_test.go`) |
|---|---|---|
| 2 | BFS tree, order, each file once, cycles | `TestTreeFromLinks` |
| 3 | no home | `TestNoHome` |
| 4 | unlinked files grouped by folder | `TestOtherPages` |
| 5 | breadcrumbs | `TestBreadcrumb` |
| 6 | equality | `TestEqual` |
| 7 | performance | `BenchmarkBuild` + `TestBuildLarge` |

Edge cases: empty workspace, a single file, links to missing files (ignored), README linking to itself, deep chains (1 000 levels, no recursion overflow).

## UI

None (rendered by ui/navigation).

## Security

None: operates on already validated workspace-relative paths; performs no I/O.

## Notes

- **Out of scope:** search, manual ordering (`nav:` in config is a beta spec), incremental updates.
- **Integration:** none.
- **Changelog:** Added: navigation tree built from links between pages.

## Prompt

> Implement `specs/core/index/spec.md` following the rules in `specs/README.md`. Pure
> functions over the injected `links` and `titles`; no file access, no imports from other
> core packages. Done when the tests in the table pass.
