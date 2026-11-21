# engine/export-theme

| | |
|---|---|
| Status | planned |
| Target version | 0.7.0 |
| Branch | `feature/export-theme` |
| Wave | 44 |
| Depends on | engine/export-key (`export:`), ui/site-topbar (the site shell), core/static-build, engine/runtime (tokens.css, static.css) |
| Owns | `engine/config/**` (`export.theme`), `engine/site/**` (the head, shell, assets and footer), `runtime/src/static/**`, `runtime/src/css/tokens.css` (only to name the tokens a theme may set), new `builtins/themes/**` (the built-in site themes), Settings › Export › Website (a Theme part, in `apps/zenmark/frontend/src/shell/{buildsite,exportform}.ts`) + tests, `engine/session/buildui.go` (saving it), `docs/THEMES.md` (new), `docs/BUILD.md`, the demo's Export page and a sample theme in `examples/showcase` |

## Purpose

An exported website looks exactly like the app: black and white, and the
app's fonts. That's right for the app, but a site is published under someone
else's name. The person exporting should be able to give it their own look
without touching the generated files:
- colours and fonts;
- their logo and favicon;
- a footer;
- a stylesheet of their own;
- a choice of built-in site themes.

This applies to the HTML export, and to the PDF where it makes sense. The app
stays black and white: these settings only change exports.

## Behaviour

1. **`export.theme` in `.zenmd.yml` or `config.yml`:**
   ```yaml
   export:
     theme:
       name: default            # a built-in site theme, or a folder theme in .zen/themes/<id>/
       scheme: auto             # auto (follows the reader's system), light or dark
       tokens:                  # design tokens, applied to light (and to dark unless dark: is given)
         accent: "#0b5fff"
         fg: "#1a1a1a"
         font-body: "Georgia, serif"
         font-ui: "Inter, system-ui, sans-serif"
         radius: 8px
       dark:                    # the same keys, for the dark scheme
         accent: "#7aa7ff"
       css: [theme/site.css]    # stylesheets of the folder, loaded last, in order
       logo: images/logo.svg    # the sidebar and header logo (defaults to the folder's logo:)
       favicon: images/icon.png # .png, .svg or .ico
       footer: "© 2026 Ada Lovelace · [Source](https://example.com)"   # one line of markdown, links allowed
       social: images/card.png  # the og:image for link previews
   ```
2. **Tokens.**
   - **Which tokens:** only the documented ones can be set: colours (`bg`, `bg-subtle`, `fg`, `fg-muted`, `rule`, `accent`, `focus`, the button colours), fonts (`font-body`, `font-ui`, `font-mono`), `radius`, and the reading `measure` (line width).
   - **`accent`** is new, and on a site only. Links, the current sidebar entry and primary buttons use it. In the app and in the default theme it equals `fg`, so nothing changes there.
   - **Checks:** each value is validated, colours as CSS colours and sizes as lengths. A bad value is a page-level export warning, and the default is kept. Values are written as CSS custom properties in a generated `theme.css`. They are never pasted into HTML.
   - **Dark:** `dark:` gives the dark scheme's values. Without it, the light values are used for both.
3. **Themes.**
   - **Built-in themes:**
     - `default`: today's look;
     - `paper`: a warm off-white and a serif UI;
     - `docs`: a blue accent, sans-serif throughout and a wider measure;
     - `mono`: a monospace UI.

     Each one is only a set of token values plus at most one small stylesheet.
   - **Folder themes:** `.zen/themes/<id>/theme.yml`, giving `tokens`, `dark` and `css`. A folder theme is trusted like a folder layout: it is allowed in the app first, and a build without that permission uses `default` with a warning. Theme CSS can't run code, but it can load fonts and images, so the files are copied into the site as assets.
   - **Order of the stylesheets:** zen.css, then the theme's CSS, then the generated token CSS, then `css:` from the config. The user's own stylesheet always wins.
4. **Scheme.** `auto` follows `prefers-color-scheme`, as today. `light` and `dark` force one scheme, which also means the site has no switch. A light/dark switch in the top bar is part of the theme, shown with `scheme: auto` and `switch: true`, and remembered per site.
5. **Branding.**
   - **Logo:** `logo` is shown beside the site's title in the sidebar header, as in the app.
   - **Favicon and preview image:** `favicon` and `social` are copied and linked in every page's head, with `og:title`, `og:description` (the page's first paragraph) and `og:image`.
   - **Footer:** `footer` is rendered as one line of markdown at the bottom of every page. Links and emphasis are allowed. HTML is not, and it goes through the same sanitiser as page text.
   - **Paths:** every path must be inside the folder. The checks are the same as for `logo:`, so there is no `..`, no absolute path and no symlink out.
6. **PDF.** The PDF export uses the theme's fonts and its `accent` for headings and links, but it is always on white paper (light scheme). It ignores `css:` unless `pdf: { css: true }`. The cover shows `logo`.
7. **Settings › Export › Website** gets a **Theme** part, using the same form pieces:
   - a theme picker (the built-ins and any allowed folder themes);
   - Scheme (Auto, Light or Dark);
   - colour fields for Accent, Text and Background, as text fields with a swatch;
   - fields for the body and UI fonts;
   - pickers for the logo, favicon and footer;
   - "Your stylesheet", a folder file picker.

   **Preview** opens the exported home page in the browser after a quick export into a temporary folder. It never touches `exports/html`. **Save as folder defaults** writes `export.theme`.
8. **No theme, no change.** A folder without `export.theme` exports exactly as today. The golden build stays byte-identical unless a theme is set.

## Interfaces

```go
// engine/config
type Theme struct {
	Name, Scheme  string
	Tokens, Dark  map[string]string
	CSS           []string
	Logo, Favicon, Footer, Social string
	Switch        bool
}
func (c *Config) Theme() (Theme, []string) // the theme and its warnings
```

`docs/THEMES.md` lists the tokens, the built-in themes, `theme.yml` and the stylesheet order. That list is the contract for theme authors.

## Tests

| # | Behaviour | Test |
|---|---|---|
| 1 | `export.theme` parsed, merged (folder over global), bad values warned and dropped | `engine/config/theme_test.go` |
| 2 | Tokens become CSS custom properties in theme.css, light and dark; never in HTML attributes | `engine/site/theme_test.go` |
| 3 | Stylesheet order; folder `css:` and theme assets copied; paths outside the folder refused | `engine/site/theme_test.go` |
| 4 | Built-in themes load; folder theme needs trust, else default with a warning | `builtins/themes` tests, `engine/site` |
| 5 | Logo, favicon, og tags and the sanitised footer | `engine/site/shell_test.go` |
| 6 | Scheme light, dark and auto; the switch shown only with `auto` + `switch` and remembered | `runtime/src/static/static.test.ts` |
| 7 | The PDF uses the fonts and accent, on white | `apps/zenmark` print tests, `kit/ui/src/print` tests |
| 8 | No theme, byte-identical output | `TestDemoBuildUnchanged` unchanged |
| 9 | Settings Theme part: fields, Preview into a temporary folder, Save as folder defaults | `buildsite.test.ts`, `buildui_test.go` |

## UI

UI consistency: the Theme part of Settings › Export › Website uses the export form's pieces and the Settings frame. Colour fields are text fields with a swatch, not a new kind of control. The app itself stays black and white: themes only change what is exported.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| CSS or token injection into HTML | Tokens validated and written only into a CSS file as custom properties; footer sanitised like page markdown | #2, #5 |
| Paths that read outside the folder | Same checks as `logo:` (no `..`, no absolute paths, no symlinks out) | #3 |
| A folder theme bringing remote resources | Folder themes need trust as layouts do; the CSS is copied as is, so a `@import url(https://…)` is warned about in the export result | #4 |

## Notes

- **Phases, if it is too big for one wave:**
  1. Tokens, scheme, logo, favicon and footer in config, with the export.
  2. Built-in themes and folder themes.
  3. The Settings Theme part and Preview.
  4. The PDF.
- **Out of scope:** replacing the site's HTML shell with custom templates (layouts already cover pages); JavaScript in themes; per-page themes.
- **Changelog** (added by the lead): `Added: export.theme gives an exported website its own look (built-in themes, colours, fonts, logo, favicon, footer, your own stylesheet), set in .zenmd.yml or Settings › Export; the app stays black and white.`

## Prompt

> Implement `specs/engine/export-theme/spec.md` (one phase per run if split) following CLAUDE.md and docs/UI.md, in `../zen-worktrees/export-theme` on `feature/export-theme`. Tests first, committed failing. Only the Owns paths. Do not launch the app, run task package/install/release, push or touch main.
