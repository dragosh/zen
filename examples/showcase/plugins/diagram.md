---
plugins:
  diagram: { theme: neutral }
---
# Diagrams

Write a ` ```mermaid ` block and zen draws it. Diagrams follow the light or
dark theme.

## Flowchart

```mermaid
graph LR
  A[Open folder] --> B{README.md?}
  B -- yes --> C[Show README]
  B -- no --> D[Pick a page]
  C --> E[Edit]
  D --> E
  E -->|save| C
```

## Sequence

```mermaid
sequenceDiagram
  participant You
  participant Editor
  participant Main as Main window
  You->>Editor: type
  Editor-->>Main: unsaved text (live)
  You->>Editor: ⌘S
  Editor->>Main: saved, reload from disk
```

## When a diagram is wrong

The source stays visible with mermaid's error message under it:

```mermaid
graph TD
  A --> 
```

## Options

```yaml
plugins:
  diagram: { theme: neutral }   # neutral (default), default or dark
```
