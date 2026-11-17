---
title: A memo
layout: memo
to: The whole team
from: Ada
date: 2026-11-17
nav:
  parent: layouts/index.md
  order: 9
  title: Memo (a folder layout)
---
# A memo

This page uses `memo`, a layout that lives in this folder, in
`.zen/layouts/memo/`. It is a template and a style sheet, and it reads the
fields `to`, `from` and `date` from the frontmatter.

The layout is not built in, so zen asked before using it, like it does for
plugins. [Writing your own layout](writing-your-own-layout.md) builds it step by
step.
