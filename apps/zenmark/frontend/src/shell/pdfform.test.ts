// @vitest-environment happy-dom
import { beforeEach, expect, test } from "vitest";
import { createPdfForm, type PdfApi, type PdfOptions } from "./pdfform";

const ROOT = "/Users/me/Guide";
const DEFAULTS: PdfOptions = { out: `${ROOT}/exports/pdf/Guide.pdf`, drafts: false, cover: true, open: true };

/** A local fake of the PDF part of the api. runPdfExport returns at once; the result comes as the pdf:export event. */
function fakeApi(over: Partial<PdfApi> = {}) {
  const calls: { name: string; args: unknown[] }[] = [];
  const listeners = new Set<(r: { path?: string; error?: string }) => void>();
  const rec = <T>(name: string, value: () => T) => (...args: unknown[]) => { calls.push({ name, args }); return value(); };
  const api: PdfApi = {
    pdfDefaults: rec("pdfDefaults", () => Promise.resolve({ ...DEFAULTS })),
    savePdfDefaults: rec("savePdfDefaults", () => Promise.resolve("Adds to .zenmd.yml:\n  export.pdf.cover: false")),
    pickPdfFile: rec("pickPdfFile", () => Promise.resolve("")),
    runPdfExport: rec("runPdfExport", () => Promise.resolve()),
    previewPdf: rec("previewPdf", () => Promise.resolve()),
    revealExport: rec("revealExport", () => Promise.resolve()),
    openExport: rec("openExport", () => Promise.resolve()),
    on: ((_event: string, fn: (r: { path?: string; error?: string }) => void) => { listeners.add(fn); return () => { listeners.delete(fn); }; }) as unknown as PdfApi["on"],
    ...over,
  };
  return {
    api, calls,
    to: (name: string) => calls.filter((c) => c.name === name),
    emit: (r: { path?: string; error?: string }) => listeners.forEach((f) => f(r)),
    listeners,
  };
}

const settle = async () => { for (let i = 0; i < 6; i++) await new Promise((r) => setTimeout(r, 0)); };
beforeEach(() => document.body.replaceChildren());

async function open(over: Partial<PdfApi> = {}, extra: { root?: () => string; currentPage?: () => string; unsaved?: () => string[] } = {}) {
  const f = fakeApi(over);
  const d = createPdfForm({ api: f.api, doc: document, root: extra.root ?? (() => ROOT), currentPage: extra.currentPage ?? (() => "docs/a.md"), unsaved: extra.unsaved });
  document.body.append(d.el);
  await d.refresh();
  const q = <T extends Element = HTMLElement>(s: string) => d.el.querySelector<T>(s)!;
  const btn = (label: string) => Array.from(d.el.querySelectorAll<HTMLButtonElement>("button")).find((b) => b.textContent === label)!;
  const box = (name: string) => d.el.querySelector<HTMLInputElement>(`input[name=${name}]`)!;
  return { f, d, q, btn, box };
}

test("the fields and their defaults, shown folder-relative", async () => {
  const { q, box, btn, d } = await open();
  expect(q<HTMLInputElement>(".zen-buildsite-out").value).toBe("exports/pdf/Guide.pdf");
  expect(d.el.querySelector("label.zen-buildsite-label")!.textContent).toBe("Output file");
  expect(Array.from(d.el.querySelectorAll<HTMLInputElement>("input[type=radio]")).map((r) => [r.value, r.checked, r.disabled])).toEqual([["all", true, false], ["section", false, false]]);
  expect(Array.from(d.el.querySelectorAll<HTMLInputElement>("input[type=checkbox]")).map((b) => [b.name, b.checked])).toEqual([["drafts", false], ["cover", true], ["open", true]]);
  expect(box("cover").parentElement!.textContent).toContain("Cover and contents");
  expect(box("open").parentElement!.textContent).toContain("Open when done");
  expect(btn("Choose…")).toBeDefined();
  expect(btn("Export PDF").classList.contains("primary")).toBe(true);
  expect(btn("Export PDF").type).toBe("submit");
  expect(btn("Preview…").classList.contains("primary")).toBe(false);
});

test("This section is off without an open page", async () => {
  const { d } = await open({}, { currentPage: () => "" });
  expect(d.el.querySelector<HTMLInputElement>("input[value=section]")!.disabled).toBe(true);
});

test("no folder: the form is disabled and the left column's words are said", async () => {
  const { d, btn } = await open({}, { root: () => "" });
  expect(d.el.textContent).toContain("Open a folder to export it.");
  expect(btn("Export PDF").disabled).toBe(true);
});

test("Export PDF runs with the form's options: no panel, progress, then the result with Show in Finder and Open", async () => {
  const { f, d, q, btn, box } = await open();
  box("drafts").checked = true;
  box("cover").checked = false;
  box("open").checked = false;
  d.el.querySelector<HTMLInputElement>("input[value=section]")!.checked = true;
  q<HTMLInputElement>(".zen-buildsite-out").value = "pdf/Mine";
  btn("Export PDF").click();
  await settle();
  expect(f.to("runPdfExport")[0].args).toEqual([{ out: "pdf/Mine", drafts: true, cover: false, open: false, section: true }]);
  expect(f.to("previewPdf")).toHaveLength(0);
  expect(q("[role=progressbar]")).not.toBeNull();
  expect(d.busy()).toBe(true);
  f.emit({ path: `${ROOT}/pdf/Mine.pdf` });
  await settle();
  expect(d.busy()).toBe(false);
  expect(q(".zen-buildsite-summary").textContent).toContain("Mine.pdf");
  btn("Show in Finder").click();
  btn("Open").click();
  await settle();
  expect(f.to("revealExport")).toHaveLength(1);
  expect(f.to("openExport")).toHaveLength(1);
  btn("Export Again").click();
  expect(q<HTMLInputElement>(".zen-buildsite-out")).not.toBeNull();
  expect(btn("Export PDF").disabled).toBe(false);
});

test("a failure shows in the status line and the form stays", async () => {
  const { f, d, q, btn } = await open();
  btn("Export PDF").click();
  await settle();
  f.emit({ error: "disk full" });
  await settle();
  expect(q(".zen-buildsite-error").textContent).toContain("disk full");
  expect(d.busy()).toBe(false);
  expect(btn("Export PDF").disabled).toBe(false);
  // The call itself failing (a path outside the folder) shows too.
  const g = await open({ runPdfExport: () => Promise.reject(new Error("../x.pdf is outside the folder")) });
  g.btn("Export PDF").click();
  await settle();
  expect(g.q(".zen-buildsite-error").textContent).toContain("outside the folder");
  expect(g.d.busy()).toBe(false);
});

test("Preview… opens the print preview with the same options", async () => {
  const { f, btn, box } = await open();
  box("drafts").checked = true;
  btn("Preview…").click();
  await settle();
  expect(f.to("previewPdf")[0].args).toEqual([{ out: "exports/pdf/Guide.pdf", drafts: true, cover: true, open: true, section: false }]);
  expect(f.to("runPdfExport")).toHaveLength(0);
});

test("Choose… takes the save panel's file; a cancel leaves the field", async () => {
  const { f, q, btn } = await open({ pickPdfFile: () => Promise.resolve(`${ROOT}/books/Mine.pdf`) });
  btn("Choose…").click();
  await settle();
  expect(q<HTMLInputElement>(".zen-buildsite-out").value).toBe("books/Mine.pdf");
  void f;
  const g = await open();
  g.btn("Choose…").click();
  await settle();
  expect(g.q<HTMLInputElement>(".zen-buildsite-out").value).toBe("exports/pdf/Guide.pdf");
});

test("Save as folder defaults previews, then writes; an editor with unsaved .zenmd.yml blocks it", async () => {
  const { f, q, btn, box } = await open();
  box("cover").checked = false;
  btn("Save as folder defaults").click();
  await settle();
  expect(f.to("savePdfDefaults")[0].args).toEqual([{ out: "exports/pdf/Guide.pdf", drafts: false, cover: false, open: true }, false]);
  expect(q(".zen-buildsite-preview").textContent).toContain("export.pdf.cover");
  btn("Write .zenmd.yml").click();
  await settle();
  expect(f.to("savePdfDefaults")[1].args[1]).toBe(true);
  expect(q(".zen-buildsite-status").textContent).toBe("Saved to .zenmd.yml.");
  const g = await open({}, { unsaved: () => ["zen:config/workspace"] });
  g.btn("Save as folder defaults").click();
  await settle();
  expect(g.f.to("savePdfDefaults")).toHaveLength(0);
  expect(g.q(".zen-buildsite-error").textContent).toContain("unsaved");
});

test("a read-only build has no Save as folder defaults", async () => {
  const { d } = await open({ build: { features: ["reader", "export", "print"] } as PdfApi["build"] });
  expect(Array.from(d.el.querySelectorAll("button")).some((b) => b.textContent === "Save as folder defaults")).toBe(false);
});

test("the layout is the website form's: options, then buttons (primary first, defaults last), then the status", async () => {
  const { d } = await open();
  const order = Array.from(d.el.children).map((c) => c.className.split(" ")[0]);
  expect(order.indexOf("zen-buildsite-actions-form")).toBeGreaterThan(order.indexOf("zen-buildsite-options"));
  expect(order.indexOf("zen-buildsite-error")).toBeGreaterThan(order.indexOf("zen-buildsite-actions-form"));
  expect(order.indexOf("zen-buildsite-status")).toBeGreaterThan(order.indexOf("zen-buildsite-error"));
  const foot = Array.from(d.el.querySelector(".zen-buildsite-actions-form")!.children);
  expect(foot[0].classList.contains("primary")).toBe(true);
  expect(foot.at(-1)!.classList.contains("zen-buildsite-link")).toBe(true);
});
