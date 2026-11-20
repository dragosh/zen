// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { createFakeApi, sampleWorkspace } from "../api/fake";
import type { Api, Startup } from "../api";
import { openNewSite, type NewSiteDialog, type SiteApi, type SiteTemplate } from "./newsite";
import { createShell, type Shell } from "./shell";

const tpl = (id: string, over: Partial<SiteTemplate> = {}): SiteTemplate => ({
  id, name: `Name of ${id}`, description: `About ${id}`, version: "1.0.0", builtin: true, update: "", error: "",
  ask: [{ key: "title", label: "Site title", default: "{{folder}}" }, { key: "author", label: "Author", default: "the team" }],
  ...over,
});
const TEMPLATES: SiteTemplate[] = [
  tpl("techdocs"),
  tpl("learning", { ask: [{ key: "title", label: "Course title", default: "" }] }),
  tpl("broken", { error: "template.yml: id is required" }),
  tpl("planner", { ask: [] }),
];

type Calls = { siteTemplates: unknown[][]; pickSiteFolder: unknown[][]; createSite: unknown[][]; revealTemplates: unknown[][] };
type Rig = SiteApi & {
  site: Calls; templates: SiteTemplate[]; folder: string;
  create: (id: string, dir: string, v: Record<string, string>) => Startup;
  fireNewSite(): void; fail: Partial<Record<keyof Calls, Error>>; hold?: Promise<void>;
};

const STARTUP: Startup = { workspace: { root: "/tmp/Acme", name: "Acme", home: "README.md", tree: [] }, page: "README.md", mode: "read" };

/** The fake api plus the methods and the event core/init adds. */
function rig(templates = TEMPLATES, startup?: Startup): Rig {
  const base = createFakeApi(startup ? { startup } : {});
  const listeners = new Set<() => void>();
  const r = base as unknown as Rig;
  const track = async <T>(name: keyof Calls, args: unknown[], run: () => T): Promise<T> => {
    r.site[name].push(args);
    const err = r.fail[name];
    if (err) { delete r.fail[name]; throw err; }
    if (name === "siteTemplates" && r.hold) await r.hold;
    return run();
  };
  const origOn = base.on.bind(base) as (e: string, f: (x: never) => void) => () => void;
  Object.assign(base, {
    site: { siteTemplates: [], pickSiteFolder: [], createSite: [], revealTemplates: [] } as Calls,
    templates, folder: "/Users/me/Sites/Acme Docs", fail: {},
    create: () => STARTUP,
    fireNewSite: () => { for (const f of Array.from(listeners)) f(); },
    siteTemplates: () => track("siteTemplates", [], () => r.templates.map((t) => ({ ...t }))),
    pickSiteFolder: () => track("pickSiteFolder", [], () => r.folder),
    createSite: (id: string, dir: string, v: Record<string, string>) => track("createSite", [id, dir, v], () => r.create(id, dir, v)),
    revealTemplates: () => track("revealTemplates", [], () => undefined),
    on: (event: string, fn: (x: never) => void) => {
      if (event === "menu:new-site") { listeners.add(fn as () => void); return () => listeners.delete(fn as () => void); }
      return origOn(event, fn);
    },
  });
  return r;
}

const settle = async () => { for (let i = 0; i < 6; i++) await new Promise((r) => setTimeout(r, 0)); };
let dialogs: NewSiteDialog[] = [];
beforeEach(() => { document.body.replaceChildren(); dialogs = []; });
afterEach(() => { for (const d of dialogs) d.close(); localStorage.removeItem("zen.developer"); });

function open(api: Rig, preselect?: string) {
  const onCreated = vi.fn();
  const onClose = vi.fn();
  const d = openNewSite({ api, doc: document, onCreated, onClose, preselect });
  dialogs.push(d);
  const q = <T extends Element = HTMLElement>(s: string) => d.el.querySelector<T>(s)!;
  const cards = () => Array.from(d.el.querySelectorAll<HTMLElement>(".zen-newsite-template"));
  const field = (key: string) => d.el.querySelector<HTMLInputElement>(`input[name="${key}"]`)!;
  const btn = (label: string) => Array.from(d.el.querySelectorAll("button")).find((b) => b.textContent === label)!;
  const names = () => cards().map((c) => c.dataset.template);
  return { d, q, cards, field, btn, names, onCreated, onClose };
}

test("loading, then a card for every template; the first valid one is chosen and its fields are shown", async () => {
  let release!: () => void;
  const api = rig();
  api.hold = new Promise((r) => { release = r; });
  const o = open(api);
  expect(o.q("[role=dialog]").getAttribute("aria-modal")).toBe("true");
  expect(o.q("[role=status]").textContent).toContain("Loading");
  expect(o.cards()).toHaveLength(0);
  expect(o.btn("Create").hasAttribute("disabled")).toBe(true);
  release();
  await settle();
  expect(o.d.el.querySelector("[role=status]")).toBeNull();
  expect(o.names()).toEqual(["techdocs", "learning", "broken", "planner"]);
  expect(o.q("[role=radiogroup]")).not.toBeNull();
  expect(o.cards()[0].querySelector(".zen-newsite-template-name")!.textContent).toBe("Name of techdocs");
  expect(o.cards()[0].querySelector(".zen-newsite-template-hint")!.textContent).toBe("About techdocs");
  expect(o.cards()[0].getAttribute("aria-checked")).toBe("true");
  expect(o.cards()[1].getAttribute("aria-checked")).toBe("false");
  // The fields of the template: title first, pre-filled; focus starts on the first one.
  expect(o.d.el.querySelectorAll("input")).toHaveLength(2);
  expect(o.field("title").labels?.[0].textContent).toBe("Site title");
  expect(o.field("author").value).toBe("the team");
  expect(document.activeElement).toBe(o.field("title"));
});

test("arrow keys and clicks choose; an invalid template is greyed out with its error and cannot be chosen", async () => {
  const o = open(rig());
  await settle();
  const broken = o.cards()[2];
  expect(broken.getAttribute("aria-disabled")).toBe("true");
  expect(broken.textContent).toContain("template.yml: id is required");
  expect(broken.tabIndex).toBe(-1);
  broken.click();
  expect(broken.getAttribute("aria-checked")).toBe("false");
  expect(o.cards()[0].getAttribute("aria-checked")).toBe("true");
  const press = (key: string) => o.q("[role=radiogroup]").dispatchEvent(new KeyboardEvent("keydown", { key, bubbles: true }));
  press("ArrowDown");
  expect(o.cards()[1].getAttribute("aria-checked")).toBe("true");
  expect(document.activeElement).toBe(o.cards()[1]);
  expect(o.d.el.querySelectorAll("input")).toHaveLength(1); // learning asks only for a title
  press("ArrowDown"); // skips the broken one
  expect(o.cards()[3].getAttribute("aria-checked")).toBe("true");
  expect(o.d.el.querySelectorAll("input")).toHaveLength(0); // planner asks for nothing
  press("ArrowRight"); // wraps
  expect(o.cards()[0].getAttribute("aria-checked")).toBe("true");
  press("ArrowUp");
  expect(o.cards()[3].getAttribute("aria-checked")).toBe("true");
  o.cards()[1].click();
  expect(o.cards()[1].getAttribute("aria-checked")).toBe("true");
});

test("preselect chooses that template, an unknown or invalid one is ignored", async () => {
  let o = open(rig(), "planner");
  await settle();
  expect(o.cards().find((c) => c.getAttribute("aria-checked") === "true")!.dataset.template).toBe("planner");
  o.d.close();
  o = open(rig(), "broken");
  await settle();
  expect(o.cards().find((c) => c.getAttribute("aria-checked") === "true")!.dataset.template).toBe("techdocs");
  o.d.close();
  o = open(rig(), "nosuch");
  await settle();
  expect(o.cards()[0].getAttribute("aria-checked")).toBe("true");
});

test("Create waits for a folder; the native panel picks it and the folder name fills the default title", async () => {
  const api = rig();
  const o = open(api);
  await settle();
  const create = o.btn("Create") as HTMLButtonElement;
  expect(create.disabled).toBe(true);
  expect(o.q(".zen-newsite-path").textContent).toBe("No folder chosen");
  expect(o.field("title").value).toBe(""); // the default is the folder's name, not known yet
  o.btn("Choose folder…").click();
  await settle();
  expect(api.site.pickSiteFolder).toHaveLength(1);
  expect(o.q(".zen-newsite-path").textContent).toBe("/Users/me/Sites/Acme Docs");
  expect(create.disabled).toBe(false);
  expect(o.field("title").value).toBe("Acme Docs");
  // A value the user typed is kept when another folder is chosen.
  o.field("title").value = "My title";
  o.field("title").dispatchEvent(new Event("input", { bubbles: true }));
  api.folder = "/Users/me/Other";
  o.btn("Choose folder…").click();
  await settle();
  expect(o.field("title").value).toBe("My title");
  expect(o.q(".zen-newsite-path").textContent).toBe("/Users/me/Other");
});

test("cancelling the folder panel changes nothing; a panel error is shown", async () => {
  const api = rig();
  const o = open(api);
  await settle();
  api.folder = "";
  o.btn("Choose folder…").click();
  await settle();
  expect(o.q(".zen-newsite-path").textContent).toBe("No folder chosen");
  expect((o.btn("Create") as HTMLButtonElement).disabled).toBe(true);
  api.fail.pickSiteFolder = new Error("panel broke");
  o.btn("Choose folder…").click();
  await settle();
  expect(o.q("[role=alert]").textContent).toContain("panel broke");
});

test("Create makes the site with the typed values, hands the result over and closes", async () => {
  const api = rig();
  api.create = vi.fn(() => STARTUP) as Rig["create"];
  const o = open(api);
  await settle();
  o.btn("Choose folder…").click();
  await settle();
  o.field("author").value = "Ana";
  o.q<HTMLFormElement>("form").requestSubmit();
  await settle();
  expect(api.site.createSite).toEqual([["techdocs", "/Users/me/Sites/Acme Docs", { title: "Acme Docs", author: "Ana" }]]);
  expect(o.onCreated).toHaveBeenCalledWith(STARTUP);
  expect(o.onClose).toHaveBeenCalledTimes(1);
  expect(o.d.el.isConnected).toBe(false);
});

test("Create does nothing without a folder; Enter in a field creates", async () => {
  const api = rig();
  const o = open(api);
  await settle();
  o.q<HTMLFormElement>("form").requestSubmit();
  await settle();
  expect(api.site.createSite).toHaveLength(0);
  o.btn("Choose folder…").click();
  await settle();
  o.field("title").dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true }));
  o.q<HTMLFormElement>("form").requestSubmit(); // what Enter in a text field does
  await settle();
  expect(api.site.createSite).toHaveLength(1);
});

test("files in the way: the error lists them, the dialog stays open and the folder stays chosen", async () => {
  const api = rig();
  api.create = () => { throw new Error("files already exist: README.md, .zenmd.yml"); };
  const o = open(api);
  await settle();
  o.btn("Choose folder…").click();
  await settle();
  o.q<HTMLFormElement>("form").requestSubmit();
  await settle();
  const alert = o.q("[role=alert]");
  expect(alert.textContent).toContain("files already exist: README.md, .zenmd.yml");
  expect(o.onCreated).not.toHaveBeenCalled();
  expect(o.d.el.isConnected).toBe(true);
  expect(o.q(".zen-newsite-path").textContent).toBe("/Users/me/Sites/Acme Docs");
  expect((o.btn("Create") as HTMLButtonElement).disabled).toBe(false);
  expect((o.btn("Cancel") as HTMLButtonElement).disabled).toBe(false);
  // Choosing another folder clears the message.
  api.folder = "/Users/me/Elsewhere";
  o.btn("Choose folder…").click();
  await settle();
  expect(o.q("[role=alert]").textContent).toBe("");
});

test("an error while writing is shown too, and the dialog can be used again", async () => {
  const api = rig();
  api.fail.createSite = new Error("no space left");
  const o = open(api);
  await settle();
  o.btn("Choose folder…").click();
  await settle();
  o.q<HTMLFormElement>("form").requestSubmit();
  await settle();
  expect(o.q("[role=alert]").textContent).toContain("no space left");
  o.q<HTMLFormElement>("form").requestSubmit();
  await settle();
  expect(o.onCreated).toHaveBeenCalledTimes(1);
});

test("no templates: says so and offers to reveal the folder", async () => {
  localStorage.setItem("zen.developer", "1");
  const api = rig([]);
  const o = open(api);
  await settle();
  expect(o.d.el.textContent).toContain("No templates");
  expect(o.cards()).toHaveLength(0);
  expect((o.btn("Create") as HTMLButtonElement).disabled).toBe(true);
  o.btn("Reveal in Finder").click();
  await settle();
  expect(api.site.revealTemplates).toHaveLength(1);
});

test("only invalid templates: nothing can be chosen", async () => {
  const o = open(rig([tpl("bad", { error: "nope" })]));
  await settle();
  expect(o.cards()).toHaveLength(1);
  expect(o.cards().every((c) => c.getAttribute("aria-checked") === "false")).toBe(true);
  expect((o.btn("Create") as HTMLButtonElement).disabled).toBe(true);
});

test("Open templates folder is hidden unless developer mode is on", async () => {
  let o = open(rig());
  await settle();
  expect(o.d.el.textContent).not.toContain("Open templates folder");
  o.d.close();
  localStorage.setItem("zen.developer", "1");
  o = open(rig());
  await settle();
  expect(o.d.el.textContent).toContain("Open templates folder");
  localStorage.removeItem("zen.developer");
});

test("empty state: Reveal in Finder only in developer mode", async () => {
  let o = open(rig([]));
  await settle();
  expect(o.d.el.textContent).toContain("No templates");
  expect(o.d.el.textContent).not.toContain("Reveal in Finder");
  o.d.close();
  localStorage.setItem("zen.developer", "1");
  o = open(rig([]));
  await settle();
  expect(o.d.el.textContent).toContain("Reveal in Finder");
  localStorage.removeItem("zen.developer");
});

test("a failing list is an error, and Open templates folder reveals the folder", async () => {
  localStorage.setItem("zen.developer", "1");
  const api = rig();
  api.fail.siteTemplates = new Error("cannot read");
  const o = open(api);
  await settle();
  expect(o.q("[role=alert]").textContent).toContain("cannot read");
  o.btn("Open templates folder").click();
  await settle();
  expect(api.site.revealTemplates).toHaveLength(1);
  api.fail.revealTemplates = new Error("no finder");
  o.btn("Open templates folder").click();
  await settle();
  expect(o.q("[role=alert]").textContent).toContain("no finder");
});

test("Esc, Cancel and a click outside close it; Tab stays inside", async () => {
  let o = open(rig());
  await settle();
  document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
  expect(o.onClose).toHaveBeenCalledTimes(1);
  expect(o.d.el.isConnected).toBe(false);
  o = open(rig());
  o.btn("Cancel").click();
  expect(o.onClose).toHaveBeenCalledTimes(1);
  o = open(rig());
  o.d.el.dispatchEvent(new MouseEvent("mousedown", { bubbles: true }));
  expect(o.onClose).toHaveBeenCalledTimes(1);
  o = open(rig());
  await settle();
  const last = Array.from(o.d.el.querySelectorAll<HTMLElement>("button:not([disabled])")).pop()!;
  last.focus();
  const tab = new KeyboardEvent("keydown", { key: "Tab", bubbles: true, cancelable: true });
  document.dispatchEvent(tab);
  expect(tab.defaultPrevented).toBe(true);
  expect(o.d.el.contains(document.activeElement)).toBe(true);
});

test("text from templates is shown as text, never as markup", async () => {
  const o = open(rig([tpl("x", { name: "<img src=x onerror=alert(1)>", description: "<b>bold</b>" })]));
  await settle();
  expect(o.d.el.querySelector("img, b")).toBeNull();
  expect(o.cards()[0].textContent).toContain("<img src=x onerror=alert(1)>");
});

// ---- the menu event, in the shell

let root: HTMLElement;
let shell: Shell | undefined;
afterEach(() => { shell?.destroy(); shell = undefined; root?.remove(); });

async function boot(api: Rig): Promise<Shell> {
  root = document.createElement("div");
  document.body.append(root);
  shell = createShell(root, { api: api as unknown as Api, features: [], search: "" });
  await shell.start();
  await settle();
  return shell;
}

test("menu event: menu:new-site opens the dialog from the welcome screen and from an open folder", async () => {
  const api = rig(TEMPLATES);
  await boot(api);
  expect(root.querySelector(".zen-welcome")).not.toBeNull();
  api.fireNewSite();
  await settle();
  expect(document.querySelector(".zen-newsite")).not.toBeNull();
  expect(document.querySelectorAll(".zen-newsite-template").length).toBe(4);
  api.fireNewSite(); // a second event does not stack a second dialog
  expect(document.querySelectorAll(".zen-newsite")).toHaveLength(1);
  // Create opens the new site in the window.
  const choose = Array.from(document.querySelectorAll("button")).find((b) => b.textContent === "Choose folder…")!;
  choose.click();
  await settle();
  document.querySelector<HTMLFormElement>(".zen-newsite")!.requestSubmit();
  await settle();
  expect(document.querySelector(".zen-newsite")).toBeNull();
  expect(root.querySelector(".zen-welcome")).toBeNull();
  expect(shell!.store.get().workspace?.root).toBe("/tmp/Acme");
  // And now, with a folder open, the same event opens it again.
  api.fireNewSite();
  await settle();
  expect(document.querySelector(".zen-newsite")).not.toBeNull();
});

test("menu event: with a folder open at startup", async () => {
  const api = rig(TEMPLATES, { workspace: sampleWorkspace(), page: "", mode: "read" });
  await boot(api);
  api.fireNewSite();
  await settle();
  expect(document.querySelector(".zen-newsite")).not.toBeNull();
  // Esc closes it without touching the folder.
  document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
  expect(document.querySelector(".zen-newsite")).toBeNull();
  expect(shell!.store.get().workspace?.root).toBe("/sample-docs");
});
