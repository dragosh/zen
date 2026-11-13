# Specs

Every feature is specified here before it is built.

```
specs/<scope>/<feature>/
  spec.md     behaviour, acceptance criteria, out of scope
  prompt.md   the prompt used to build the feature
```

Tests live next to the code they cover and are listed in `spec.md`.

Order of work for a feature:

1. Copy `specs/_template` to `specs/<scope>/<feature>` and fill it in.
2. Write the tests named in the spec. They must fail.
3. Commit spec, prompt and tests.
4. Implement until the tests pass, then commit.

Scopes: `cli`, `core/document`, `core/watch`, `core/render`, `core/plugins`,
`core/resources`, `ui/editor`, `ui/preview`, `plugins/<name>`,
`packaging/macos`. See [docs/PLAN.md](../docs/PLAN.md).

| Feature | Status |
|---|---|
| [cli/entrypoint](cli/entrypoint/spec.md) | specified |
