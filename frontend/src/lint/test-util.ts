import type { Alert } from "../api";

/** An alert with defaults for the fields a test does not care about. */
export function alert(p: Partial<Alert> & Pick<Alert, "match">): Alert {
  return { line: 1, col: 1, endCol: p.match.length, severity: "error", check: "Vale.Spelling", message: `Did you really mean '${p.match}'?`, ...p };
}
