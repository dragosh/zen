import { expect, test } from "vitest";
import { formatDate, formatSize, readingLabel } from "./format";

test("sizes are human readable", () => {
  expect(formatSize(0)).toBe("0 B");
  expect(formatSize(512)).toBe("512 B");
  expect(formatSize(1024)).toBe("1 KB");
  expect(formatSize(1536)).toBe("1.5 KB");
  expect(formatSize(10 * 1024)).toBe("10 KB");
  expect(formatSize(5 * 1024 * 1024 + 300 * 1024)).toBe("5.3 MB");
  expect(formatSize(3 * 1024 ** 3)).toBe("3 GB");
  expect(formatSize(-1)).toBe("");
});

test("reading label", () => {
  expect(readingLabel(0)).toBe("under a minute");
  expect(readingLabel(1)).toBe("~1 min");
  expect(readingLabel(7)).toBe("~7 min");
});

test("dates show the day and the time; 0 or garbage is empty", () => {
  const s = formatDate(Date.UTC(2026, 2, 4, 12, 0, 0));
  expect(s).toMatch(/2026/);
  expect(formatDate(0)).toBe("");
  expect(formatDate(Number.NaN)).toBe("");
});
