# Vendored files

| File | Package | Version | Source | SHA-256 | License |
|---|---|---|---|---|---|
| `assets/mermaid.min.js` | mermaid | 11.4.1 | https://cdn.jsdelivr.net/npm/mermaid@11.4.1/dist/mermaid.min.js | `a43bc1afd446f9c4cc66ac5dd45d02e8d65e26fc5344ec0ef787f88d6ddb6f9e` | MIT |

`mermaid.min.js` is the IIFE build: loading it as a classic script defines
`globalThis.mermaid`. It is not an ES module.

To update: download the new version from the same URL pattern, replace the
file, update this table, and run `go test ./internal/plugins/diagram`.
