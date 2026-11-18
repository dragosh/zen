# Settings

Every setting of {{title}}, with its type and default.

*This page is an example. Replace the table with the real settings.*

## Options

| Name | Type | Default | Description |
|---|---|---|---|
| `name` | text | the folder name | The name shown in titles and messages. |
| `version` | text | `0.1.0` | The version, in the form `major.minor.patch`. |
| `verbose` | true or false | `false` | Print each step as it runs. |
| `retries` | whole number | `3` | How many times to try again after a failure. |

## Example

```yaml
name: first-project
version: 0.1.0
verbose: true
retries: 5
```

## Rules

- Names are lower case and have no spaces.
- A setting that is not listed here is an error.
- Values in the file are read once, when the tool starts.
