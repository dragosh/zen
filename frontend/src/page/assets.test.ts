// @vitest-environment happy-dom
import { beforeEach, expect, test } from "vitest";
import { loadAssets } from "./assets";
import { mountPage } from "./index";
import type { Page } from "../api";

beforeEach(() => document.head.replaceChildren());

const page = (assets: Page["assets"]): Page => ({
  path: "a.md", title: "A", html: "<h1>A</h1>", frontmatter: {}, breadcrumb: [], assets, warnings: [],
});

test("assets are loaded once per URL, only from /_plugins/", () => {
  const assets = { css: ["/_plugins/d/a.css", "https://evil.test/x.css", "/_ws/x.css"], js: ["/_plugins/d/a.js", "//evil.test/x.js", "/_plugins/../x.js"] };
  loadAssets(document, assets);
  loadAssets(document, assets);
  const links = document.head.querySelectorAll("link[rel=stylesheet]");
  const scripts = document.head.querySelectorAll("script");
  expect(Array.from(links, (l) => l.getAttribute("href"))).toEqual(["/_plugins/d/a.css"]);
  expect(Array.from(scripts, (s) => s.getAttribute("src"))).toEqual(["/_plugins/d/a.js"]);
  expect(scripts[0].getAttribute("type")).toBe("module");
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
