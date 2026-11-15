# core/navmeta

| | |
|---|---|
| Status | specified |
| Target version | 0.1.0-alpha.2 |
| Branch | `feature/core-navmeta` |
| Wave | 5 |
| Depends on | core/index, core/render, core/config, app/services |
| Owns | `internal/core/index/`, `internal/app/state.go` (reindex, title/meta collection), `internal/app/slugs.go` (new) |

## Purpose

A page can tune its place in the navigation from its frontmatter. Without
these keys the link-based tree stays exactly as it is today.

## Behaviour

Frontmatter keys (next to `title`, `plugins`):

```yaml
slug: install                 # extra link target
nav:
  title: Setup                # label in sidebar and breadcrumb
  order: 2                    # position among siblings
  parent: guides/index.md     # place under this page (path or slug)
  hidden: true                # not in the sidebar
```

1. **Default unchanged.** Pages without `slug`/`nav` produce the same tree as today (existing index tests keep passing).
2. **`nav.title`** replaces the label in the sidebar and breadcrumb. The page heading and `Page.title` are unchanged. Non-string or empty → ignored with a warning.
3. **`nav.order`** (integer) sorts siblings: pages with an order come first, ascending; ties and pages without an order keep their link/document order.
4. **`nav.parent`** (workspace path or slug) places the page under that page, overriding the link-based parent. A missing parent, the page itself, or a parent that would create a cycle → ignored with a warning, link-based placement kept. A parent that is hidden makes the child hidden too.
5. **`nav.hidden: true`** leaves the page out of the sidebar (and its children unless they set their own visible parent). The page is still reachable by links and gets a breadcrumb as if it were not hidden.
6. **`slug`** (`[a-z0-9][a-z0-9-]*(/[a-z0-9][a-z0-9-]*)*`, max 100 chars) is an extra link target: a link `[x](install)`, `[x](/install)` or `[x](install#part)` that is otherwise broken resolves to that page. Real files always win over slugs. Duplicate slugs: the first page by path keeps it, others get a warning. Invalid slug → warning, ignored.
7. Warnings appear in `Page.warnings` of the page that has the problem.
8. Changing frontmatter re-indexes on save/watch like today and emits `workspace:changed` when the tree changes.

## Architecture

`core/index` gains page metadata: `type Meta struct{ Title string; Order *int; Parent string; Hidden bool }` and
`BuildWithMeta(files, home, links, titles, meta func(path) Meta) *Index`; `Build` stays and calls it with no meta.
Hidden nodes are kept internally for breadcrumbs but left out of `Tree()`.
`internal/app/state.go` reads frontmatter during reindex (it already renders each page for its title), builds a
slug map (`slugs.go`), and wraps the workspace link resolver so `render` and the index see slug links as internal.

## Interfaces

```go
package index
type Meta struct { Title string; Order *int; Parent string; Hidden bool }
func BuildWithMeta(files []string, home string, links func(string) []string, titles func(string) string, meta func(string) Meta) *Index
func ParseMeta(frontmatter map[string]any) (Meta, slug string, warnings []string)
```

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | no meta = identical tree | `internal/core/index/meta_test.go: TestNoMetaUnchanged` |
| 2 | nav title in tree and breadcrumb | `TestNavTitle` |
| 3 | order among siblings, ties stable | `TestOrder` |
| 4 | parent override, missing, self, cycle, hidden parent | `TestParent` |
| 5 | hidden out of tree, breadcrumb kept | `TestHidden` |
| 2–6 | ParseMeta: types, invalid values, slug pattern | `TestParseMeta` |
| 6 | slug links resolve, files win, duplicates, anchors | `internal/app/slugs_test.go: TestSlugLinks` |
| 7,8 | warnings on page, reindex on frontmatter change | `internal/app/state_test.go` (added cases) |

## UI

None new: the sidebar and breadcrumb show what the index returns.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| Slug used to escape the workspace | slugs map only to indexed workspace files; pattern forbids `..`, `/` at start, schemes | `TestParseMeta`, `TestSlugLinks` |
| Cycles hanging the index | cycle detection on parent overrides | `TestParent` |

## Notes

- **Out of scope:** slugs as window URLs, renaming files, a nav editor UI.
- **Integration (lead):** document keys in docs/PLUGINS.md "Enabling, disabling, configuring" (frontmatter section).
- **Changelog:** Added: frontmatter `slug` and `nav` (title, order, parent, hidden).

## Prompt

> Implement `specs/core/navmeta/spec.md` following the rules in `specs/README.md`.
> Read `internal/core/index`, `internal/app/state.go` and `internal/core/render` first.
> Keep `index.Build` and all existing tests working unchanged. Another agent edits
> `internal/app/app.go`, `windows.go` and `service.go` in parallel: do not touch them.
