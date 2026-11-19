# zenmd export — static websites and PDF

`zenmd export site` (also `zenmd build`) exports a folder as a static website, and `zenmd export pdf` as one PDF file (see the end of this page). Every page is rendered with
the same Go pipeline as the app (plugins, layouts, sidebar, breadcrumb), then
written as plain HTML with its assets next to it. Spec:
[specs/core/static-build](../specs/core/static-build/spec.md). User guide:
[examples/demo/build/index.md](../examples/demo/build/index.md).

```sh
zenmd export site [folder] [-o dir] [--hosted] [--drafts] [--clean]
```

| Option | Meaning |
|---|---|
| `-o`, `--out` | output folder (default `build.out`, else `<folder>/_site`) |
| *(default)* | portable: opens from disk (`file://`) and from any web server. Scripts are embedded in each page and loaded as Blob modules |
| `--hosted` | for a web server only: scripts stay separate files loaded with `import()`, which browsers refuse from `file://` (each block then says so) |
| `--portable` | the default, named explicitly (cannot be combined with `--hosted`) |
| `--drafts` | include pages with `draft: true` |
| `--no-print` | leave out `print.html`, the whole site as one printable page |
| `--clean` | empty the output first (only an empty folder or an earlier build, marked `.zen-build`) |

Exit codes: 0 built (warnings on stderr), 1 error, 2 usage.

## In the app

File › Export › Website… (⇧⌘B) or Settings › Export builds the open folder with
the same code: the options above, a progress bar with Cancel, the warnings by
page, then Open in Browser and Show in Finder. "Save as folder defaults" writes
the options to `.zen.yml` after showing the change.

## Output

```
_site/
  index.html              home page (or a redirect to it)
  print.html              the whole site as one document: cover, contents, every page
  README.html, a/b.html   one file per page, same paths
  img/…                   workspace files the pages use (/_ws/…)
  _zen/zen.css, zen.js    zen's page styles and the static runtime
  _zen/plugins/<id>/…     plugin files the pages use
  _zen/layouts/<id>/…     layout files the pages use
  .zen-build              marker for --clean
```

The build is deterministic: there are no timestamps or absolute paths, so
the same input gives the same bytes.

## Plugins and layouts

- **Render-time plugins** (built-in Go plugins, Tengo plugins, external backends that return HTML) run during the build. Their output is static HTML.
- **Page scripts** use the same contract as in the app (`export default { mount(el, ctx) }`):
  - `ctx.storage` is kept in the reader's `localStorage` under `zen.site.<id>.<plugin|layout>.<id>.<key>`;
  - `ctx.invoke` and `ctx.stream` reject with "runs in the zen app only".
- **Trust is read-only.** Only plugins and layouts already allowed for this folder in the app are used, re-checked by hash. Anything else is left out with a warning. A build never prompts.
- Plugin options with `scope: user` (tokens, keys) are never passed to plugins during a build.

## Security

Every page has a CSP meta tag:
- `default-src 'self'`, with `script-src 'self'` (portable output, the default, adds `file: blob:`);
- `object-src 'none'`;
- `base-uri 'none'`.

Links are rewritten with an HTML tokenizer: internal links become relative
`.html` links, external links open in a new tab, and broken links stay muted.
Only files inside the folder are copied.

## Configuration

```yaml
# .zen.yml or the global config.yml; flags win
build:
  out: _site
  id: my-notes          # storage namespace for readers; default: folder name
  editURL: https://github.com/me/notes/edit/main/{path}   # the docs layout's "Edit this page"
  drafts: false
  portable: true        # false = --hosted
  exclude: ["private/**"]
```

## Add-ons

`ADDONS` chooses the optional add-ons of a build (docs/ADDONS.md,
specs/kit/addons): `task build|package|run|dev|release ADDONS=vale`. Each is
compiled in with the tag `addon_<id>`, and its helpers are bundled only when it
is in the build. The release workflow uses the repository variable `ADDONS`;
without it the DMG has no add-ons. `zenmd version` lists the add-ons of a build.

## Developing

The static runtime is `frontend/src/static/` and its own Vite config
(`src/static/vite.static.config.ts`). `npm run build` builds it into
`frontend/dist/static/` after the app bundle, and `main.go` hands
`frontend/dist` to `app.StaticAssets`.

## PDF

`zenmd export pdf [folder] [-o file.pdf]` writes the whole folder as one PDF:
the print document (cover, contents, every page in sidebar order) laid out by
macOS printing, portrait, with the system's default paper size. It runs without
showing a window, needs macOS and a logged-in session, and prints
`Wrote <file> (<n> pages)`. In the app, File › Export › PDF… (⇧⌘P) opens the
preview, where **Export PDF…** saves the file. Spec: `specs/app/export/spec.md`.

