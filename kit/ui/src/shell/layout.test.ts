// @vitest-environment happy-dom
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { beforeEach, expect, test } from "vitest";
import { INSPECTOR_CLOSE, INSPECTOR_STATE, INSPECTOR_TOGGLE } from "../inspector/events";
import { icon } from "./icons";
import { createLayout, navSideRow, setNavSide, SIDEBAR_DEFAULT, SIDEBAR_MAX, SIDEBAR_MIN, storedNavSide, WIDTH_KEY } from "./layout";

beforeEach(() => localStorage.clear());

test("has nav, header and main landmarks and the four slots", () => {
  const l = createLayout();
  // The nav slot is the sidebar's body; the sidebar adds Open… and the cog around it.
  expect(l.el.querySelector("nav")!.contains(l.slots.nav)).toBe(true);
  expect(l.el.querySelector("main")).toBe(l.slots.main);
  const header = l.el.querySelector("header")!;
  expect(header.contains(l.slots.breadcrumb)).toBe(true);
  expect(header.contains(l.slots.actions)).toBe(true);
});

test("sidebar starts at 260px and toggles", () => {
  const l = createLayout();
  expect(SIDEBAR_DEFAULT).toBe(260);
  expect(l.sidebarWidth()).toBe(260);
  expect(l.sidebarVisible()).toBe(true);
  l.toggleSidebar();
  expect(l.sidebarVisible()).toBe(false);
  expect(l.el.querySelector("nav")!.hidden).toBe(true);
  l.toggleSidebar();
  expect(l.el.querySelector("nav")!.hidden).toBe(false);
});

test("the sidebar has no Open button; the cog at the bottom reports itself", () => {
  let anchor: HTMLButtonElement | null = null;
  const l = createLayout({ onOpenFolder: () => {}, onSettings: (a) => { anchor = a; } });
  const nav = l.el.querySelector("nav")!;
  expect(l.el.querySelector(".zen-sidebar-open")).toBeNull();
  expect(nav.querySelector("button[aria-label='Open a folder']")).toBeNull();
  const cog = nav.querySelector<HTMLButtonElement>(".zen-sidebar-foot .zen-settings-button")!;
  expect(cog.getAttribute("aria-label")).toBe("Settings");
  expect(cog.querySelector("svg")).not.toBeNull();
  // Order: body (slot), then the footer last.
  expect(nav.lastElementChild!.classList.contains("zen-sidebar-foot")).toBe(true);
  cog.click();
  expect(anchor).toBe(cog);
});

test("the toggle button toggles the sidebar and reports its state", () => {
  const l = createLayout();
  const btn = l.el.querySelector<HTMLButtonElement>('button[data-action="toggle-sidebar"]')!;
  expect(btn.getAttribute("aria-expanded")).toBe("true");
  btn.click();
  expect(l.sidebarVisible()).toBe(false);
  expect(btn.getAttribute("aria-expanded")).toBe("false");
});

test("the toggle is the sidebar icon, as on an exported site (ui/site-topbar), with no text glyph", () => {
  const l = createLayout();
  const btn = l.el.querySelector<HTMLButtonElement>('button[data-action="toggle-sidebar"]')!;
  expect(btn.textContent).toBe("");
  expect(btn.querySelector("svg")!.outerHTML).toBe(icon(document, "sidebar").outerHTML);
  expect(btn.getAttribute("aria-label")).toBe("Toggle sidebar");
});

test("width is clamped to 200–400", () => {
  const l = createLayout();
  l.setSidebarWidth(50);
  expect(l.sidebarWidth()).toBe(SIDEBAR_MIN);
  l.setSidebarWidth(1000);
  expect(l.sidebarWidth()).toBe(SIDEBAR_MAX);
  l.setSidebarWidth(321.4);
  expect(l.sidebarWidth()).toBe(321);
  expect(l.el.style.getPropertyValue("--sidebar-width")).toBe("321px");
});

test("width is remembered in localStorage", () => {
  createLayout().setSidebarWidth(300);
  expect(localStorage.getItem(WIDTH_KEY)).toBe("300");
  expect(createLayout().sidebarWidth()).toBe(300);
});

test("a bad stored width falls back to a valid one", () => {
  localStorage.setItem(WIDTH_KEY, "banana");
  expect(createLayout().sidebarWidth()).toBe(260);
  localStorage.setItem(WIDTH_KEY, "9999");
  expect(createLayout().sidebarWidth()).toBe(400);
});

test("dragging the edge resizes within the limits", () => {
  const l = createLayout();
  document.body.append(l.el);
  const edge = l.el.querySelector<HTMLElement>('[role="separator"]')!;
  edge.dispatchEvent(new MouseEvent("mousedown", { clientX: 260, button: 0, bubbles: true }));
  window.dispatchEvent(new MouseEvent("mousemove", { clientX: 310 }));
  expect(l.sidebarWidth()).toBe(310);
  window.dispatchEvent(new MouseEvent("mousemove", { clientX: 900 }));
  expect(l.sidebarWidth()).toBe(400);
  window.dispatchEvent(new MouseEvent("mouseup", { clientX: 900 }));
  expect(localStorage.getItem(WIDTH_KEY)).toBe("400");
  window.dispatchEvent(new MouseEvent("mousemove", { clientX: 220 }));
  expect(l.sidebarWidth()).toBe(400);
  l.destroy();
  l.el.remove();
});

test("the edge is resizable from the keyboard", () => {
  const l = createLayout();
  const edge = l.el.querySelector<HTMLElement>('[role="separator"]')!;
  edge.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowRight" }));
  expect(l.sidebarWidth()).toBeGreaterThan(260);
  expect(edge.getAttribute("aria-valuenow")).toBe(String(l.sidebarWidth()));
});

test("showView swaps the main pane for a view; the sidebar and the header controls stay", () => {
  const l = createLayout();
  const view = document.createElement("section");
  l.showView(view);
  expect(l.el.querySelector(".zen-content")!.contains(view)).toBe(true);
  expect(l.slots.main.hidden).toBe(true);
  expect(l.slots.actions.hidden).toBe(true);
  expect(l.slots.breadcrumb.hidden).toBe(true);
  expect(l.el.querySelector("nav")!.hidden).toBe(false);
  expect(l.el.querySelector('button[aria-label="Back"]')).not.toBeNull();
  l.showView(null);
  expect(view.isConnected).toBe(false);
  expect(l.slots.main.hidden).toBe(false);
  expect(l.slots.actions.hidden).toBe(false);
  expect(l.slots.breadcrumb.hidden).toBe(false);
});

test("the cog reports whether Settings is open", () => {
  const l = createLayout();
  const cog = l.el.querySelector<HTMLButtonElement>(".zen-settings-button")!;
  expect(cog.hasAttribute("aria-haspopup")).toBe(false); // it opens a page, not a popup
  l.setSettingsOpen(true);
  expect(cog.getAttribute("aria-expanded")).toBe("true");
  l.setSettingsOpen(false);
  expect(cog.getAttribute("aria-expanded")).toBe("false");
});

const inspectorBtn = (l: ReturnType<typeof createLayout>) => l.el.querySelector<HTMLButtonElement>(".zen-inspector-button")!;
const state = (available: boolean, open: boolean, count: number) =>
  document.dispatchEvent(new CustomEvent(INSPECTOR_STATE, { detail: { available, open, count } }));

test("the Inspector button is an icon button last in the top bar's action group, hidden until a page offers one", () => {
  const l = createLayout();
  const group = l.el.querySelector(".zen-top-actions")!;
  expect(l.el.querySelector("header")!.contains(group)).toBe(true);
  expect(Array.from(group.children)).toEqual([l.slots.actions, inspectorBtn(l)]); // the page's actions, then the Inspector
  expect(l.el.querySelector(".zen-sidebar-foot")!.contains(inspectorBtn(l))).toBe(false);
  expect(inspectorBtn(l).classList.contains("zen-settings-button")).toBe(true);
  expect(inspectorBtn(l).querySelector("svg")).not.toBeNull();
  expect(inspectorBtn(l).title).toContain("⌥⌘I");
  expect(inspectorBtn(l).hidden).toBe(true);
  state(true, false, 0);
  expect(inspectorBtn(l).hidden).toBe(false);
  state(false, false, 0);
  expect(inspectorBtn(l).hidden).toBe(true);
  l.destroy();
});

test("the Inspector button asks the page to toggle and reflects open state", () => {
  const l = createLayout();
  state(true, false, 0);
  let n = 0;
  const on = () => n++;
  document.addEventListener(INSPECTOR_TOGGLE, on);
  inspectorBtn(l).click();
  expect(n).toBe(1);
  expect(inspectorBtn(l).getAttribute("aria-expanded")).toBe("false");
  state(true, true, 0);
  expect(inspectorBtn(l).getAttribute("aria-expanded")).toBe("true");
  document.removeEventListener(INSPECTOR_TOGGLE, on);
  l.destroy();
});

test("the Inspector button carries a red count badge, none at 0, and a name that says it", () => {
  const l = createLayout();
  const badge = () => inspectorBtn(l).querySelector<HTMLElement>(".zen-inspector-badge")!;
  state(true, false, 0);
  expect(badge().hidden).toBe(true);
  expect(inspectorBtn(l).getAttribute("aria-label")).toBe("Inspector");
  state(true, false, 3);
  expect(badge().hidden).toBe(false);
  expect(badge().textContent).toBe("3");
  expect(inspectorBtn(l).getAttribute("aria-label")).toBe("Inspector, 3 issues");
  state(true, false, 1);
  expect(inspectorBtn(l).getAttribute("aria-label")).toBe("Inspector, 1 issue");
  l.destroy();
  state(true, false, 5); // destroyed: no longer listens
  expect(badge().textContent).toBe("1");
});

test("the footer has Settings and Export only, and no Developer button: the Console is a tab of the Inspector (#25)", () => {
  const l = createLayout();
  expect(l.el.querySelector(".zen-developer-button")).toBeNull();
  const kids = Array.from(l.el.querySelector(".zen-sidebar-foot")!.children);
  expect(kids.map((k) => k.getAttribute("aria-label"))).toEqual(["Settings", "Export"]);
  expect(l.el.querySelector(".zen-sidebar-foot .zen-inspector-button")).toBeNull();
  expect("setDeveloperAvailable" in l || "setDeveloperOpen" in l).toBe(false);
  l.destroy();
});

// ---- ui/nav-panel: header, footer and side (#24)

const head = (l: ReturnType<typeof createLayout>) => l.el.querySelector<HTMLElement>(".zen-sidebar-head")!;

test("identity: the header has the logo, the title and a close button, inside the nav landmark, above the body", () => {
  const l = createLayout();
  const nav = l.el.querySelector("nav")!;
  expect(nav.firstElementChild).toBe(head(l));
  expect(nav.contains(head(l))).toBe(true);
  // The Zenmark mark until the folder gives a logo; no title yet.
  expect(head(l).querySelector("svg")).not.toBeNull();
  expect(head(l).querySelector("img")).toBeNull();

  l.setIdentity({ logo: "/_ws/img/logo.svg", title: "My Course" });
  const img = head(l).querySelector<HTMLImageElement>("img.zen-sidebar-logo")!;
  expect(img.getAttribute("src")).toBe("/_ws/img/logo.svg");
  expect(img.getAttribute("alt")).toBe("");
  expect(head(l).querySelector("svg")).toBeNull();
  const title = head(l).querySelector<HTMLElement>(".zen-sidebar-title")!;
  expect(title.textContent).toBe("My Course");
  expect(title.title).toBe("My Course");

  // Switching folders replaces both; no logo goes back to the mark.
  l.setIdentity({ logo: "", title: "Other" });
  expect(head(l).querySelector("img")).toBeNull();
  expect(head(l).querySelector("svg")).not.toBeNull();
  expect(head(l).querySelectorAll(".zen-sidebar-title")).toHaveLength(1);
  expect(head(l).querySelector(".zen-sidebar-title")!.textContent).toBe("Other");
  l.destroy();
});

test("identity: a very long title is one line, cut by CSS, with the full text as its tooltip; markup stays text", () => {
  const l = createLayout();
  const long = "A very long title ".repeat(30) + "<b>x</b>";
  l.setIdentity({ logo: "", title: long });
  const title = head(l).querySelector<HTMLElement>(".zen-sidebar-title")!;
  expect(title.title).toBe(long);
  expect(title.textContent).toBe(long);
  expect(title.querySelector("b")).toBeNull();
  l.destroy();
});

test("logo is an img: a logo that is an SVG with a script is never put inline", () => {
  const l = createLayout();
  l.setIdentity({ logo: '/_ws/evil.svg" onerror="alert(1)', title: "t" });
  expect(head(l).querySelector("script")).toBeNull();
  const img = head(l).querySelector<HTMLImageElement>("img")!;
  expect(img.hasAttribute("onerror")).toBe(false);
  expect(img.getAttribute("src")).toBe('/_ws/evil.svg" onerror="alert(1)');
  expect(head(l).querySelectorAll("svg")).toHaveLength(0); // no inline svg of the folder
  l.destroy();
});

test("the close button is Close folder, with its shortcut in the tooltip, and asks the shell", () => {
  let closed = 0;
  const l = createLayout({ onClose: () => { closed++; } });
  const x = head(l).querySelector<HTMLButtonElement>("button")!;
  expect(x.getAttribute("aria-label")).toBe("Close folder");
  expect(x.title).toBe("Close Folder ⇧⌘W");
  expect(x.classList.contains("ghost")).toBe(true);
  x.click();
  expect(closed).toBe(1);
  l.destroy();
});

test("footer buttons: Settings and Export with icons, tooltips and labels; the Inspector is in the top bar; Export asks the shell", () => {
  let exported = 0;
  const l = createLayout({ onExport: () => { exported++; } });
  const kids = Array.from(l.el.querySelector(".zen-sidebar-foot")!.children) as HTMLButtonElement[];
  expect(kids.map((b) => b.getAttribute("aria-label"))).toEqual(["Settings", "Export"]);
  expect(kids[0].title).toBe("Settings ⌘,");
  expect(kids[1].title).toBe("Export ⇧⌘B");
  const insp = l.el.querySelector<HTMLButtonElement>(".zen-top-actions .zen-inspector-button")!;
  expect(insp.getAttribute("aria-label")).toBe("Inspector");
  expect(insp.title).toBe("Inspector (⌥⌘I)");
  for (const b of [...kids, insp]) expect(b.querySelector("svg")).not.toBeNull();
  expect(l.el.querySelector(".zen-developer-button")).toBeNull();
  kids[1].click();
  expect(exported).toBe(1);
  // the separators are the footer's and header's own rules (panel.css); the three parts are in order
  const nav = l.el.querySelector("nav")!;
  expect(Array.from(nav.children).map((c) => c.className)).toEqual(["zen-sidebar-head", "zen-sidebar-body", "zen-sidebar-foot"]);
  l.destroy();
});

test("nav side: left by default, stored per machine, applied at once and when a layout is made", () => {
  expect(storedNavSide()).toBe("left");
  const l = createLayout();
  document.body.append(l.el);
  expect(l.el.dataset.navSide ?? "left").toBe("left");
  setNavSide("right", document);
  expect(localStorage.getItem("zen.navSide")).toBe("right");
  expect(storedNavSide()).toBe("right");
  expect(l.el.dataset.navSide).toBe("right");
  expect(createLayout().el.dataset.navSide).toBe("right");
  setNavSide("left", document);
  expect(l.el.dataset.navSide ?? "left").toBe("left");
  expect(storedNavSide()).toBe("left");
  // A bad stored value is the default.
  localStorage.setItem("zen.navSide", "up");
  expect(storedNavSide()).toBe("left");
  l.el.remove();
  l.destroy();
});

test("nav side: with Right the resize edge and the arrow keys are mirrored; the toggle works the same", () => {
  localStorage.setItem("zen.navSide", "right");
  const l = createLayout();
  document.body.append(l.el);
  l.el.getBoundingClientRect = () => ({ left: 0, right: 1000, top: 0, bottom: 600, width: 1000, height: 600, x: 0, y: 0, toJSON() {} }) as DOMRect;
  const edge = l.el.querySelector<HTMLElement>(".zen-resizer")!;
  edge.dispatchEvent(new MouseEvent("mousedown", { button: 0, bubbles: true, cancelable: true }));
  window.dispatchEvent(new MouseEvent("mousemove", { clientX: 700 })); // the sidebar is the part right of the pointer
  window.dispatchEvent(new MouseEvent("mouseup", { clientX: 700 }));
  expect(l.sidebarWidth()).toBe(300);
  l.setSidebarWidth(260);
  edge.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowLeft", bubbles: true, cancelable: true })); // towards the page = wider
  expect(l.sidebarWidth()).toBe(276);
  edge.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowRight", bubbles: true, cancelable: true }));
  expect(l.sidebarWidth()).toBe(260);
  l.toggleSidebar();
  expect(l.el.querySelector("nav")!.hidden).toBe(true);
  l.toggleSidebar();
  expect(l.el.querySelector("nav")!.hidden).toBe(false);
  l.el.remove();
  l.destroy();
});

test("the Navigation row is a labelled radiogroup, Left and Right, that follows and sets the side", () => {
  const row = navSideRow(document);
  document.body.append(row);
  const group = row.getAttribute("role") === "radiogroup" ? row : row.querySelector<HTMLElement>('[role="radiogroup"]')!;
  expect(group.getAttribute("aria-label") ?? document.getElementById(group.getAttribute("aria-labelledby") ?? "")?.textContent).toBe("Navigation");
  const radios = Array.from(group.querySelectorAll<HTMLInputElement>('input[type="radio"]'));
  expect(radios.map((r) => r.parentElement!.textContent)).toEqual(["Left", "Right"]);
  expect(radios.map((r) => r.checked)).toEqual([true, false]);
  const l = createLayout();
  document.body.append(l.el);
  radios[1].click();
  expect(storedNavSide()).toBe("right");
  expect(l.el.dataset.navSide).toBe("right");
  expect(radios.map((r) => r.checked)).toEqual([false, true]);
  radios[0].click();
  expect(storedNavSide()).toBe("left");
  row.remove();
  l.el.remove();
  l.destroy();
});

const foot = (l: ReturnType<typeof createLayout>) => ({
  settings: l.el.querySelector<HTMLButtonElement>(".zen-sidebar-foot .zen-settings-button")!,
  exp: l.el.querySelector<HTMLButtonElement>(".zen-export-button")!,
  inspector: inspectorBtn(l),
});

test("active footer buttons", () => {
  const l = createLayout();
  const { settings, exp, inspector } = foot(l);
  // none is active at first
  expect(settings.getAttribute("aria-expanded")).toBe("false");
  expect(exp.getAttribute("aria-pressed")).toBe("false");
  expect(inspector.getAttribute("aria-expanded")).toBe("false");
  // Export uses aria-pressed, the others aria-expanded; one is on at a time
  l.setActive("export", true);
  expect(exp.getAttribute("aria-pressed")).toBe("true");
  expect(exp.hasAttribute("aria-expanded")).toBe(false);
  l.setActive("export", false);
  expect(exp.getAttribute("aria-pressed")).toBe("false");
  // setSettingsOpen is the same as setActive("settings")
  l.setSettingsOpen(true);
  expect(settings.getAttribute("aria-expanded")).toBe("true");
  l.setSettingsOpen(false);
  expect(settings.getAttribute("aria-expanded")).toBe("false");
  // the Inspector's own event still sets its state
  state(true, false, 0);
  expect(inspector.getAttribute("aria-expanded")).toBe("false");
  state(true, true, 0);
  expect(inspector.getAttribute("aria-expanded")).toBe("true");
});

test("active footer buttons: inverted (the icon in --bg on --fg), in tokens, with no side bar", () => {
  const css = readFileSync(resolve(__dirname, "../styles/shell.css"), "utf8").replace(/\/\*[\s\S]*?\*\//g, "");
  const rules = Array.from(css.matchAll(/([^{}]+)\{([^{}]*)\}/g)).filter((m) => /\.zen-settings-button[^,{]*(aria-pressed|aria-expanded)="true"/.test(m[1]));
  expect(rules.length).toBeGreaterThan(0);
  const all = rules.map((m) => m[1] + "{" + m[2]).join("\n");
  expect(all).toContain('[aria-pressed="true"]');
  expect(all).toContain('[aria-expanded="true"]');
  expect(all).toMatch(/background:\s*var\(--fg\)/);
  expect(all).toMatch(/color:\s*var\(--bg\)/);
  expect(all).not.toMatch(/box-shadow/); // no bar against the rounded corners
  expect(all).not.toMatch(/#[0-9a-f]{3,8}\b|rgba?\(/i);
  // hover is lighter than the active state: it only changes the colour
  expect(css).not.toMatch(/\.zen-settings-button:hover[^{]*\{[^}]*background/);
});

// The footer is one group: at most one button is on, so two views never overlap (Settings and the Inspector),
// and a future footer button joins the same rule through addViewButton.
test("footer buttons: one active at a time; opening one closes the view of the other", () => {
  const closed: string[] = [];
  const l = createLayout({ onCloseView: (v) => closed.push(v) });
  document.body.append(l.el);
  const btn = (sel: string) => l.el.querySelector<HTMLElement>(sel)!;
  const on = () => Array.from(l.el.querySelectorAll<HTMLElement>(".zen-sidebar-foot button, .zen-top-actions button"))
    .filter((b) => b.getAttribute("aria-expanded") === "true" || b.getAttribute("aria-pressed") === "true")
    .map((b) => b.getAttribute("aria-label") ?? b.textContent);
  let inspectorClosed = 0;
  document.addEventListener(INSPECTOR_CLOSE, () => inspectorClosed++);
  // the Inspector opens while Settings shows: Settings closes
  l.setActive("settings", true);
  document.dispatchEvent(new CustomEvent(INSPECTOR_STATE, { detail: { available: true, open: true, count: 0 } }));
  expect(closed).toEqual(["settings"]);
  expect(btn(".zen-settings-button:not(.zen-inspector-button):not(.zen-export-button)").getAttribute("aria-expanded")).toBe("false");
  expect(on()).toHaveLength(1);
  // Settings opens while the Inspector is open: the Inspector is asked to close
  l.setActive("settings", true);
  expect(inspectorClosed).toBe(1);
  // Export and Settings share one view: switching between them moves the mark and closes nothing
  closed.length = 0;
  l.setActive("export", true);
  expect(closed).toEqual([]);
  expect(btn(".zen-export-button").getAttribute("aria-pressed")).toBe("true");
  expect(btn(".zen-settings-button:not(.zen-inspector-button):not(.zen-export-button)").getAttribute("aria-expanded")).toBe("false");
  l.setActive("settings", true);
  expect(btn(".zen-export-button").getAttribute("aria-pressed")).toBe("false");
  expect(on()).toHaveLength(1);
  // a future footer button follows the same rule
  let mineClosed = 0;
  const mine = l.addViewButton({ id: "notes", label: "Notes", title: "Notes", view: "notes", onClick: () => l.setActive("notes", true), close: () => { mineClosed++; } });
  expect(l.el.querySelector(".zen-sidebar-foot")!.contains(mine)).toBe(true); // the footer is the default place
  mine.click();
  expect(closed).toEqual(["settings"]);
  expect(mine.getAttribute("aria-expanded")).toBe("true");
  expect(on()).toHaveLength(1);
  l.setActive("settings", true);
  expect(mineClosed).toBe(1);
  expect(mine.getAttribute("aria-expanded")).toBe("false");
  l.el.remove();
});

// ui/top-bar-actions: a view button's place (footer or top bar) is apart from its group.
test("a view button placed in the top bar is in the same one-at-a-time group as the footer's", () => {
  const closed: string[] = [];
  const l = createLayout({ onCloseView: (v) => closed.push(v) });
  document.body.append(l.el);
  let topClosed = 0;
  const top = l.addViewButton({ id: "notes", label: "Notes", title: "Notes", view: "notes", place: "top", onClick: () => l.setActive("notes", true), close: () => { topClosed++; } });
  const group = l.el.querySelector(".zen-top-actions")!;
  expect(group.contains(top)).toBe(true);
  expect(l.el.querySelector(".zen-sidebar-foot")!.contains(top)).toBe(false);
  expect(top.classList.contains("zen-settings-button")).toBe(true); // the footer's icon button, so its on style
  top.click();
  expect(top.getAttribute("aria-expanded")).toBe("true");
  l.setActive("settings", true); // opening Settings closes the one in the top bar
  expect(topClosed).toBe(1);
  expect(top.getAttribute("aria-expanded")).toBe("false");
  l.setActive("notes", true); // and the other way round
  expect(closed).toEqual(["settings"]);
  l.el.remove();
});

test("addTopButton puts a plain icon button in the group, in order, with no part in the view group", () => {
  const l = createLayout();
  let n = 0;
  const b = l.addTopButton({ id: "x", label: "X", title: "X (⌘X)", icon: icon(document, "pencil"), onClick: () => n++ });
  expect(l.el.querySelector(".zen-top-actions")!.contains(b)).toBe(true);
  expect(b.getAttribute("aria-label")).toBe("X");
  expect(b.title).toBe("X (⌘X)");
  expect(b.textContent).toBe("");
  expect(b.querySelector("svg")).not.toBeNull();
  expect(b.classList.contains("zen-settings-button")).toBe(true);
  b.click();
  expect(n).toBe(1);
  l.setActive("settings", true);
  expect(b.hasAttribute("aria-expanded")).toBe(false);
});

test("the top bar's buttons use the footer's icon set and the same size", () => {
  const css = readFileSync(resolve(__dirname, "../styles/shell.css"), "utf8").replace(/\/\*[\s\S]*?\*\//g, "");
  expect(css).toMatch(/\.zen-top-actions\s*\{[^}]*display:\s*flex/);
  const l = createLayout();
  expect(inspectorBtn(l).querySelector("svg")!.getAttribute("stroke-width")).toBeNull(); // the paths carry the stroke
  expect(inspectorBtn(l).querySelector("path, rect")!.getAttribute("stroke-width")).toBe("1.5");
  l.destroy();
});
