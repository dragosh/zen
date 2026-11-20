# Add-ons

An **add-on** is an optional capability that an app includes only when it is
built with it: usually a helper program, the Go code that drives it, and the UI
it adds. Each app built on the kit (Zenmark today; HashDoc, HashTalk, HashStudy
and HashDish later) lists its add-ons in `apps/<app>/app-assembly.yml`, next to
its built-in plugins, layouts, site templates and theme (specs/kit/assembly):

```yaml
app: zenmark
addons: [vale]
plugins: all
layouts: all
templates: all
theme: default
```

`ADDONS=` on the command line overrides the file for one build:

```sh
task package ADDONS=        # no add-ons
task release ADDONS=vale,tts
```

How it works is specified in [specs/kit/addons](../specs/kit/addons/spec.md):
the `addon_<id>` build tag, the `addons/<id>` Go package that registers itself,
helpers in `resources.yml` marked `addon: <id>`, and Settings › This build, which
lists what a build includes.

## Add-on, plugin or layout?

| | Add-on | Plugin | Layout |
|---|---|---|---|
| Chosen by | whoever builds the app | the folder's author (`.zen.yml`, frontmatter) | the page (`layout:`) |
| Can bring | helper programs, native APIs, Go services, menu items, panel tabs, entitlements | render hooks, page scripts, storage, streams, badges | page structure and CSS |
| Runs in | the app | the render pipeline and the page | the page |
| Example | Vale linting | mermaid diagrams | slides, handout |

Rule of thumb: if it needs a program or a system permission the app would not
otherwise have, or it changes the app rather than a page, it is an add-on. If
the folder should decide, it is a plugin. A plugin may depend on an add-on
(a "speak this block" plugin needs the text-to-speech add-on) and must then say
so and degrade quietly without it.

A fourth kind, the **feature**, is part of the app itself, on by default:
`features:` in `app-assembly.yml` can leave one out (specs/kit/features). The
features are `reader`, `nav`, `find`, `editor` (without it the app is read-only), `inspector`, `print`, `export`, `newsite` and `developer` (docs/BUILD.md). An add-on may need a feature: `vale` needs `reader`, and its Linting tab appears only with `inspector`. A **dialect** (`dialects:`,
specs/engine/dialects) is another markdown syntax for the same plugins, such as MyST;
the build decides which exist, the folder's `.zen.yml` which are on.

## Add-ons

| Id | Add-on | Status |
|---|---|---|
| `vale` | **Vale linting**: spelling and style marks in the reader and the editor, the Problems console, the Inspector's Linting tab | done (0.6.0) |

## Ideas

Not planned or specified yet. Each names what it would need from the kit; the
first add-on that needs a hook adds it, in its own spec.

### Voice

| Idea | What it does | Needs |
|---|---|---|
| **Text to speech** (`tts`) | Read the page aloud from the cursor or a selection, with the spoken sentence highlighted; a speed control; skip code blocks | macOS `AVSpeechSynthesizer` (no helper, system voices); a panel tab or toolbar control; a "speak" command |
| **Dictation** (`dictation`) | Speak into the editor; text is inserted at the cursor, with punctuation commands | a speech-to-text helper (whisper.cpp with a bundled model) or macOS `SFSpeechRecognizer`; the microphone entitlement and usage string; a toolbar button |
| **Audio notes** | Record a voice note into the folder and link it from the page, with an optional transcript | microphone; writes into the workspace; dictation's helper for the transcript |

### AI and agents

| Idea | What it does | Needs |
|---|---|---|
| **MCP server** (`mcp`) | Lets external agents (Claude, other MCP clients) read the open folder: list pages, read a page, search, see lint issues; writes only with a confirmation in the app | a local MCP endpoint (stdio through `zenmd mcp`, or a socket); a permission prompt per write; an activity log in the Developer panel |
| **LLM assistant** (`assistant`) | Ask about the page or the folder, summarise, rewrite a selection, suggest a title or front matter; answers stream into a panel tab | a provider setting (API key in the Keychain, or a local model); network access; a panel tab; the editor's selection API |
| **Agent tasks** | Hand a task to an external agent ("draft the missing pages of this course") and review its changes as a diff before they are saved | the MCP add-on; a review view in the editor |

### Language

| Idea | What it does | Needs |
|---|---|---|
| **Grammar** (`languagetool`) | Grammar and style checks beyond Vale, in many languages, through the same marks and Linting tab | a LanguageTool server helper (Java runtime) or its HTTP API; the linter hook from kit/addons |
| **Dictionaries** | Extra spelling languages for Vale (Hunspell dictionaries) | depends on `vale`; dictionary files as helpers |
| **Translate** | Translate a page or selection into a sibling file (`page.fr.md`) | a local model or a provider; writes into the workspace |

### Import and export

| Idea | What it does | Needs |
|---|---|---|
| **Pandoc** (`pandoc`) | Export › Word (.docx), EPUB, LaTeX; import .docx as markdown | the pandoc helper (~150 MB); the Export menu (app/export) |
| **Citations** | `[@key]` citations from a BibTeX or Zotero library, with a bibliography | pandoc's citeproc or a Go implementation; a library file setting |
| **OCR** | Text from images and scanned PDFs into the page | macOS Vision framework (no helper) |
| **Image optimiser** | Shrinks images when they are added to the folder, or on export | oxipng / cwebp helpers |

### Publishing and sync

| Idea | What it does | Needs |
|---|---|---|
| **Publish** | Export › Website straight to GitHub Pages, Netlify or an S3 bucket | network access; credentials in the Keychain |
| **Git** | History of a page, diff with the last commit, commit from the app | the system git or a bundled one; a panel tab |
| **Collaboration** | Several people on one folder with live presence | a sync service; out of reach for now, listed so the add-on model leaves room |

### Theming

Themes may not need an add-on: the look of pages is already the job of
layouts and the token CSS (docs/UI.md), and a folder can bring its own. An add-on
fits only if a build should ship a **set of app themes** (the chrome, not the
pages: colours, fonts, density) that its users pick in Settings › Appearance,
for example a branded look for one of the Hash apps. That would need a theme
hook: a list of token sets the Appearance section offers. Decide when the first
branded app is built.

## Adding an add-on

1. Write `specs/addons/<id>/spec.md` from the template: what it does, the helper
   (pinned in `resources.yml` with `addon: <id>` and a checksum), the hooks it
   needs (and add any new hook to `kit/addons` in the same spec), its settings,
   entitlements and what the app looks like without it.
2. Build it in `addons/<id>/` under `//go:build addon_<id>`, imported by the app in
   `apps/<app>/addons_<id>.go`.
3. Add its id to the known list checked by `kit/addons/cmd/check`, and its row to
   the table above.
