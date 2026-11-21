// @vitest-environment happy-dom
import { beforeEach, expect, test, vi } from "vitest";
import { createBuildForm, lastBuild, registerBuildSite, type BuildApi, type BuildOptions, type BuildProgress, type BuildResult } from "./buildsite";

const ROOT = "/Users/me/site";
const OPTIONS: BuildOptions = { out: `${ROOT}/_site`, portable: true, drafts: false, clean: false, print: true, scheme: "auto" };
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
    saveBuildDefaults: rec("saveBuildDefaults", () => Promise.resolve("Adds to .zenmd.yml:\n  export.drafts: true")),
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

/** The form, in the page (not a dialog), after its first refresh. */
async function open(over: Partial<BuildApi> = {}, extra: { unsaved?: () => string[]; root?: () => string } = {}) {
  const f = fakeApi(over);
  const onOpenPage = vi.fn();
  const d = createBuildForm({ api: f.api, doc: document, root: extra.root ?? (() => ROOT), onOpenPage, unsaved: extra.unsaved });
  document.body.append(d.el);
  await d.refresh();
  const q = <T extends Element = HTMLElement>(s: string) => d.el.querySelector<T>(s)!;
  const btn = (label: string) => Array.from(d.el.querySelectorAll<HTMLButtonElement>("button")).find((b) => b.textContent === label)!;
  return { f, d, q, btn, onOpenPage };
}

test("export in place: menu: the event asks for Settings › Export, once a folder is open; no modal opens", async () => {
  const f = fakeApi();
  let root = "";
  const show = vi.fn();
  const reg = registerBuildSite({ api: f.api, doc: document, root: () => root, name: () => "site", onOpenPage: vi.fn(), showExport: show });
  f.menu();
  expect(show).not.toHaveBeenCalled(); // no folder open
  root = ROOT;
  f.menu();
  expect(show).toHaveBeenCalledTimes(1);
  reg.open();
  expect(show).toHaveBeenCalledTimes(2);
  expect(document.querySelector(".zen-buildsite-backdrop, [role=dialog]")).toBeNull();
  reg.destroy();
  expect(f.listeners.menu.size).toBe(0);
});

test("the form is in the page, not a dialog, and is filled from buildOptions: output relative inside the folder", async () => {
  const { q, btn, d } = await open();
  expect(d.el.getAttribute("role")).not.toBe("dialog");
  expect(d.el.getAttribute("aria-modal")).toBeNull();
  expect(d.el.querySelector(".zen-buildsite-title")).toBeNull(); // the title is Settings' h3
  expect(q<HTMLInputElement>(".zen-buildsite-out").value).toBe("_site");
  const radios = Array.from(d.el.querySelectorAll<HTMLInputElement>("[data-field=opens] input[type=radio]"));
  expect(radios.map((r) => [r.value, r.checked])).toEqual([["portable", true], ["hosted", false]]);
  const boxes = Array.from(d.el.querySelectorAll<HTMLInputElement>("input[type=checkbox]"));
  expect(boxes.map((b) => [b.name, b.checked])).toEqual([["drafts", false], ["clean", false], ["print", true]]);
  expect(btn("Export").type).toBe("submit");
  expect(btn("Export").classList.contains("primary")).toBe(true);
  expect(Array.from(d.el.querySelectorAll("button")).some((b) => b.textContent === "Build")).toBe(false);
  d.focus();
  expect(document.activeElement).toBe(btn("Export"));
});

test("an output outside the folder is shown absolute", async () => {
  const { q } = await open({ buildOptions: () => Promise.resolve({ ...OPTIONS, out: "/Volumes/web/site" }) });
  expect(q<HTMLInputElement>(".zen-buildsite-out").value).toBe("/Volumes/web/site");
});

test("no folder: the form says so and cannot build", async () => {
  const { f, d, btn } = await open({}, { root: () => "" });
  expect(d.el.textContent).toContain("Open a folder to export it.");
  expect(btn("Export").disabled).toBe(true);
  expect(f.to("buildOptions")).toHaveLength(0);
});

test("Choose… fills the output from the folder panel; cancelling keeps it", async () => {
  const picks = ["/Volumes/web/out", `${ROOT}/public`, ""];
  const { q, btn } = await open({ pickBuildFolder: () => Promise.resolve(picks.shift()!) });
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

test("Export passes the options, shows progress with Cancel, and disables the form", async () => {
  const { f, q, btn, d } = await open();
  expect(d.el.querySelector<HTMLElement>(".zen-buildsite-progress")!.hidden).toBe(true);
  q<HTMLInputElement>(".zen-buildsite-out").value = "docs/out";
  d.el.querySelector<HTMLInputElement>("input[value=hosted]")!.click();
  d.el.querySelector<HTMLInputElement>("input[name=drafts]")!.click();
  btn("Export").click();
  await settle();
  expect(f.to("buildWebsite")[0].args[0]).toEqual({ out: `${ROOT}/docs/out`, portable: false, drafts: true, clean: false, print: true, scheme: "auto" });
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
  expect(d.busy()).toBe(true);
  d.el.dispatchEvent(new Event("submit", { cancelable: true })); // Enter does not start a second build
  expect(f.to("buildWebsite")).toHaveLength(1);
  f.finish(RESULT);
  await settle();
  expect(d.busy()).toBe(false);
});

test("result: the summary with the time, an announced status, and the actions", async () => {
  const { f, q, btn, d } = await open();
  btn("Export").click();
  await settle();
  f.finish(RESULT);
  await settle();
  const status = q(".zen-buildsite-summary");
  expect(status.getAttribute("role")).toBe("status");
  expect(status.textContent).toContain("Built 34 pages and 19 files into _site");
  expect(status.textContent).toContain("1.2 s");
  expect(d.el.querySelector(".zen-buildsite-warnings")).toBeNull();
  expect(Array.from(d.el.querySelectorAll<HTMLButtonElement>(".zen-buildsite-actions button")).map((b) => b.textContent)).toEqual(["Open in Browser", "Show in Finder", "Build Again"]);
  expect(btn("Open in Browser").classList.contains("primary")).toBe(true);
  expect(lastBuild(ROOT)!.result).toEqual(RESULT);
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

test("result: warnings are buttons by page that open the page; site-wide ones are plain; text is never HTML", async () => {
  const { f, q, btn, d, onOpenPage } = await open();
  btn("Export").click();
  await settle();
  f.finish({ ...RESULT, warnings: [{ path: "guide/intro.md", message: "img/x.png: not found; left out" }, { path: "", message: "plugin \"hello\" is not allowed" }, { path: "<b>a</b>.md", message: "<img src=x onerror=alert(1)>" }] });
  await settle();
  const rows = Array.from(q(".zen-buildsite-warnings").querySelectorAll("li"));
  expect(rows).toHaveLength(3);
  expect(q(".zen-buildsite-summary").textContent).toContain("3 warnings");
  expect(rows[0].querySelector("button")!.textContent).toContain("guide/intro.md");
  expect(rows[1].querySelector("button")).toBeNull();
  expect(d.el.querySelector(".zen-buildsite-warnings img, .zen-buildsite-warnings b")).toBeNull();
  rows[0].querySelector("button")!.click();
  expect(onOpenPage).toHaveBeenCalledWith("guide/intro.md");
});

test("Cancel asks the service to stop; the cancelled result says what was kept; Esc cancels too", async () => {
  const { f, q, btn, d } = await open();
  btn("Export").click();
  await settle();
  f.progress({ done: 3, total: 10 });
  btn("Cancel").click();
  expect(f.to("cancelBuild")).toHaveLength(1);
  const esc = new KeyboardEvent("keydown", { key: "Escape", bubbles: true, cancelable: true });
  d.el.dispatchEvent(esc); // already cancelling: nothing twice, but Esc is taken (it does not leave Settings mid-build)
  expect(f.to("cancelBuild")).toHaveLength(1);
  expect(esc.defaultPrevented).toBe(true);
  f.finish({ ...RESULT, pages: 3, cancelled: true });
  await settle();
  expect(q(".zen-buildsite-summary").textContent).toContain("Cancelled after 3 of 10 pages");
  expect(q(".zen-buildsite-summary").textContent).toContain("what was written is kept");
  const idle = new KeyboardEvent("keydown", { key: "Escape", bubbles: true, cancelable: true });
  d.el.dispatchEvent(idle); // idle: Esc is left to Settings
  expect(idle.defaultPrevented).toBe(false);
});

test("Esc while building cancels the build", async () => {
  const { f, btn, d } = await open();
  btn("Export").click();
  await settle();
  d.el.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true, cancelable: true }));
  expect(f.to("cancelBuild")).toHaveLength(1);
  f.finish({ ...RESULT, cancelled: true });
  await settle();
});

test("errors: one message with the cause, the options stay editable; buildOptions failing shows the cause", async () => {
  const { f, q, btn, d } = await open();
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
  q<HTMLInputElement>(".zen-buildsite-out").value = "_site";
  btn("Export").click();
  await settle();
  expect(q(".zen-buildsite-error").textContent).toBe("");
  f.finish(RESULT);
  await settle();
  const bad = await open({ buildOptions: () => Promise.reject(new Error("build must be a mapping with keys such as out and portable")) });
  expect(bad.q(".zen-buildsite-error").textContent).toContain("build must be a mapping");
  expect(bad.btn("Export").disabled).toBe(false);
});

test("Save as folder defaults previews the change first and writes only on confirm", async () => {
  const { f, q, btn, d } = await open();
  d.el.querySelector<HTMLInputElement>("input[name=drafts]")!.click();
  btn("Save as folder defaults").click();
  await settle();
  expect(f.to("saveBuildDefaults")[0].args).toEqual([{ ...OPTIONS, drafts: true }, false]);
  expect(q(".zen-buildsite-preview").textContent).toContain("export.drafts: true");
  btn("Cancel changes").click();
  expect(d.el.querySelector(".zen-buildsite-preview")).toBeNull();
  expect(f.to("saveBuildDefaults")).toHaveLength(1);
  btn("Save as folder defaults").click();
  await settle();
  btn("Write .zenmd.yml").click();
  await settle();
  expect(f.to("saveBuildDefaults")[2].args).toEqual([{ ...OPTIONS, drafts: true }, true]);
  expect(q(".zen-buildsite-status").textContent).toContain("Saved to .zenmd.yml");
});

test("Save as folder defaults refuses while .zenmd.yml has unsaved changes; a failing preview shows the cause", async () => {
  const { f, q, btn } = await open({}, { unsaved: () => ["zen:config/workspace", "a.md"] });
  btn("Save as folder defaults").click();
  await settle();
  expect(f.to("saveBuildDefaults")).toHaveLength(0);
  expect(q(".zen-buildsite-error").textContent).toContain(".zenmd.yml has unsaved changes");
  const bad = await open({ saveBuildDefaults: () => Promise.reject(new Error("the file is not a YAML mapping")) });
  bad.btn("Save as folder defaults").click();
  await settle();
  expect(bad.q(".zen-buildsite-error").textContent).toContain("not a YAML mapping");
});

test("a note says the website uses the saved files when editors have unsaved pages", async () => {
  expect((await open({}, { unsaved: () => ["a.md"] })).q(".zen-buildsite-note").textContent).toBe("1 page has unsaved changes; the website uses the saved files.");
  expect((await open({}, { unsaved: () => ["a.md", "b.md", "zen:config/workspace"] })).q(".zen-buildsite-note").textContent).toBe("2 pages have unsaved changes; the website uses the saved files.");
  expect((await open({}, { unsaved: () => [] })).d.el.querySelector(".zen-buildsite-note:not(:empty)")).toBeNull();
});

test("read-only: no save defaults; with the editor the link is there; destroy drops the progress listener", async () => {
  const build = { app: "x", features: [] as string[], addons: [], plugins: [], layouts: [], templates: [], dialects: [], theme: "default" };
  const ro = await open({ build });
  expect(Array.from(ro.d.el.querySelectorAll("button")).some((b) => b.textContent === "Save as folder defaults")).toBe(false);
  expect(ro.btn("Export")).toBeDefined();
  const full = await open({ build: { ...build, features: ["editor"] } });
  expect(full.btn("Save as folder defaults")).toBeDefined();
  expect(full.f.listeners.progress.size).toBe(1);
  full.d.destroy();
  expect(full.f.listeners.progress.size).toBe(0);
});

test("the default output is exports/html: before the options arrive, and for an emptied field", async () => {
  // buildOptions never answers: the field shows the default meanwhile.
  const f = fakeApi({ buildOptions: () => new Promise<BuildOptions>(() => {}) });
  const d = createBuildForm({ api: f.api, doc: document, root: () => ROOT, onOpenPage: vi.fn() });
  document.body.append(d.el);
  void d.refresh();
  expect(d.el.querySelector<HTMLInputElement>(".zen-buildsite-out")!.value).toBe("exports/html");
  document.body.replaceChildren();
  // An emptied field builds into exports/html in the folder.
  const { f: f2, q, btn } = await open();
  q<HTMLInputElement>(".zen-buildsite-out").value = "  ";
  btn("Export").click();
  await settle();
  expect(f2.to("buildWebsite")[0].args[0]).toMatchObject({ out: `${ROOT}/exports/html` });
});

// The Theme row (engine/export-cli-parity): Auto, Light (white) or Dark (black), a radio group of the shared form pieces,
// filled from the folder's export.theme.scheme, built with, and saved by Save as folder defaults.
test("Theme: a radio group Auto, Light (white), Dark (black); auto by default, filled from the options, passed to the build and the save", async () => {
  const { f, d, btn } = await open();
  const group = d.el.querySelector<HTMLElement>("[data-field=scheme]")!;
  expect(group.getAttribute("role")).toBe("radiogroup");
  const label = document.getElementById(group.getAttribute("aria-labelledby")!)!;
  expect(label.textContent).toBe("Theme");
  const radios = Array.from(group.querySelectorAll<HTMLInputElement>("input[type=radio]"));
  expect(radios.map((r) => [r.value, r.checked])).toEqual([["auto", true], ["light", false], ["dark", false]]);
  expect(Array.from(group.querySelectorAll(".zen-buildsite-choice-name")).map((n) => n.textContent)).toEqual(["Auto", "Light (white)", "Dark (black)"]);
  // The same pieces as the other radio group: one class for the group and one for each choice.
  expect(group.className).toBe(d.el.querySelector("[data-field=opens]")!.className);
  radios[2].click();
  btn("Export").click();
  await settle();
  expect(f.to("buildWebsite")[0].args[0]).toMatchObject({ scheme: "dark" });
  f.finish(RESULT);
  await settle();
  btn("Build Again").click();
  btn("Save as folder defaults").click();
  await settle();
  expect(f.to("saveBuildDefaults")[0].args).toEqual([expect.objectContaining({ scheme: "auto" }), false]);
});

test("Theme: the folder's scheme is shown, and the radios are locked while building", async () => {
  const { f, d, btn } = await open({ buildOptions: () => Promise.resolve({ ...OPTIONS, scheme: "light" }) });
  await settle();
  const radios = Array.from(d.el.querySelectorAll<HTMLInputElement>("[data-field=scheme] input[type=radio]"));
  expect(radios.map((r) => r.checked)).toEqual([false, true, false]);
  btn("Export").click();
  await settle();
  expect(radios.every((r) => r.disabled)).toBe(true);
  f.finish(RESULT);
});
