import { expect, test } from "vitest";
import { createHistory } from "./history";

test("empty history cannot move", () => {
  const h = createHistory();
  expect(h.current()).toBeNull();
  expect(h.back()).toBeNull();
  expect(h.forward()).toBeNull();
  expect(h.canBack()).toBe(false);
  expect(h.canForward()).toBe(false);
});

test("back and forward walk the stack", () => {
  const h = createHistory();
  h.push({ path: "a.md", anchor: "" });
  h.push({ path: "b.md", anchor: "x" });
  h.push({ path: "c.md", anchor: "" });
  expect(h.back()).toEqual({ path: "b.md", anchor: "x" });
  expect(h.back()).toEqual({ path: "a.md", anchor: "" });
  expect(h.back()).toBeNull();
  expect(h.current()).toEqual({ path: "a.md", anchor: "" });
  expect(h.canForward()).toBe(true);
  expect(h.forward()).toEqual({ path: "b.md", anchor: "x" });
  expect(h.forward()).toEqual({ path: "c.md", anchor: "" });
  expect(h.forward()).toBeNull();
});

test("push after back drops the forward entries", () => {
  const h = createHistory();
  h.push({ path: "a.md", anchor: "" });
  h.push({ path: "b.md", anchor: "" });
  h.back();
  h.push({ path: "c.md", anchor: "" });
  expect(h.canForward()).toBe(false);
  expect(h.back()).toEqual({ path: "a.md", anchor: "" });
});

test("pushing the current entry again adds nothing; a new anchor does", () => {
  const h = createHistory();
  h.push({ path: "a.md", anchor: "" });
  h.push({ path: "a.md", anchor: "" });
  expect(h.canBack()).toBe(false);
  h.push({ path: "a.md", anchor: "intro" });
  expect(h.canBack()).toBe(true);
});

test("reset clears everything", () => {
  const h = createHistory();
  h.push({ path: "a.md", anchor: "" });
  h.push({ path: "b.md", anchor: "" });
  h.reset();
  expect(h.current()).toBeNull();
  expect(h.canBack()).toBe(false);
});
