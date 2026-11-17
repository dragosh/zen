# Roadmap

What happens when, as a chart and as a table. The dates start on {{date}}.
Change the chart and the table together, so they never disagree.

```mermaid
gantt
  title {{title}}
  dateFormat YYYY-MM-DD
  axisFormat %d %b
  section Plan
  Define the goal       :done, goal, {{date}}, 7d
  Agree the scope       :active, scope, after goal, 7d
  section Build
  First version         :first, after scope, 21d
  Review                :review, after first, 7d
  section Launch
  Prepare the release   :prep, after review, 10d
  Release               :milestone, rel, after prep, 0d
```

## Milestones

| Milestone | Target | Owner | Status |
|---|---|---|---|
| Goal defined | {{date}} plus 1 week | {{author}} | done |
| Scope agreed | {{date}} plus 2 weeks | {{author}} | in progress |
| First version | {{date}} plus 5 weeks | | not started |
| Release | {{date}} plus 8 weeks | | not started |

## How to read this page

The status column holds one of: not started, in progress, blocked or done. When
something is blocked, add a line below the table that says why.
