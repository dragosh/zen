// @vitest-environment happy-dom
// The option table (apps/zenmark/export-options.json) against the two Settings forms: every option of the table that
// has a field is a field of its form, with the same default, and the forms have no option the table lacks.
import { beforeEach, expect, test, vi } from "vitest";
import table from "../../../export-options.json";
import { createBuildForm, type BuildApi, type BuildOptions } from "./buildsite";
import { createPdfForm, type PdfApi, type PdfOptions } from "./pdfform";

type Entry = { flag: string; negative?: string; key: string | null; field: string | null; default: unknown; help: string };
const site = table.site as Entry[];
const pdf = table.pdf as Entry[];
const ROOT = "/Users/me/site";

beforeEach(() => document.body.replaceChildren());

/** A form that never hears from its api: it shows its own defaults. */
async function siteForm() {
  const api = { buildOptions: () => new Promise<BuildOptions>(() => {}), on: () => () => {} } as unknown as BuildApi;
  const d = createBuildForm({ api, doc: document, root: () => ROOT, onOpenPage: vi.fn() });
  document.body.append(d.el);
  void d.refresh();
  return d.el;
}
async function pdfForm() {
  const api = { pdfDefaults: () => new Promise<PdfOptions>(() => {}), on: () => () => {} } as unknown as PdfApi;
  const d = createPdfForm({ api, doc: document, root: () => ROOT, currentPage: () => "a.md" });
  document.body.append(d.el);
  void d.refresh();
  return d.el;
}

const appDefault = (v: unknown) => (v && typeof v === "object" && "app" in v ? (v as { app: unknown }).app : v);

/** The value the form shows for a field, in the table's terms. */
function shown(form: HTMLElement, field: string): unknown {
  const el = form.querySelector<HTMLElement>(`[data-field="${field}"]`);
  if (!el) return undefined;
  if (el instanceof HTMLInputElement && el.type === "checkbox") return el.checked;
  if (el instanceof HTMLInputElement) return el.value;
  const on = Array.from(el.querySelectorAll<HTMLInputElement>("input[type=radio]")).find((r) => r.checked);
  const value = on?.value;
  if (field === "opens") return value === "portable"; // the table says portable: true
  return value;
}

for (const [name, entries, open] of [["website", site, siteForm], ["PDF", pdf, pdfForm]] as const) {
  test(`every option of the table with a field is in the ${name} form, with its default`, async () => {
    const form = await open();
    const withField = entries.filter((e) => e.field !== null);
    expect(withField.length).toBeGreaterThanOrEqual(4);
    for (const e of withField) {
      const got = shown(form, e.field!);
      if (got === undefined) throw new Error(`${name} form has no field ${e.field} (${e.flag})`);
      const want = appDefault(e.default);
      // A path default may name the file inside the folder; the form starts from the folder.
      if (typeof want === "string" && e.field === "out") expect(want.startsWith(got as string), `${e.field}: ${got} vs ${want}`).toBe(true);
      else expect(got, `${name} field ${e.field}`).toEqual(want);
    }
  });

  test(`the ${name} form has no field that is not in the table`, async () => {
    const form = await open();
    const known = new Set(entries.map((e) => e.field).filter((f) => f !== null));
    const fields = Array.from(form.querySelectorAll<HTMLElement>("[data-field]")).map((e) => e.dataset.field!);
    expect(fields.length).toBeGreaterThanOrEqual(4);
    for (const f of fields) expect(known.has(f), `${name} form field ${f} is not in the table`).toBe(true);
  });
}

test("the table is well-formed: a flag, a help line, and a key that names its mapping", () => {
  for (const [target, entries] of [["site", site], ["pdf", pdf]] as const) {
    const flags = new Set<string>();
    for (const e of entries) {
      expect(e.flag).toMatch(/^--[a-z][a-z-]*$/);
      expect(flags.has(e.flag), `${target} ${e.flag} twice`).toBe(false);
      flags.add(e.flag);
      expect(e.help.length).toBeGreaterThan(10);
      if (e.negative) expect(e.negative).toMatch(/^--[a-z][a-z-]*$/);
      if (e.key !== null) expect(e.key).toMatch(target === "site" ? /^export\.(?!pdf\.)[a-z.]+$/ : /^export\.pdf\.[a-z]+$/);
    }
  }
  // The scheme is the website's, and --open's two defaults are recorded.
  expect(site.find((e) => e.key === "export.theme.scheme")?.default).toBe("auto");
  expect(pdf.find((e) => e.flag === "--open")?.default).toEqual({ cli: false, app: true });
});
