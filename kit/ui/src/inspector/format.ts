// Plain formatting for the Inspector's values.

const UNITS = ["B", "KB", "MB", "GB", "TB"];

/** 512 -> "512 B", 1536 -> "1.5 KB": binary steps, one decimal below 10. "" for a size that makes no sense. */
export function formatSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) return "";
  let n = bytes;
  let u = 0;
  while (n >= 1024 && u < UNITS.length - 1) {
    n /= 1024;
    u++;
  }
  const text = u === 0 || n >= 10 ? String(Math.round(n)) : String(Math.round(n * 10) / 10);
  return `${text} ${UNITS[u]}`;
}

/** A Unix-millisecond time in the user's locale, day and minutes. "" for 0 or garbage. */
export function formatDate(ms: number): string {
  if (!Number.isFinite(ms) || ms <= 0) return "";
  try {
    return new Date(ms).toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });
  } catch {
    return new Date(ms).toISOString();
  }
}

/** "~7 min", or "under a minute" for an empty page. */
export function readingLabel(minutes: number): string {
  return minutes > 0 ? `~${minutes} min` : "under a minute";
}
