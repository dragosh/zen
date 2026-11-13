import { expect, test, vi } from "vitest";
import { createStore } from "./store";

test("starts empty in read mode", () => {
  expect(createStore().get()).toEqual({ workspace: null, path: "", anchor: "", mode: "read" });
});

test("set merges and notifies subscribers", () => {
  const store = createStore();
  const fn = vi.fn();
  store.subscribe(fn);
  store.set({ path: "a.md" });
  expect(store.get().path).toBe("a.md");
  expect(store.get().mode).toBe("read");
  expect(fn).toHaveBeenCalledTimes(1);
  expect(fn).toHaveBeenCalledWith(store.get());
});

test("set without a change does not notify", () => {
  const store = createStore({ path: "a.md" });
  const fn = vi.fn();
  store.subscribe(fn);
  store.set({ path: "a.md" });
  store.set({});
  expect(fn).not.toHaveBeenCalled();
});

test("unsubscribe stops notifications", () => {
  const store = createStore();
  const fn = vi.fn();
  const off = store.subscribe(fn);
  off();
  store.set({ mode: "edit" });
  expect(fn).not.toHaveBeenCalled();
});

test("a subscriber that unsubscribes during a notification does not break others", () => {
  const store = createStore();
  const second = vi.fn();
  const off = store.subscribe(() => off());
  store.subscribe(second);
  store.set({ path: "a.md" });
  expect(second).toHaveBeenCalledTimes(1);
});
