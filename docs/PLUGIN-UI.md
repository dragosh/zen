# Plugin UI: the kit and the rules

Every interactive block (quiz, rating, checklist, sandbox, a course step) looks like one app. The look lives in
**one stylesheet, the plugin kit** (`runtime/src/css/plugin-kit.css`). zen ships it with the page CSS on every page,
in the app and in `zenmd build` output, so a plugin or layout never lists it. This file is the rule for every
plugin and layout, built in or your own. The reference look is the sandbox's bar and buttons.

## The kit

| Class | Is |
|---|---|
| `.zen-block` | The frame of a block: 1px `--rule` border, `--radius`, `--bg-subtle`, UI font 14px, margin `1em 0`. |
| `.zen-block-head` | The title row: UI font 14px, weight 600. Put a `.zen-block-status` on its right. |
| `.zen-block-status` | A muted status, 13px (a best score, "Not rated yet", a run's exit code). |
| `.zen-block-body` | The block's content, padding `10px 12px`. |
| `.zen-block-bar` | The action row: flex, gap 8px, a rule above. |
| `.zen-btn` | A button: 13px UI font, `--btn-bg`, no border, hover `--btn-bg-hover`, disabled at 0.45. |
| `.zen-btn-primary` | The main action (Run, Check, Next, Finish). One per block. |
| `.zen-btn-quiet` | A secondary text action (Clear, Reset): transparent, muted, hover background. |
| `.zen-choice` | An option row with a radio or checkbox. The input (16px) sits inside the frame, and wrapped text lines up after it. Put it on the `label` (or on the `li` of a task list). |
| `.zen-field` | A text input or textarea: 14px UI font, `--bg`, 1px `--rule` border. |

All of them have a `:focus-visible` ring. A `fieldset.zen-block` with a `legend.zen-block-head` keeps the group
semantics and shows the legend as the title row inside the frame; put the content in a `.zen-block-body` after it.

```html
<fieldset class="zen-block">
  <legend class="zen-block-head">1. Which method is idempotent?</legend>
  <div class="zen-block-body">
    <label class="zen-choice"><input type="radio" name="q1"><span>PUT</span></label>
    <button type="submit" class="zen-btn zen-btn-primary">Check</button>
  </div>
</fieldset>
```

## Rules

1. **Use the kit.** Never restyle buttons, choices, fields or frames. If the kit lacks something every block needs,
   add it to the kit (one place), not to your plugin. A plugin's CSS keeps only what is its own: the sandbox
   console, the rating stars, the quiz's right and wrong border.
2. **Type.** Inside a block: the UI font (`--font-ui`) at 14px; titles weight 600; status and small print 13px muted.
   Never the page's serif (`--font-body`) or heading styles inside a block, and never a legend left in its default
   look. Code is `--font-mono`.
3. **Colour.** Only the tokens of [UI.md](UI.md): black and white, `--rule`, `--fg-muted`, `--focus`. The only hues
   are the signal colours (`--lint-error`, `--code-meta` for "right"). No hex, rgb or hsl in plugin CSS.
4. **States.** Empty, loading, error and done each look the same in every block: a status line in
   `.zen-block-status` style (muted, 13px); an error in `--lint-error`; "done" or "right" in `--code-meta`. Empty text
   is plain muted text, never a placeholder graphic. A busy control is disabled, not hidden.
5. **Keyboard and focus.** Everything reachable with Tab; native controls (`button`, `input`, `label`) rather than
   clickable `div`s; a visible `:focus-visible` ring (the kit's, never `outline: none` without a replacement);
   move focus to a new result (`tabindex="-1"` and `.focus()`); live updates in `role="status"` or `aria-live="polite"`.
6. **Reduced motion.** Honour `prefers-reduced-motion: reduce`: no animation, or a slower and calmer one (the sandbox
   spinner slows down). Nothing may depend on motion.
7. **Text only.** Set text with `textContent`; never `innerHTML` with data from the page, storage or a backend.
8. **Layouts.** A layout may change the page around blocks (columns, step list, navigation, spacing). It never
   changes a block's controls: its own buttons are `.zen-btn`, its checklists `.zen-block` and `.zen-choice`.
9. **Print.** Controls are hidden or readable in print; a block never needs the screen to make sense.

## Checklist for review

- [ ] Frames, buttons, choices and fields carry the kit classes; no copies of their rules in the plugin's CSS.
- [ ] One `.zen-btn-primary` at most per block; secondary actions are `.zen-btn` or `.zen-btn-quiet`.
- [ ] No serif, no heading styles, no hex or rgb colours inside a block.
- [ ] Empty, loading, error and done states are written and use the status style.
- [ ] Tab reaches every control, the focus ring shows, the result is announced.
- [ ] `prefers-reduced-motion` is respected.
- [ ] All text from data goes through `textContent`.
- [ ] A layout leaves the blocks' controls alone.
- [ ] Looked at in light and dark, at a narrow width, and next to a sandbox block.
