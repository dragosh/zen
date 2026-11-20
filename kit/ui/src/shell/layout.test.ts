// @vitest-environment happy-dom
import { readFileSync } from "node:fs";
import { beforeEach, expect, test } from "vitest";
import { INSPECTOR_STATE, INSPECTOR_TOGGLE } from "../inspector/events";
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

test("the Inspector button is an icon button right after the cog in the footer, hidden until a page offers one", () => {
  const l = createLayout();
  const foot = l.el.querySelector(".zen-sidebar-foot")!;
  const kids = Array.from(foot.children);
  expect(kids[0].getAttribute("aria-label")).toBe("Settings");
  expect(kids[2]).toBe(inspectorBtn(l)); // Settings, Export, Inspector
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

test("the footer has the cog and the Inspector button, and no Developer button: the Console is a tab of the Inspector (#25)", () => {
  const l = createLayout();
  expect(l.el.querySelector(".zen-developer-button")).toBeNull();
  expect(l.el.querySelector(".zen-sidebar-foot")!.children).toHaveLength(3);
  expect(l.el.querySelector(".zen-sidebar-foot")!.lastElementChild!.classList.contains("zen-inspector-button")).toBe(true);
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

test("footer buttons per feature: Settings, Export, Inspector in that order, icons with tooltips and labels; Export asks the shell", () => {
  let exported = 0;
  const l = createLayout({ onExport: () => { exported++; } });
  const kids = Array.from(l.el.querySelector(".zen-sidebar-foot")!.children) as HTMLButtonElement[];
  expect(kids.map((b) => b.getAttribute("aria-label"))).toEqual(["Settings", "Export", "Inspector"]);
  expect(kids[0].title).toBe("Settings ⌘,");
  expect(kids[1].title).toBe("Export ⇧⌘B");
  expect(kids[2].title).toContain("⌥⌘I");
  for (const b of kids) expect(b.querySelector("svg")).not.toBeNull();
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
  l.setActive("settings", true);
  l.setActive("export", true);
  l.setActive("inspector", true);
  expect(settings.getAttribute("aria-expanded")).toBe("true");
  expect(exp.getAttribute("aria-pressed")).toBe("true");
  expect(exp.hasAttribute("aria-expanded")).toBe(false);
  expect(inspector.getAttribute("aria-expanded")).toBe("true");
  l.setActive("export", false);
  expect(exp.getAttribute("aria-pressed")).toBe("false");
  expect(settings.getAttribute("aria-expanded")).toBe("true"); // the others stay
  // setSettingsOpen is the same as setActive("settings")
  l.setSettingsOpen(false);
  expect(settings.getAttribute("aria-expanded")).toBe("false");
  // the Inspector's own event still sets its state
  state(true, false, 0);
  expect(inspector.getAttribute("aria-expanded")).toBe("false");
  state(true, true, 0);
  expect(inspector.getAttribute("aria-expanded")).toBe("true");
});

test("active footer buttons: the CSS rule, in tokens, mirrored with the sidebar", () => {
  const css = readFileSync(new URL("../styles/shell.css", import.meta.url), "utf8").replace(/\/\*[\s\S]*?\*\//g, "");
  const rules = Array.from(css.matchAll(/([^{}]+)\{([^{}]*)\}/g)).filter((m) => /\.zen-settings-button[^,{]*(aria-pressed|aria-expanded)="true"/.test(m[1]));
  expect(rules.length).toBeGreaterThan(0);
  const all = rules.map((m) => m[1] + "{" + m[2]).join("\n");
  expect(all).toContain('[aria-pressed="true"]');
  expect(all).toContain('[aria-expanded="true"]');
  expect(all).toMatch(/background:\s*var\(--btn-bg\)/);
  expect(all).toMatch(/color:\s*var\(--fg\)/);
  expect(all).toMatch(/box-shadow:\s*inset\s+-?2px\s+0\s+0\s+var\(--fg\)/);
  expect(all).not.toMatch(/#[0-9a-f]{3,8}\b|rgba?\(/i);
  // the bar is on the content side: it flips when the sidebar is on the right
  expect(css).toMatch(/data-nav-side="right"[^{]*\.zen-settings-button/);
  // hover is lighter than the active state: it only changes the colour
  expect(css).not.toMatch(/\.zen-settings-button:hover[^{]*\{[^}]*background/);
});
