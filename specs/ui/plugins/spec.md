# ui/plugins

| | |
|---|---|
| Status | specified |
| Target version | 0.2.0-alpha.1 |
| Branch | `feature/ui-plugins` |
| Wave | 6 |
| Depends on | ui/page, ui/shell, core/extplugins (contract only) |
| Owns | `frontend/src/plugins/` (new), `frontend/src/api/` (additions), `frontend/src/page/` (runtime + layout wiring), `frontend/src/shell/settings.ts` + its test and CSS (Plugins section), `frontend/src/main.ts`, `examples/demo/.zen/plugins/progress/`, `examples/demo/plugins/writing-plugins.md`, `examples/demo/plugins/index.md`, `examples/demo/plugins/directives.md`, `examples/demo/README.md` |

## Purpose

The page side of external plugins: the `zen` object plugin scripts use, mount
and clean-up of plugin blocks and layouts, the Plugins list in Settings, and
the how-to in the demo folder. Contract: docs/PLUGINS.md "Writing your own plugin".

## Behaviour

1. `window.zen = { register(id, { mount(el, ctx) }), version: 1 }` exists before any plugin script loads. Registering twice replaces the handler and remounts.
2. After every `zen:page-rendered` (first render, live reload, draft follow), for each `[data-zen-plugin]` wrapper inside the article whose id has a handler: build `ctx` (`id`, `kind` from `data-zen-layout` → `"layout"`, `span` → `"inline"`, else `"block"`; `name` from `data-zen-name` or the layout name; `attrs` from `data-zen-attrs`, `{}` when missing or invalid JSON; `page {path,title}`; `storage`; `invoke`; `navigate`) and call `mount`. Handlers registering later (script still loading) mount the wrappers already on the page. Before a re-render replaces the article, every clean-up function returned by `mount` (sync or async result) runs; errors in `mount` or clean-up are caught, logged, and shown as `.zen-plugin-error` in that wrapper only.
3. `ctx.storage.get/set` call `api.pluginStorageGet/Set` with JSON text; `get` resolves `null` for missing keys; values must be JSON-serialisable (else the promise rejects). `ctx.invoke(payload)` calls `api.pluginInvoke`. A plugin can only reach its own id.
4. Built-in plugin wrappers (highlight, diagram) are left alone unless something registers for them.
5. **Layouts.** `Page.layout` sets `article.dataset.layout` and class `zen-layout-<name>`; the reading column's max-width and padding are dropped for a layout page so the plugin controls the whole pane.
6. **API.** `Api` gains `plugins()`, `pluginTrust(id, allow)`, `pluginStorageGet(id, key)`, `pluginStorageSet(id, key, json)`, `pluginInvoke(id, json)`; `Page` gains `layout: string`. The fake keeps storage in memory, records calls, and lets tests set the plugin list and invoke answers.
7. **Settings › Plugins.** Under Configuration files, a list from `api.plugins()`: name, version, where it comes from (Built in / This folder / Your plugins), what it runs (`runs`, as text), status, and for external plugins an **Allow** or **Block** button calling `pluginTrust`, then refreshing. Invalid plugins show their `problem`. Empty state: "No plugins in this folder."
8. **Demo.** `examples/demo/.zen/plugins/progress/` — a frontend-only plugin (`blocks: [progress]`, `frontend: {js, css}`): a `:::progress{id=…}` container holding a markdown task list becomes a clickable checklist whose state persists with `ctx.storage` and shows "n of m done". `examples/demo/plugins/writing-plugins.md` is the how-to: the three parts, a walkthrough of the three demo plugins (`note` shell backend, `calc` tool, `progress` frontend) with their full source, the `plugin.yml` reference, the backend variables, the `ctx` table, layouts, trust, and a troubleshooting list. `plugins/index.md` and `directives.md` are updated (the `:::note` example now really renders), and the README links the how-to. Pages must pass Vale with the folder's vocabulary (add words to `accept.txt` as needed).

## Interfaces

```ts
// frontend/src/plugins/runtime.ts
export type PluginCtx = { id: string; kind: "block" | "inline" | "layout"; name: string; attrs: Record<string, string>;
  page: { path: string; title: string };
  storage: { get(key: string): Promise<unknown>; set(key: string, value: unknown): Promise<void> };
  invoke(payload: unknown): Promise<unknown>; navigate(path: string, anchor?: string): void };
export type PluginHandler = { mount(el: HTMLElement, ctx: PluginCtx): void | (() => void) | Promise<void | (() => void)> };
export function installRuntime(opts: { api: Api; navigate(path: string, anchor?: string): void; page(): { path: string; title: string } }): { mountAll(root: HTMLElement): void; unmountAll(): void; destroy(): void };
```

## Tests

| # | Behaviour | Test (`frontend/src/plugins/*.test.ts` unless noted) |
|---|---|---|
| 1,2 | register, mount per wrapper, kinds, attrs parsing, late registration, clean-up order, error isolation | `runtime.test.ts` |
| 3 | storage and invoke round trips, own id only, non-serialisable values | `runtime.test.ts` |
| 2,5 | page feature mounts after render, unmounts before re-render and on draft updates; layout class | `frontend/src/page/plugins.test.ts` |
| 6 | fake and wails client | `frontend/src/api/fake.test.ts`, `wails.test.ts` |
| 7 | list, statuses, Allow/Block, invalid, empty | `frontend/src/shell/settings.test.ts` |
| 8 | the demo plugin's script against the runtime (import the file, fake storage) | `demo-progress.test.ts` |

## Security

| Risk | Mitigation | Test |
|---|---|---|
| A plugin script reading another plugin's data | `ctx` is bound to the wrapper's id; storage/invoke take no id argument | `runtime.test.ts` |
| One broken plugin breaking the page | every mount/clean-up isolated | `runtime.test.ts` |
| Plugin text in Settings | all through `textContent` | `settings.test.ts` |

## Notes

- **Out of scope:** the Go side (core/extplugins), plugin scripts in the editor window, a plugin dev-reload button.
- **Integration (lead):** regenerate bindings; `wails.ts` maps the five new service methods (names in core/extplugins).
- **Changelog:** Added: `zen` plugin script API, Plugins in Settings, plugin how-to in `examples/demo`.

## Prompt

> Implement `specs/ui/plugins/spec.md` following the rules in `specs/README.md`;
> docs/PLUGINS.md "Writing your own plugin" is the contract. Another agent builds the Go
> side and `examples/demo/.zen/plugins/{note,calc}/` exactly as that document shows them:
> write the how-to assuming those exist, and do not touch Go files or the generated bindings
> (write `wails.ts` against the method names in specs/core/extplugins; tsc will only pass for
> it after the lead regenerates bindings — report that, keep everything else green).
