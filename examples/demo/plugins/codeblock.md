# Code blocks

Fenced code blocks with a language name are coloured. Without a name, or with
an unknown one, they stay plain. Hover a block (or tab to it) for two buttons
top right: **Copy** copies the code, **Wrap** wraps long lines.

```go
// Package greet says hello.
package greet

import "fmt"

func Hello(name string) string {
	return fmt.Sprintf("hello, %s (%d)", name, 42)
}
```

```python
def fib(n: int) -> int:
    """Return the n-th Fibonacci number."""
    a, b = 0, 1
    for _ in range(n):
        a, b = b, a + b  # step
    return a
```

```ts
type Page = { path: string; title: string };
export const titles = (pages: Page[]) => pages.map((p) => p.title.trim());
```

```json
{ "name": "zenmark", "plugins": ["codeblock", "diagram"], "beta": false }
```

## Options

```yaml
plugins:
  codeblock:
    lineNumbers: true   # add a line-number gutter to code blocks
    wrap: true          # start with long lines wrapped
```

Set them in `.zenmd.yml` for the folder, or in a page's frontmatter for one
page. Turn the plugin off with `codeblock: false`.
