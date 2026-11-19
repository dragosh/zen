# Checking your writing with Vale

Builds of Zenmark with the **Vale add-on** include [Vale](https://vale.sh), a
prose linter. It checks spelling and style on the page you read and on the text
you type, without sending anything anywhere. Settings › This build shows whether
your build has it; without it, nothing on this page appears.

## Try it

This sentnce has a speling mistake, so you should see two red wavy
underlines here, and a count next to **Edit**. Open the editor to see the
**Problems** list: click a problem to jump to it and read why it was flagged.

| Underline | Meaning |
|---|---|
| red, wavy | error, usually spelling |
| amber, wavy | warning |
| blue, wavy | suggestion |

Code blocks and inline code are never checked:

```text
mispeled words in code are fine
```

## Your own rules and words

Without settings zen uses Vale's spelling check and built-in rules. A
`.vale.ini` at the root of the folder replaces them. This folder has one,
which accepts project words such as zen, Wails and frontmatter:

```ini
StylesPath = .vale/styles
MinAlertLevel = suggestion
Vocab = Zen

[*.md]
BasedOnStyles = Vale
```

The accepted words live in `.vale/styles/config/vocabularies/Zen/accept.txt`,
one per line.

## Turning it off

```yaml
plugins:
  lint: false
```

in a page's frontmatter, the folder's `.zen.yml` or the global `config.yml`.
