# plugins/quiz (plan)

| | |
|---|---|
| Status | done |
| Target version | 0.2.0 |
| Depends on | core/extplugins, ui/plugins |
| Kind | external plugin, frontend only; optional backend for exporting results |

## Goal

Questionnaires in a page, written as markdown, with answers and scores that
persist.

```markdown
:::quiz{id=http-basics title="HTTP basics" pass=70}
1. Which method is idempotent?
   - ( ) POST
   - (x) PUT
   - ( ) PATCH
   > PUT replaces the resource, so repeating it changes nothing.
2. Select the 2xx codes.
   - [x] 200
   - [x] 204
   - [ ] 301
3. What does TLS stand for? = Transport Layer Security
:::
```

- `( )` / `(x)` single choice, `[ ]` / `[x]` multiple choice, `= answer` free text (case and spacing ignored; `= /regex/` allowed). A blockquote under a question is its explanation, shown after answering.
- **Flow:** answer → Check → per-question right/wrong with the explanation → score and pass/fail at the end → Retry. Options `shuffle`, `reveal=end|each`, `attempts=3`.
- **Persistence (`ctx.storage`, key `quiz:<id>`):** answers so far (resume after closing the app), attempts with date and score, best score. A header line shows "Best 80% · 2 attempts".
- **Answers are not in the page.** The container is parsed by a tiny backend `block` hook that strips the `(x)`/`[x]`/`=` markers from the HTML it returns and keeps the key server-side; checking goes through `ctx.invoke({op: "check", id, answers})`. (A frontend-only mode `{open}` leaves answers in the DOM for self-study pages.)

## Needs from the core

| Need | Why | Change |
|---|---|---|
| Default container content + `data-zen-attrs` | parse the list in the plugin | in core/extplugins (wave 6) |
| Backend state between `block` and `action` | answer key outside the page | `ZEN_DATA_DIR` (wave 6): the block hook writes `keys/<page>-<id>.json`, the action hook reads it |
| Storage listing | a "My results" page | `ctx.storage.keys(prefix)` (small addition) |
| Export | hand results to a teacher | action `export` → CSV in `ZEN_DATA_DIR`, then `ctx.reveal(path)` (new, asks before showing in Finder) |

## Steps

1. Parser for the three question types (pure function, table-driven tests), HTML without answers.
2. `check` action with scoring; storage schema v1 `{version, answers, attempts[], best}` and migration stub.
3. Frontend: keyboard-first form (radio/checkbox/text with labels, `aria-live` results), resume, retry, summary.
4. `ctx.storage.keys`, `:::quiz-results` block listing every quiz in the folder with best scores.
5. Demo `examples/demo/plugins/quiz.md`; docs in the how-to.

## Security and privacy

- Scores stay on this Mac (user config dir), never written into the folder unless exported.
- Regex answers are length-limited and run with a time budget to avoid catastrophic patterns.
- The backend never evaluates page content; it only compares.
