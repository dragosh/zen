# Possible plugins

Ideas for plugins that fit the current plugin system (docs/PLUGINS.md). Not
planned or specified yet: the plugin internals get hardened first. Each row
names the parts of the system it would use:

- **Go**: a block, inline, source or HTML hook in the render pipeline
- **script**: frontend JS registered with `zen.register`
- **storage**: `ctx.storage`, kept per plugin and folder on this Mac
- **stream**: `ctx.stream`, live output from a backend
- **layout**: `layout:` in frontmatter
- **badge**: `ctx.badge` in the sidebar

## Reading and writing

| Plugin | What it does | Uses |
|---|---|---|
| **toc** | `:::toc` or `toc: true` in frontmatter: a contents list for the page; a floating variant follows the scroll | Go (headings → list), script (scroll spy) |
| **math** | `$x^2$` and `$$…$$` rendered with KaTeX, bundled and checksummed like mermaid | Go (keep the source intact), script |
| **include** | `:::include{file=snippets/api.md lines=10-40}` pulls another page or a code file into this one | Go (reads inside the folder only, loop protection) |
| **backlinks** | "Linked from" list at the bottom of each page, from the link graph the sidebar already has | Go (index) |
| **search** | ⌘F across the folder: full text, results with context, jump to the match | Go (index built on open), script (overlay) |
| **callouts** | `:::tip`, `:::warning`, `:::danger`, and GitHub-style `> [!NOTE]` | Go |
| **embed** | `:::youtube{id=…}`, or a link card (title, image) for a URL, fetched once and cached | Go (opt-in network access) |

## Knowledge and planning

| Plugin | What it does | Uses |
|---|---|---|
| **kanban** | `layout: board`: each `##` is a column, each list item a card; dragging writes back to the markdown | layout, script, saving through the editor |
| **tasks** | `:::tasks{tag=todo}` collects open `- [ ]` items from every page, each linked to its page | Go (folder scan) |
| **timeline** | dated list items become a vertical timeline, or a Gantt chart through mermaid | Go, CSS |
| **glossary** | terms defined once in `glossary.md` get a hover definition everywhere | Go (index, adds the hovers) |
| **daily notes** | ⌘D opens or creates `journal/YYYY-MM-DD.md` from a template | menu command, Go creates the file |

## Learning

| Plugin | What it does | Uses |
|---|---|---|
| **flashcards** | `Q:` / `A:` pairs become a spaced-repetition deck; due cards show as a sidebar badge | script, storage, badge |
| **playground** | runs JS or Python in the page (QuickJS or Pyodide bundled); no container setup | script |
| **terminal recording** | plays back an `asciinema` recording inline | script (bundled player) |
| **course certificate** | when a `course` is finished, a PDF summary of steps and quiz scores | storage, print |

## Tools

| Plugin | What it does | Uses |
|---|---|---|
| **http** | a ```` ```http ```` block with a request and a Send button; status, headers, formatted body; saved environments for base URLs and tokens | stream, user-only options |
| **sql** | ```` ```sql {db=notes.sqlite} ```` queries a SQLite file in the folder and shows a table, read-only by default | Go |
| **csv / table** | `:::table{file=data.csv}`: sortable, filterable table; `chart=bar` draws a chart | Go (reads the file), script |
| **git** | history of a page, diff against the last commit, "last edited by" in the header | Go (calls `git`) |
| **ai summary** | "Summarize" or "Explain this block", calling the Claude API with your own key; opt-in per folder | stream, user-only options |

## Suggested order

1. **search** and **backlinks**: most useful for any folder; zen already has most of the data.
2. **toc** and **callouts**: small, visible, needed by every docs folder.
3. **math**: common in technical notes; the same pattern as mermaid.
4. **kanban**: shows off layouts plus writing back to the markdown.
