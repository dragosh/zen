# ui/page

| | |
|---|---|
| Status | done |
| Target version | 0.1.0-alpha.1 |
| Branch | `feature/ui-page` |
| Wave | 2 |
| Depends on | ui/shell |
| Owns | `frontend/src/page/` |

## Purpose

Shows a rendered page with editorial typography and makes its links work.

## Behaviour

1. When `store.path` changes in `read` mode, loads `api.page(path)` and mounts `page.html` in `<article class="zen-page">` inside the `main` slot; sets `document.title` to `<title> — zen`; stores the page in the store (`page`).
2. Clicks on `a[data-zen-link="internal"]` call `navigate(data-path, data-anchor)`; `external` calls `api.openExternal(href)` and never navigates the webview; `broken` does nothing. Modifier-clicks and middle clicks behave the same (no new windows). Any other `<a href>` with a scheme is treated as external.
3. After mounting, scrolls to the anchor if given (element id), else to the top; same-page `#anchor` links scroll smoothly (instantly with reduced motion) and push history.
4. `page.assets.css` / `.js` are loaded once per URL (`<link>` / `<script type="module">` in `<head>`), only URLs starting with `/_plugins/`; after mount and after every re-render a `zen:page-rendered` CustomEvent (`detail: {root}`) is dispatched on `document` so plugin scripts can enhance their blocks.
5. Loading: previous page stays; a progress line appears under the header after 150 ms. A slower earlier request never overwrites a newer one.
6. Error: message and Retry button in the page area.
7. `page:changed` for the current path re-renders in place and keeps the scroll position; for another path it does nothing.
8. `page.warnings` are shown in a dismissible muted bar above the article.
9. The header `actions` slot gets an "Edit" button (⌘E) that sets `mode: "edit"`.
10. `page/page.css` styles every element the renderer can emit (headings, lists, task lists, tables, code, blockquote, footnotes, images max-width 100%, `hr`, `.zen-plugin-error`, broken links) per docs/UI.md.

## Architecture

`page/index.ts` exports the `Feature` (`id: "page"`); `page/links.ts`,
`page/assets.ts`, `page/page.css`. HTML is trusted only because it comes from
`api.page()` (sanitised in Go); this is the single place `innerHTML` is used.

## Interfaces

```ts
export const pageFeature: Feature;
export function mountPage(el: HTMLElement, page: Page): void;
export function handleLinkClick(e: MouseEvent, ctx: Ctx): void;
```

## Tests

| # | Behaviour | Test (`frontend/src/page/*.test.ts`) |
|---|---|---|
| 1 | load, mount, title, store | `page.test.ts` |
| 2 | internal / external / broken / modifier clicks | `links.test.ts` |
| 3 | anchors | `page.test.ts` |
| 4 | assets loaded once, origin check, event fired | `assets.test.ts` |
| 5 | request race | `page.test.ts: latest request wins` |
| 6 | error and retry | `page.test.ts` |
| 7 | live reload keeps scroll | `page.test.ts` |
| 8,9 | warnings bar, Edit button | `page.test.ts` |
| 10 | every listed element has a rule | `css.test.ts` (parses page.css for selectors) |

## UI

docs/UI.md "Reader": page column 68ch centred, serif body, states. Print
stylesheet is out of scope.

## Security

| Risk | Mitigation | Test |
|---|---|---|
| Webview navigating away to a remote site | every anchor click is intercepted; only `openExternal` handles URLs | `links.test.ts` |
| Loading script from a page-controlled URL | assets only from `/_plugins/` prefix | `assets.test.ts` |
| `innerHTML` with unsanitised text | only `Page.html`; errors and warnings via `textContent` | `page.test.ts: error text is escaped` |

## Notes

- **Out of scope:** table of contents, zoom, find in page, image lightbox.
- **Integration (lead):** in `features.ts` replace the page placeholder with `pageFeature`; import `page/page.css`.
- **Changelog:** Added: page view with in-place link navigation and external link confirmation.

## Prompt

> Implement `specs/ui/page/spec.md` following the rules in `specs/README.md`. Build
> against the fake API (extend the sample pages in your own tests, not in `api/fake.ts`).
> PLAN.md "Rendered HTML conventions" defines the link markup. Tokens only, no new
> colours. Done when the tests in the table pass.
