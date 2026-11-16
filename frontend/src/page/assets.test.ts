// @vitest-environment happy-dom
import { beforeEach, expect, test } from "vitest";
import { loadAssets } from "./assets";
import { mountPage } from "./index";
import type { Page } from "../api";

beforeEach(() => document.head.replaceChildren());

const page = (assets: Page["assets"]): Page => ({
  path: "a.md", title: "A", html: "<h1>A</h1>", frontmatter: {}, breadcrumb: [], assets, warnings: [], layout: "",
});

test("stylesheets are linked once per (versioned) URL, only from /_plugins/; scripts are never injected", () => {
  const assets = { css: ["/_plugins/d/a.css", "https://evil.test/x.css", "/_ws/x.css"], js: ["/_plugins/d/a.js", "//evil.test/x.js", "/_plugins/../x.js"] };
  loadAssets(document, assets);
  loadAssets(document, assets);
  const links = document.head.querySelectorAll("link[rel=stylesheet]");
  expect(Array.from(links, (l) => l.getAttribute("href"))).toEqual(["/_plugins/d/a.css"]);
  // the runtime import()s plugin scripts, keyed by plugin id (plugins/runtime.ts)
  expect(document.head.querySelectorAll("script")).toHaveLength(0);
  // a changed plugin has a new URL and gets its own link
  loadAssets(document, { css: ["/_plugins/d/a.css?v=abc123abc123"], js: [] });
  expect(document.head.querySelectorAll("link[rel=stylesheet]")).toHaveLength(2);
});

test("mountPage sets the html, loads assets and fires zen:page-rendered", () => {
  const el = document.createElement("article");
  let detail: unknown;
  const on = (e: Event) => { detail = (e as CustomEvent).detail; };
  document.addEventListener("zen:page-rendered", on);
  mountPage(el, page({ css: ["/_plugins/p/s.css"], js: [] }));
  document.removeEventListener("zen:page-rendered", on);
  expect(el.innerHTML).toBe("<h1>A</h1>");
  expect(document.head.querySelector("link")?.getAttribute("href")).toBe("/_plugins/p/s.css");
  expect(detail).toEqual({ root: el });
});
