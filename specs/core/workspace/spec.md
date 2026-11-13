# core/workspace

| | |
|---|---|
| Status | specified |
| Target version | 0.1.0-alpha.1 |
| Branch | `feature/core-workspace` |
| Wave | 1 |
| Depends on | none |
| Owns | `internal/core/workspace/` |

## Purpose

Represents the opened folder and is the only code that touches its files, so
every other feature gets safe, workspace-relative file access.

## Behaviour

1. `Open(dir)` accepts an existing directory (absolute or relative, symlinks resolved) and returns a `Workspace`; a file path or missing path is an error.
2. `Name()` is the folder's base name; `Root()` the absolute, symlink-resolved path.
3. `Files()` returns every `*.md` / `*.markdown` file (case-insensitive extension) as sorted, `/`-separated relative paths. It skips directories starting with `.`, `node_modules`, `vendor`, and symlinks that leave the root.
4. `Home()` returns `README.md` (any case, root only) if present, else `""`.
5. `Resolve(rel)` returns the absolute path for a relative path, or `ErrOutside` if the cleaned, symlink-resolved result is not under the root. Absolute inputs, `..` escapes and NUL bytes are rejected.
6. `Read(rel)` returns file bytes; files larger than 10 MB return `ErrTooLarge`.
7. `Write(rel, data)` writes atomically (temp file in the same directory + rename), keeps the file mode, only for existing or new `.md`/`.markdown` files inside the root.
8. `Asset(rel)` opens a file for serving only if its extension is one of png, jpg, jpeg, gif, webp, svg, avif.
9. `ResolveLink(from, href)` resolves a link found in page `from` to `(relPath, anchor, kind)` where kind is `internal` (existing markdown file), `asset`, `external` (has a scheme or starts with `//`), or `broken`. A link to a directory resolves to its `README.md`; a link without extension tries `.md`.

## Architecture

Pure Go, standard library only. No Wails import. Used by `core/index`,
`core/watch`, `core/render` (through an interface) and `app/services`.

## Interfaces

```go
package workspace

var ErrOutside, ErrTooLarge, ErrNotMarkdown, ErrNotAsset error

type LinkKind string // "internal" | "asset" | "external" | "broken"
type Link struct{ Path, Anchor string; Kind LinkKind }

type Workspace struct{ /* unexported */ }
func Open(dir string) (*Workspace, error)
func (w *Workspace) Root() string
func (w *Workspace) Name() string
func (w *Workspace) Files() ([]string, error)
func (w *Workspace) Home() string
func (w *Workspace) Resolve(rel string) (string, error)
func (w *Workspace) Read(rel string) ([]byte, error)
func (w *Workspace) Write(rel string, data []byte) error
func (w *Workspace) Asset(rel string) (*os.File, error)
func (w *Workspace) ResolveLink(from, href string) Link
```

## Tests

| # | Behaviour | Test (`internal/core/workspace/workspace_test.go`) |
|---|---|---|
| 1,2 | open dir / reject file / missing | `TestOpen` |
| 3 | listing, skips, sorting | `TestFiles` |
| 4 | home detection, case | `TestHome` |
| 5 | traversal, absolute, symlink escape | `TestResolveRejectsEscapes` |
| 6 | read, size limit | `TestRead` |
| 7 | atomic write, mode kept, non-markdown refused | `TestWrite` |
| 8 | asset allow-list | `TestAsset` |
| 9 | link kinds, anchors, directory and extensionless links, URL-encoded paths | `TestResolveLink` |

Edge cases: unicode and spaces in names, `%20` in links, `./` and `../` inside the root, empty folder.

## UI

None.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| Path traversal (`../../etc/passwd`) | `Resolve` cleans, resolves symlinks, checks prefix with separator | `TestResolveRejectsEscapes` |
| Symlink pointing outside | resolved before the check; skipped in `Files` | same |
| Writing arbitrary files | `Write` limited to markdown extensions inside root | `TestWrite` |
| Serving arbitrary files to the webview | `Asset` extension allow-list | `TestAsset` |

## Notes

- **Out of scope:** watching, recent folders, creating/deleting/renaming files.
- **Integration:** none.
- **App Store / sandbox:** the root must come from a user action (dialog, CLI argument); bookmark persistence is a beta spec.
- **Changelog:** Added: open a folder as a workspace with safe file access.

## Prompt

> Implement `specs/core/workspace/spec.md` following the rules in `specs/README.md`.
> Create package `internal/core/workspace` with exactly the interface above, standard
> library only. Use `t.TempDir()` fixtures. Done when every test in the table exists and
> passes, and `go vet ./... && go test ./...` is clean.
