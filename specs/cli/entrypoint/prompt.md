# Prompt: cli/entrypoint

## Context

`main.go` embeds `frontend/dist` and calls `app.Run` directly.
`internal/app/app.go` exposes `Run(Options{Assets, Files})`.

## Task

Implement `specs/cli/entrypoint/spec.md` in `internal/cli/cli.go` and route
`main.go` through `cli.Run`.

## Constraints

- Stay inside `internal/cli` and `main.go`.
- Standard library only.
- `internal/cli` must not import Wails, so it tests without a display.
- Do not change `internal/cli/cli_test.go`; make it pass.

## Done when

- All tests in the spec's acceptance table pass.
- `wails3 task test` passes.
