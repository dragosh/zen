# <scope>/<feature>

| | |
|---|---|
| Status | draft \| specified \| in progress \| done |
| Target version | 0.1.0-alpha.1 |
| Branch | `feature/<scope>-<feature>` |
| Wave | 1 |
| Depends on | other specs, or none |
| Owns | paths only this feature may create or edit |

## Purpose

One or two sentences: what the user can do once this exists.

## Behaviour

Numbered, testable statements.

1. ...

## Architecture

Where the code lives, what it imports, what imports it, data flow. Reference
PLAN.md sections instead of repeating them.

## Interfaces

Public Go types and functions, service methods, events, TS modules or plugin
hooks this feature adds. Signatures, not prose.

## Tests

Written first and committed failing. Each behaviour maps to at least one test.

| # | Behaviour | Test |
|---|---|---|
| 1 | ... | `path/to/file_test.go: TestName` |

Edge cases to cover: ...

## UI

States, keyboard, accessibility, reference to UI.md. "None" for non-visual features.

## Security

Risks, mitigation, and the test that proves each. "None" needs a reason.

## Notes

- **Out of scope:** ...
- **Integration (done by the lead at merge):** exact lines to add to shared files.
- **App Store / sandbox:** implications or "none".
- **Changelog:** the line to add under Unreleased.

## Prompt

The prompt handed to the subagent that builds this feature.

> Implement `specs/<scope>/<feature>/spec.md` following the rules in
> `specs/README.md`. Context: ... Done when: ...
