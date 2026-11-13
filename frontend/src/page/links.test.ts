// @vitest-environment happy-dom
import { expect, test } from "vitest";
import { handleLinkClick } from "./index";
import { setup, tick } from "./helpers.test-util";

function click(html: string, init: MouseEventInit = {}, type = "click") {
  const env = setup();
  env.ctx.store.set({ path: "README.md" });
  const root = document.createElement("article");
  root.innerHTML = html;
  document.body.append(root);
  const target = root.querySelector("a span, a") as HTMLElement;
  const e = new MouseEvent(type, { bubbles: true, cancelable: true, button: 0, ...init });
  root.addEventListener(type, (ev) => handleLinkClick(ev as MouseEvent, env.ctx));
  target.dispatchEvent(e);
  return { env, e };
}

test("internal link navigates in place", () => {
  const { env, e } = click(`<a data-zen-link="internal" data-path="guides/install.md" data-anchor="intro" href="#"><span>x</span></a>`);
  expect(e.defaultPrevented).toBe(true);
  expect(env.navigations).toEqual([["guides/install.md", "intro"]]);
});

test("external link calls openExternal and never navigates", async () => {
  const { env, e } = click(`<a data-zen-link="external" href="https://example.com/a">x</a>`);
  await tick();
  expect(e.defaultPrevented).toBe(true);
  expect(env.api.callsTo("openExternal")).toEqual([["https://example.com/a"]]);
  expect(env.navigations).toEqual([]);
});

test("broken link does nothing", () => {
  const { env, e } = click(`<a data-zen-link="broken">x</a>`);
  expect(e.defaultPrevented).toBe(true);
  expect(env.navigations).toEqual([]);
  expect(env.api.calls).toEqual([]);
});

test("modifier and middle clicks behave like plain clicks", async () => {
  for (const init of [{ metaKey: true }, { ctrlKey: true }, { shiftKey: true }]) {
    const { env, e } = click(`<a data-zen-link="internal" data-path="guides/usage.md" data-anchor="" href="#">x</a>`, init);
    expect(e.defaultPrevented).toBe(true);
    expect(env.navigations).toEqual([["guides/usage.md", ""]]);
  }
  const { env, e } = click(`<a data-zen-link="external" href="https://example.com">x</a>`, { button: 1 }, "auxclick");
  await tick();
  expect(e.defaultPrevented).toBe(true);
  expect(env.api.callsTo("openExternal")).toEqual([["https://example.com"]]);
});

test("unmarked anchor with a scheme is external; same-page hash navigates to the anchor", async () => {
  const ext = click(`<a href="mailto:a@b.c">x</a>`);
  await tick();
  expect(ext.e.defaultPrevented).toBe(true);
  expect(ext.env.api.callsTo("openExternal")).toEqual([["mailto:a@b.c"]]);

  const hash = click(`<a href="#usage">x</a>`);
  expect(hash.e.defaultPrevented).toBe(true);
  expect(hash.env.navigations).toEqual([["README.md", "usage"]]);
});

test("javascript: links are never opened or followed", async () => {
  const { env, e } = click(`<a href="javascript:alert(1)">x</a>`);
  await tick();
  expect(e.defaultPrevented).toBe(true);
  expect(env.api.calls).toEqual([]);
});
