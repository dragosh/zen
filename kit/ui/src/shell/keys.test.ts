// @vitest-environment happy-dom
import { expect, test } from "vitest";
import { closeWindowKey, reloadKey, shellAction, zoomKey } from "./keys";

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

// The window's own shortcuts, and which feature each belongs to (kit/shell-features).
test("shellAction maps ⌘O, ⌘,, ⌘N, ⇧⌘N, ⌘\\ and ⌘[ ⌘] to actions", () => {
  const all = () => true;
  const act = (init: KeyboardEventInit, has: (id: string) => boolean = all) => shellAction(ev({ metaKey: true, ...init }), has);
  expect(act({ key: "o" })).toBe("open-folder");
  expect(act({ key: "O" })).toBe("open-folder");
  expect(act({ key: "," })).toBe("settings");
  expect(act({ key: "n" })).toBe("new-page");
  expect(act({ key: "N", shiftKey: true })).toBe("new-site");
  expect(act({ key: "\\" })).toBe("toggle-sidebar");
  expect(act({ key: "[" })).toBe("back");
  expect(act({ key: "]" })).toBe("forward");
  expect(shellAction(ev({ key: "o" }), all)).toBeNull(); // no command key
  expect(act({ key: "o", altKey: true })).toBeNull();
  expect(act({ key: "o", shiftKey: true })).toBeNull();
  expect(act({ key: "x" })).toBeNull();
});

test("shellAction drops the shortcuts of missing features", () => {
  const only = (...ids: string[]) => (id: string) => ids.includes(id);
  const act = (init: KeyboardEventInit, has: (id: string) => boolean) => shellAction(ev({ metaKey: true, ...init }), has);
  expect(act({ key: "n" }, only("nav"))).toBeNull(); // New Page is the editor's
  expect(act({ key: "n" }, only("editor"))).toBe("new-page");
  expect(act({ key: "N", shiftKey: true }, only("editor"))).toBeNull(); // New Site is newsite's
  expect(act({ key: "N", shiftKey: true }, only("newsite"))).toBe("new-site");
  expect(act({ key: "\\" }, only("editor"))).toBeNull(); // the sidebar is nav's
  expect(act({ key: "\\" }, only("nav"))).toBe("toggle-sidebar");
  // open, settings and history stay in every build
  for (const key of ["o", ",", "[", "]"]) expect(act({ key }, only())).not.toBeNull();
});

test("zoom keys", () => {
  const z = (init: KeyboardEventInit) => zoomKey(ev(init));
  // ⌘+ (which is ⇧⌘= on a US keyboard), ⌘=, ⌘−, ⌘0
  expect(z({ key: "+", metaKey: true, shiftKey: true })).toBe(1);
  expect(z({ key: "+", metaKey: true })).toBe(1);
  expect(z({ key: "=", metaKey: true })).toBe(1);
  expect(z({ key: "=", ctrlKey: true })).toBe(1);
  expect(z({ key: "-", metaKey: true })).toBe(-1);
  expect(z({ key: "\u2212", metaKey: true })).toBe(-1);
  expect(z({ key: "_", metaKey: true, shiftKey: true })).toBe(-1);
  expect(z({ key: "0", metaKey: true })).toBe(0);
  // not without the command key, and not with ⌥
  expect(z({ key: "=" })).toBeNull();
  expect(z({ key: "-" })).toBeNull();
  expect(z({ key: "0" })).toBeNull();
  expect(z({ key: "=", metaKey: true, altKey: true })).toBeNull();
  expect(z({ key: "0", metaKey: true, altKey: true })).toBeNull();
  expect(z({ key: "1", metaKey: true })).toBeNull();
});
