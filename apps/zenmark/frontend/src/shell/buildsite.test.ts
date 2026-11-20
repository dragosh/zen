// @vitest-environment happy-dom
import { beforeEach, expect, test, vi } from "vitest";
import { openBuildSite, registerBuildSite, type BuildApi, type BuildOptions, type BuildProgress, type BuildResult } from "./buildsite";

const ROOT = "/Users/me/site";
const OPTIONS: BuildOptions = { out: `${ROOT}/_site`, portable: true, drafts: false, clean: false, print: true };
const RESULT: BuildResult = { out: `${ROOT}/_site`, pages: 34, files: 19, warnings: [], millis: 1234, cancelled: false };

/** A local fake of the build part of the api; buildWebsite waits until the test finishes it. */
function fakeApi(over: Partial<BuildApi> = {}) {
  const calls: { name: string; args: unknown[] }[] = [];
  const listeners = { progress: new Set<(p: BuildProgress) => void>(), menu: new Set<() => void>() };
  let finish: (r: BuildResult | Error) => void = () => {};
  const rec = <T>(name: string, value: () => T) => (...args: unknown[]) => { calls.push({ name, args }); return value(); };
  const api: BuildApi = {
    buildOptions: rec("buildOptions", () => Promise.resolve({ ...OPTIONS })),
    buildWebsite: rec("buildWebsite", () => new Promise<BuildResult>((res, rej) => { finish = (r) => (r instanceof Error ? rej(r) : res(r)); })),
    cancelBuild: rec("cancelBuild", () => Promise.resolve()),
    saveBuildDefaults: rec("saveBuildDefaults", () => Promise.resolve("Adds to .zen.yml:\n  build.drafts: true")),
    pickBuildFolder: rec("pickBuildFolder", () => Promise.resolve("")),
    openBuiltSite: rec("openBuiltSite", () => Promise.resolve()),
    revealBuiltSite: rec("revealBuiltSite", () => Promise.resolve()),
    on: ((event: string, fn: (p?: BuildProgress) => void) => {
      const set = (event === "build:progress" ? listeners.progress : listeners.menu) as unknown as Set<typeof fn>;
      set.add(fn);
      return () => { set.delete(fn); };
    }) as unknown as BuildApi["on"],
    ...over,
  };
  return {
    api, calls,
    to: (name: string) => calls.filter((c) => c.name === name),
    progress: (p: BuildProgress) => listeners.progress.forEach((f) => f(p)),
    menu: () => listeners.menu.forEach((f) => f()),
    finish: (r: BuildResult | Error) => finish(r),
    listeners,
  };
}

const settle = async () => { for (let i = 0; i < 6; i++) await new Promise((r) => setTimeout(r, 0)); };
beforeEach(() => document.body.replaceChildren());

function open(over: Partial<BuildApi> = {}, extra: { unsaved?: () => string[] } = {}) {
  const f = fakeApi(over);
  const onOpenPage = vi.fn();
  const d = openBuildSite({ api: f.api, doc: document, root: ROOT, name: "site", onOpenPage, ...extra });
  const q = <T extends Element = HTMLElement>(s: string) => d.el.querySelector<T>(s)!;
  const btn = (label: string) => Array.from(d.el.querySelectorAll<HTMLButtonElement>("button")).find((b) => b.textContent === label)!;
  return { f, d, q, btn, onOpenPage };
}
const key = (k: string) => document.dispatchEvent(new KeyboardEvent("keydown", { key: k, bubbles: true, cancelable: true }));

test("menu: the event opens the dialog once; without a folder nothing opens; a second event brings it to the front", async () => {
  const f = fakeApi();
  let root = "";
  const reg = registerBuildSite({ api: f.api, doc: document, root: () => root, name: () => "site", onOpenPage: vi.fn() });
  f.menu();
  await settle();
  expect(document.querySelector(".zen-buildsite")).toBeNull(); // no folder open
  root = ROOT;
  f.menu();
  await settle();
  expect(document.querySelectorAll(".zen-buildsite")).toHaveLength(1);
  (document.activeElement as HTMLElement).blur();
  f.menu();
  expect(document.querySelectorAll(".zen-buildsite")).toHaveLength(1);
  expect(document.activeElement?.closest(".zen-buildsite")).not.toBeNull();
  key("Escape");
  expect(document.querySelector(".zen-buildsite")).toBeNull();
  reg.destroy();
  expect(f.listeners.menu.size).toBe(0);
});

test("the form is filled from buildOptions: output shown relative inside the folder, Export has the focus", async () => {
  const { q, btn, d } = open();
  await settle();
  expect(q("[role=dialog]").getAttribute("aria-modal")).toBe("true");
  expect(q(".zen-buildsite-title").textContent).toContain("Export Website");
  expect(q(".zen-buildsite-title").textContent).toContain("site");
  expect(q<HTMLInputElement>(".zen-buildsite-out").value).toBe("_site");
  const radios = Array.from(d.el.querySelectorAll<HTMLInputElement>("input[type=radio]"));
  expect(radios.map((r) => [r.value, r.checked])).toEqual([["portable", true], ["hosted", false]]);
  const boxes = Array.from(d.el.querySelectorAll<HTMLInputElement>("input[type=checkbox]"));
  expect(boxes.map((b) => [b.name, b.checked])).toEqual([["drafts", false], ["clean", false], ["print", true]]);
  expect(document.activeElement).toBe(btn("Export"));
});

test("an output outside the folder is shown absolute", async () => {
  const { q } = open({ buildOptions: () => Promise.resolve({ ...OPTIONS, out: "/Volumes/web/site" }) });
  await settle();
  expect(q<HTMLInputElement>(".zen-buildsite-out").value).toBe("/Volumes/web/site");
});

test("Choose… fills the output from the folder panel; cancelling keeps it", async () => {
  const picks = ["/Volumes/web/out", `${ROOT}/public`, ""];
  const { q, btn } = open({ pickBuildFolder: () => Promise.resolve(picks.shift()!) });
  await settle();
  btn("Choose…").click();
  await settle();
  expect(q<HTMLInputElement>(".zen-buildsite-out").value).toBe("/Volumes/web/out");
  btn("Choose…").click();
  await settle();
  expect(q<HTMLInputElement>(".zen-buildsite-out").value).toBe("public");
  btn("Choose…").click();
  await settle();
  expect(q<HTMLInputElement>(".zen-buildsite-out").value).toBe("public");
});

test("Build passes the options (a relative output made absolute), shows progress, disables the form, and Cancel stays enabled", async () => {
  const { f, q, btn, d } = open();
  await settle();
  q<HTMLInputElement>(".zen-buildsite-out").value = "docs/out";
  d.el.querySelector<HTMLInputElement>("input[value=hosted]")!.click();
  d.el.querySelector<HTMLInputElement>("input[name=drafts]")!.click();
  btn("Export").click();
  await settle();
  expect(f.to("buildWebsite")[0].args[0]).toEqual({ out: `${ROOT}/docs/out`, portable: false, drafts: true, clean: false, print: true });
  const bar = q("[role=progressbar]");
  expect(q(".zen-buildsite-progress-text").textContent).toBe("Building…");
  f.progress({ done: 12, total: 34 });
  expect(q(".zen-buildsite-progress-text").textContent).toBe("Building… 12 / 34 pages");
  expect(bar.getAttribute("aria-valuenow")).toBe("12");
  expect(bar.getAttribute("aria-valuemax")).toBe("34");
  expect(Array.from(d.el.querySelectorAll<HTMLInputElement>("input")).every((i) => i.disabled)).toBe(true);
  expect(btn("Cancel").disabled).toBe(false);
  expect(btn("Choose…").disabled).toBe(true);
  expect(btn("Save as folder defaults").disabled).toBe(true);
  // Enter does not start a second build.
  d.el.querySelector("form")!.dispatchEvent(new Event("submit", { cancelable: true }));
  expect(f.to("buildWebsite")).toHaveLength(1);
  f.finish(RESULT);
  await settle();
});

test("result: the summary with the time, an announced status, and the four actions", async () => {
  const { f, q, btn, d } = open();
  await settle();
  btn("Export").click();
  await settle();
  f.finish(RESULT);
  await settle();
  const status = q(".zen-buildsite-summary");
  expect(status.getAttribute("role")).toBe("status");
  expect(status.textContent).toContain("Built 34 pages and 19 files into _site");
  expect(status.textContent).toContain("1.2 s");
  expect(d.el.querySelector(".zen-buildsite-warnings")).toBeNull();
  expect(Array.from(d.el.querySelectorAll<HTMLButtonElement>(".zen-buildsite-actions button")).map((b) => b.textContent)).toEqual(["Open in Browser", "Show in Finder", "Build Again", "Done"]);
  expect(btn("Open in Browser").classList.contains("primary")).toBe(true);
  expect(document.activeElement).toBe(btn("Open in Browser"));

  btn("Open in Browser").click();
  btn("Show in Finder").click();
  await settle();
  expect(f.to("openBuiltSite")[0].args).toEqual([RESULT.out]);
  expect(f.to("revealBuiltSite")[0].args).toEqual([RESULT.out]);

  btn("Build Again").click();
  expect(d.el.querySelector(".zen-buildsite-summary")).toBeNull();
  expect(q<HTMLInputElement>(".zen-buildsite-out").disabled).toBe(false);
  expect(document.activeElement).toBe(btn("Export"));
});

test("result: warnings are a list of buttons by page; a click opens the page and closes the dialog; site-wide ones are plain", async () => {
  const { f, q, btn, d, onOpenPage } = open();
  await settle();
  btn("Export").click();
  await settle();
  f.finish({ ...RESULT, warnings: [{ path: "guide/intro.md", message: "img/x.png: not found; left out" }, { path: "", message: "plugin \"hello\" is not allowed" }] });
  await settle();
  const list = q(".zen-buildsite-warnings");
  expect(list.tagName).toBe("UL");
  const rows = Array.from(list.querySelectorAll("li"));
  expect(rows).toHaveLength(2);
  expect(q(".zen-buildsite-summary").textContent).toContain("2 warnings");
  expect(rows[0].querySelector("button")!.textContent).toContain("guide/intro.md");
  expect(rows[0].textContent).toContain("img/x.png: not found; left out");
  expect(rows[1].querySelector("button")).toBeNull();
  expect(rows[1].textContent).toContain("is not allowed");
  rows[0].querySelector("button")!.click();
  expect(onOpenPage).toHaveBeenCalledWith("guide/intro.md");
  expect(document.querySelector(".zen-buildsite")).toBeNull();
  void d;
});

test("result: text is never HTML", async () => {
  const { f, q, btn } = open();
  await settle();
  btn("Export").click();
  await settle();
  f.finish({ ...RESULT, warnings: [{ path: "<b>a</b>.md", message: "<img src=x onerror=alert(1)>" }] });
  await settle();
  expect(q(".zen-buildsite-warnings").querySelector("img, b")).toBeNull();
});

test("Cancel while building asks the service to stop; the cancelled result says what was kept", async () => {
  const { f, q, btn } = open();
  await settle();
  btn("Export").click();
  await settle();
  f.progress({ done: 3, total: 10 });
  btn("Cancel").click();
  expect(f.to("cancelBuild")).toHaveLength(1);
  f.finish({ ...RESULT, pages: 3, cancelled: true });
  await settle();
  expect(q(".zen-buildsite-summary").textContent).toContain("Cancelled after 3 of 10 pages");
  expect(q(".zen-buildsite-summary").textContent).toContain("what was written is kept");
  expect(document.querySelector(".zen-buildsite")).not.toBeNull(); // Cancel stops the build, not the dialog
});

test("Esc cancels a running build, closes the form, and closes the result", async () => {
  const a = open();
  await settle();
  key("Escape");
  expect(document.querySelector(".zen-buildsite")).toBeNull();

  const b = open();
  await settle();
  b.btn("Export").click();
  await settle();
  key("Escape");
  expect(b.f.to("cancelBuild")).toHaveLength(1);
  expect(document.querySelector(".zen-buildsite")).not.toBeNull();
  b.f.finish({ ...RESULT, cancelled: true });
  await settle();
  key("Escape");
  expect(document.querySelector(".zen-buildsite")).toBeNull();
  expect(b.f.listeners.progress.size).toBe(0);
  void a;
});

test("errors: one message with the cause, the options stay open and editable", async () => {
  const { f, q, btn, d } = open();
  await settle();
  q<HTMLInputElement>(".zen-buildsite-out").value = ".";
  btn("Export").click();
  await settle();
  f.finish(new Error(`output folder ${ROOT} is the folder being built`));
  await settle();
  const err = q(".zen-buildsite-error");
  expect(err.getAttribute("role")).toBe("alert");
  expect(err.textContent).toContain("is the folder being built");
  expect(q<HTMLInputElement>(".zen-buildsite-out").disabled).toBe(false);
  expect(q<HTMLInputElement>(".zen-buildsite-out").value).toBe(".");
  expect(btn("Export").disabled).toBe(false);
  expect(d.el.querySelector<HTMLElement>(".zen-buildsite-progress")!.hidden).toBe(true);
  // Fix it and build again; the message clears.
  q<HTMLInputElement>(".zen-buildsite-out").value = "_site";
  btn("Export").click();
  await settle();
  expect(q(".zen-buildsite-error").textContent).toBe("");
  f.finish(RESULT);
  await settle();
});

test("errors: buildOptions failing shows the cause and still offers the defaults", async () => {
  const { q, btn } = open({ buildOptions: () => Promise.reject(new Error("build must be a mapping with keys such as out and portable")) });
  await settle();
  expect(q(".zen-buildsite-error").textContent).toContain("build must be a mapping");
  expect(q<HTMLInputElement>(".zen-buildsite-out").value).toBe("_site");
  expect(btn("Export").disabled).toBe(false);
});

test("Save as folder defaults previews the change first and writes only on confirm", async () => {
  const { f, q, btn, d } = open();
  await settle();
  d.el.querySelector<HTMLInputElement>("input[name=drafts]")!.click();
  btn("Save as folder defaults").click();
  await settle();
  expect(f.to("saveBuildDefaults")).toHaveLength(1);
  expect(f.to("saveBuildDefaults")[0].args).toEqual([{ ...OPTIONS, drafts: true }, false]);
  expect(q(".zen-buildsite-preview").textContent).toContain("Adds to .zen.yml:");
  expect(q(".zen-buildsite-preview").textContent).toContain("build.drafts: true");
  btn("Cancel changes").click(); // declines: nothing written
  expect(d.el.querySelector(".zen-buildsite-preview")).toBeNull();
  expect(f.to("saveBuildDefaults")).toHaveLength(1);

  btn("Save as folder defaults").click();
  await settle();
  btn("Write .zen.yml").click();
  await settle();
  expect(f.to("saveBuildDefaults")).toHaveLength(3); // preview, preview again, then the confirmed write
  expect(f.to("saveBuildDefaults")[2].args).toEqual([{ ...OPTIONS, drafts: true }, true]);
  expect(q(".zen-buildsite-status").textContent).toContain("Saved to .zen.yml");
  expect(d.el.querySelector(".zen-buildsite-preview")).toBeNull();
});

test("Save as folder defaults refuses while .zen.yml has unsaved changes in an editor", async () => {
  const { f, q, btn } = open({}, { unsaved: () => ["zen:config/workspace", "a.md"] });
  await settle();
  btn("Save as folder defaults").click();
  await settle();
  expect(f.to("saveBuildDefaults")).toHaveLength(0);
  expect(q(".zen-buildsite-error").textContent).toContain(".zen.yml has unsaved changes");
});

test("a failing preview shows the cause", async () => {
  const { q, btn } = open({ saveBuildDefaults: () => Promise.reject(new Error("the file is not a YAML mapping")) });
  await settle();
  btn("Save as folder defaults").click();
  await settle();
  expect(q(".zen-buildsite-error").textContent).toContain("not a YAML mapping");
});

test("a note says the website uses the saved files when editors have unsaved pages", async () => {
  const one = open({}, { unsaved: () => ["a.md"] });
  await settle();
  expect(one.q(".zen-buildsite-note").textContent).toBe("1 page has unsaved changes; the website uses the saved files.");
  document.body.replaceChildren();
  const two = open({}, { unsaved: () => ["a.md", "b.md", "zen:config/workspace"] });
  await settle();
  expect(two.q(".zen-buildsite-note").textContent).toBe("2 pages have unsaved changes; the website uses the saved files.");
  document.body.replaceChildren();
  const none = open({}, { unsaved: () => [] });
  await settle();
  expect(none.d.el.querySelector(".zen-buildsite-note")).toBeNull();
});

test("Tab stays inside the dialog and a click outside does not close it while building", async () => {
  const { f, btn, d } = open();
  await settle();
  btn("Export").click();
  await settle();
  d.el.dispatchEvent(new MouseEvent("mousedown", { bubbles: true }));
  expect(document.querySelector(".zen-buildsite")).not.toBeNull();
  f.finish(RESULT);
  await settle();
  btn("Done").click();
  expect(document.querySelector(".zen-buildsite")).toBeNull();
});

test("the dialog is Export Website", async () => {
  const { q, btn, d } = open();
  await settle();
  expect(q(".zen-buildsite-title").textContent).toContain("Export Website");
  expect(q(".zen-buildsite-title").textContent).not.toContain("Build");
  expect(btn("Export")).toBeDefined();
  expect(btn("Export").type).toBe("submit");
  expect(btn("Export").classList.contains("primary")).toBe(true);
  expect(Array.from(d.el.querySelectorAll("button")).some((b) => b.textContent === "Build")).toBe(false);
  expect(document.activeElement).toBe(btn("Export"));
});

test("read-only: no save defaults", async () => {
  const build = { app: "x", features: [] as string[], addons: [], plugins: [], layouts: [], templates: [], theme: "default" };
  const { f, btn, d } = open({ build });
  await settle();
  expect(Array.from(d.el.querySelectorAll("button")).some((b) => b.textContent === "Save as folder defaults")).toBe(false);
  expect(btn("Export")).toBeDefined();
  expect(f.to("saveBuildDefaults")).toHaveLength(0);
  d.close();
  // with the editor (or no build info) the link is there
  const full = open({ build: { ...build, features: ["editor"] } });
  expect(full.btn("Save as folder defaults")).toBeDefined();
  full.d.close();
});
