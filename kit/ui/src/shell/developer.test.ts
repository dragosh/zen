// @vitest-environment happy-dom
import { beforeEach, expect, test, vi } from "vitest";
import { DEV_KEY, developerEnabled, developerKey, followDeveloper, setDeveloperEnabled } from "./developer";

beforeEach(() => { localStorage.clear(); });

test("setting: off by default, remembered under the same key, and followed live", () => {
  expect(DEV_KEY).toBe("zen.developer"); // kept, so existing users keep their choice
  expect(developerEnabled()).toBe(false);
  const seen: boolean[] = [];
  const off = followDeveloper(() => seen.push(developerEnabled()));
  setDeveloperEnabled(true);
  expect(localStorage.getItem(DEV_KEY)).toBe("1");
  expect(developerEnabled()).toBe(true);
  setDeveloperEnabled(false);
  expect(localStorage.getItem(DEV_KEY)).toBeNull();
  expect(seen).toEqual([true, false]);
  off();
  setDeveloperEnabled(true);
  expect(seen).toHaveLength(2);
});

test("followDeveloper also hears another window's change (the storage event)", () => {
  const fn = vi.fn();
  const off = followDeveloper(fn);
  window.dispatchEvent(new StorageEvent("storage", { key: DEV_KEY }));
  window.dispatchEvent(new StorageEvent("storage", { key: "other" }));
  expect(fn).toHaveBeenCalledTimes(1);
  off();
});

test("broken storage means off", () => {
  vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => { throw new Error("no storage"); });
  expect(developerEnabled()).toBe(false);
  vi.restoreAllMocks();
});

test("⌥⌘J (or ⌥⌃J), no Shift, read from the physical key", () => {
  const k = (init: KeyboardEventInit) => developerKey(new KeyboardEvent("keydown", init));
  expect(k({ code: "KeyJ", key: "∆", altKey: true, metaKey: true })).toBe(true);
  expect(k({ code: "KeyJ", key: "j", altKey: true, ctrlKey: true })).toBe(true);
  expect(k({ code: "KeyJ", key: "j", metaKey: true })).toBe(false);
  expect(k({ code: "KeyJ", key: "j", altKey: true })).toBe(false);
  expect(k({ code: "KeyJ", key: "J", altKey: true, metaKey: true, shiftKey: true })).toBe(false);
});
