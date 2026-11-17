# core/layouts

| | |
|---|---|
| Status | specified |
| Target version | 0.2.0 |
| Branch | `feature/core-layouts` |
| Depends on | core/script-plugins, core/plugin-hardening |
| Owns | `internal/core/layouts/` (new), `internal/layouts/` (new; built-ins move here), `internal/plugins/{slides,course}` (removed), `internal/core/plugins` (drop layouts), `internal/core/extplugins` + `internal/core/scriptplugins` (refuse `layouts`, discover layout folders), `internal/core/render` (layout step), `internal/core/config` (folder default layout), `internal/app` (services, assets, trust wiring), `frontend/src/{api,page,plugins,shell}` (layout runtime, Settings › Layouts, New Page picker), docs, `examples/demo/layouts/**`, `examples/demo/.zen/layouts/**` |

## Why

A **plugin** is a *part* of a page: a block, an inline, a transformation.
A **layout** is the *look and feel of the whole page*: its structure,
typography, chrome and behaviour. Today a layout is just a plugin that lists
`layouts:`; that mixes the two (a plugin folder can take over every page,
Settings shows slides next to code colours, layouts cannot be shared without
writing a plugin). This spec makes layouts their own concept with their own
folder, definition, registry, settings and help — and adds **template
layouts** that anyone can write with HTML and CSS.

## Behaviour

### Choosing a layout
1. A page picks its layout in frontmatter: `layout: post`, options under `layoutOptions:` (unchanged) — **or** the new short form `layout: { name: post, cover: true }` (string values only, as today).
2. A folder can set defaults in `.zen.yml` (and the global `config.yml`):
   ```yaml
   layout: docs                  # every page without its own
   layouts:                      # by path, first match wins (glob: *, **)
     "blog/**": post
     "talks/*.md": slides
   ```
   Precedence: page frontmatter > `layouts:` pattern > `layout:` > the built-in `article` (today's reading view).
3. Unknown or blocked layout → warning on the page, rendered with `article`.

### What a layout is
4. **Built-in layouts** live in `internal/layouts/<id>` (Go, embedded assets):
   - `article` — the current reading column (explicit, no change in look),
   - `slides` and `course` — **moved unchanged** from `internal/plugins/` (same names, same options, same behaviour, same storage keys, same tests);
   - plus the template layouts below.
5. **Folder and user layouts**: `<folder>/.zen/layouts/<id>/` and `<user config dir>/zen/layouts/<id>/` with a `layout.yml`:
   ```yaml
   id: post
   version: 0.1.0
   description: Blog post with cover, date and author
   template: layout.html        # optional: page structure
   css: [post.css]              # optional
   js: [post.js]                # optional: export default { mount(root, ctx) }
   fields:                      # frontmatter the template reads (documented, checked)
     date:   { type: date, description: Publication date }
     author: { type: string }
     cover:  { type: image, description: Workspace image shown on top }
     tags:   { type: list }
   options:                     # layoutOptions, like plugin options
     toc: { type: bool, default: true }
   help:                        # same shape as plugin help
     summary: …
     snippets: [{ title: A post, markdown: "---\nlayout: post\ndate: 2026-11-17\n---\n# Title" }]
   ```
   A layout may have only CSS (restyle the article), only a template, only JS, or any mix.
6. **Templates** are rendered in Go with `html/template` (auto-escaping), from a fixed data model, then sanitised with the base policy plus `data-zen-layout-*` attributes. Data: `.Content` (the rendered page body, already sanitised, inserted as trusted HTML), `.Title`, `.Path`, `.Breadcrumb` [{title, path}], `.TOC` [{level, id, text}] (h2–h4), `.Prev` / `.Next` (from the sidebar order, may be nil), `.Field "name"` (frontmatter fields declared in `fields`, typed: dates formatted with `{{date .}}`, images resolved to `/_ws/` URLs), `.Option "name"`, `.Children` (pages under this one in the sidebar, for index/landing pages), `.Words`, `.ReadingTime`. Functions: `date`, `upper`, `lower`, `join`, `markdown` (renders a frontmatter string field). No file access, no other functions; a template that fails to parse or execute → warning, `article` used.
7. **Layout scripts** run through the existing plugin runtime as modules with a default `{ mount(root, ctx) }`, bound to the layout id (`ctx.kind === "layout"`), with `ctx.storage` (per layout and folder) — no backend, no `invoke`/`stream` (layouts have no programs).
8. **Assets** served at `/_layouts/<id>/<file>?v=<hash>`, same rules as plugin assets (listed files only, symlinks inside, changed files re-prompt).
9. **Trust**: layouts go through the same discovery, hashing, fingerprint, per-workspace approval and `layouts:changed` flow. Prompt wording: CSS/template only → "changes how pages look; runs nothing"; with JS → "… and runs a page script". Kind `layout` in the list.
10. **Plugins can no longer declare layouts**: `layouts:` in `plugin.yml` or a Tengo `plugin` map is an error naming the new place (`.zen/layouts/<id>/layout.yml`). `plugins.Manifest.Layouts`, `Registry.Layout`, `Set.Layout` are removed.
11. **Services**: `Layouts()` → `[{id, version, description, source: builtin|folder|user, status, kind: "layout", hasTemplate, hasScript, fields[], options[], help}]`; `LayoutTrust(id, allow)` (native dialog for allow, like plugins). `Page` gains `layout: {name, source}` (replacing the string; frontend updated).
12. **Settings › Layouts** section next to Plugins: list, help pane with snippets and a static preview (built-ins), Allow/Block for external ones. **File › New Page…** lists layouts (built-in + folder) with their description; choosing one inserts its first help snippet's frontmatter.
13. **Live reload**: editing `layout.html`/`.css` re-renders open pages (after the trust re-check).

## Template layouts to ship (built-in)

| id | for | template shows |
|---|---|---|
| `article` | default reading view | unchanged |
| `docs` | documentation pages | title, on-page TOC rail on the right (h2–h3), Prev / Next footer from the sidebar order, "Edit this page" |
| `post` | blog posts, notes | cover image, title, date · author · reading time, tags, body, Prev / Next |
| `landing` | a folder's home | hero (title + `tagline` field + optional cover), then cards for `.Children` (title + first paragraph) |
| `wide` | tables, diagrams, dashboards | full pane width, no reading column |
| `handout` | printable one-pagers | A4/Letter page frame on screen, print CSS, header/footer with title, date and page numbers |

(Ideas for later: docs/possible-layouts.md.)

## Tests

- Go: layout manifest parsing and refusals (every key, `fields`/`options` types, template file, assets inside); refusal of `layouts` in plugin.yml/Tengo with the new hint; precedence (frontmatter > pattern > default > article) incl. globs; template rendering with each data item, escaping (a field containing `<script>` stays text), `markdown` func, parse/exec errors → warning + article; sanitising of template output; assets and `?v=`; trust flow for layouts (prompt wording, re-prompt after editing `layout.html`); `Layouts()` JSON; slides and course behave exactly as before (their existing tests moved, unchanged); each built-in template layout renders the demo page for it without warnings.
- Frontend: layout runtime mount/unmount, storage bound to the layout, no invoke/stream on layout ctx; Settings › Layouts; New Page picker; Page.layout shape.
- Demo: `examples/demo/layouts/` gets a page per new built-in layout and a "Writing your own layout" how-to; `examples/demo/.zen/layouts/memo/` is a sample folder layout (template + CSS, fields `to`, `from`, `date`); `blog/` uses a `layouts:` pattern in the demo `.zen.yml`. `TestDemoFolder` covers them; Vale clean.

## Security

| Risk | Mitigation |
|---|---|
| Template injecting script | `html/template` escaping, fixed data model, output sanitised; only `.Content` is trusted (already sanitised) |
| Layout taking over every page of a downloaded folder | folder layouts need approval like plugins; patterns in `.zen.yml` only apply approved layouts |
| Layout script reaching programs | layouts have no backend; ctx has no invoke/stream |
