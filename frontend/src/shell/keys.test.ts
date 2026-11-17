// @vitest-environment happy-dom
import { expect, test } from "vitest";
import { closeWindowKey, reloadKey } from "./keys";

const ev = (init: KeyboardEventInit) => new KeyboardEvent("keydown", init);

test("⌘W is the close-window key, with no other modifier", () => {
  expect(closeWindowKey(ev({ key: "w", metaKey: true }))).toBe(true);
  expect(closeWindowKey(ev({ key: "W", metaKey: true }))).toBe(true);
  expect(closeWindowKey(ev({ key: "w", ctrlKey: true }))).toBe(true);
  expect(closeWindowKey(ev({ key: "w" }))).toBe(false);
  expect(closeWindowKey(ev({ key: "w", metaKey: true, shiftKey: true }))).toBe(false);
  expect(closeWindowKey(ev({ key: "w", metaKey: true, altKey: true }))).toBe(false);
  expect(closeWindowKey(ev({ key: "q", metaKey: true }))).toBe(false);
});

test("⌘R is the reload key, with no other modifier", () => {
  expect(reloadKey(ev({ key: "r", metaKey: true }))).toBe(true);
  expect(reloadKey(ev({ key: "R", ctrlKey: true }))).toBe(true);
  expect(reloadKey(ev({ key: "r" }))).toBe(false);
  expect(reloadKey(ev({ key: "r", metaKey: true, shiftKey: true }))).toBe(false);
  expect(reloadKey(ev({ key: "w", metaKey: true }))).toBe(false);
});
