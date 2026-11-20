// @vitest-environment happy-dom
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { DEV_KEY, developerEnabled } from "./developer";
import { appearanceSection, createSettingsFrame, developerSection, type SettingsItem, type SettingsSection, type SettingsView } from "./settings";
import { storedTheme, THEME_KEY } from "./theme";

beforeEach(() => {
  localStorage.clear();
  document.body.replaceChildren();
  delete document.documentElement.dataset.theme;
});
afterEach(() => vi.unstubAllGlobals());

const settle = async () => { for (let i = 0; i < 6; i++) await new Promise((r) => setTimeout(r, 0)); };
const key = (el: Element, k: string) => el.dispatchEvent(new KeyboardEvent("keydown", { key: k, bubbles: true, cancelable: true }));
const rows = (v: SettingsView, id: string) => Array.from(v.el.querySelectorAll<HTMLElement>(`.zen-settings-panel[data-section="${id}"] [role=option]`));
const detail = (v: SettingsView, id: string) => v.el.querySelector<HTMLElement>(`.zen-settings-panel[data-section="${id}"] .zen-settings-detail`)!;
const tab = (v: SettingsView, id: string) => v.el.querySelector<HTMLButtonElement>(`.zen-settings-section[data-section="${id}"]`)!;

function listSection(id: string, labels: string[], extra: Partial<SettingsSection> = {}): SettingsSection {
  return {
    id, label: id,
    items: () => labels.map((l, i): SettingsItem => ({ id: l.toLowerCase(), label: l, note: i === 1 ? "Off" : undefined, disabled: l === "Gamma" })),
    render: (item, right) => { right.append(Object.assign(document.createElement("p"), { className: "body", textContent: `about ${item.id}` })); },
    ...extra,
  };
}
function frame(...sections: SettingsSection[]): SettingsView {
  const v = createSettingsFrame(sections);
  document.body.append(v.el);
  v.show();
  return v;
}

// ---- 2: the frame

test("two-column frame", async () => {
  const v = frame(listSection("things", ["Alpha", "Beta", "Gamma", "Delta"]), { id: "old", label: "Old", el: Object.assign(document.createElement("p"), { textContent: "legacy" }) });
  await settle();
  // the section list and its panel: a listbox on the left, the detail on the right
  const panel = v.el.querySelector<HTMLElement>('.zen-settings-panel[data-section="things"]')!;
  const list = panel.querySelector<HTMLElement>("[role=listbox]")!;
  expect(list.getAttribute("aria-label")).toBe("things");
  expect(panel.querySelector(".zen-settings-cols")!.firstElementChild).toBe(list);
  expect(list.nextElementSibling).toBe(detail(v, "things"));
  expect(rows(v, "things").map((r) => r.querySelector(".zen-settings-item-label")!.textContent)).toEqual(["Alpha", "Beta", "Gamma", "Delta"]);
  expect(rows(v, "things")[1].querySelector(".zen-settings-item-note")!.textContent).toBe("Off");
  expect(rows(v, "things")[2].getAttribute("aria-disabled")).toBe("true");
  // the first item is selected; the right column has its title as h3 and its content
  expect(rows(v, "things")[0].getAttribute("aria-selected")).toBe("true");
  expect(rows(v, "things")[1].getAttribute("aria-selected")).toBe("false");
  expect(detail(v, "things").querySelector("h3")!.textContent).toBe("Alpha");
  expect(detail(v, "things").textContent).toContain("about alpha");
  // click selects; a disabled row does not
  rows(v, "things")[3].click();
  expect(detail(v, "things").querySelector("h3")!.textContent).toBe("Delta");
  expect(detail(v, "things").textContent).not.toContain("about alpha");
  rows(v, "things")[2].click();
  expect(detail(v, "things").querySelector("h3")!.textContent).toBe("Delta");
  // arrows move through the enabled rows, selecting and focusing; Home/End; Enter selects
  rows(v, "things")[0].focus();
  key(list, "ArrowDown");
  expect(document.activeElement).toBe(rows(v, "things")[1]);
  expect(rows(v, "things")[1].getAttribute("aria-selected")).toBe("true");
  key(list, "ArrowDown"); // skips the disabled Gamma
  expect(document.activeElement).toBe(rows(v, "things")[3]);
  key(list, "ArrowDown"); // stops at the end
  expect(document.activeElement).toBe(rows(v, "things")[3]);
  key(list, "Home");
  expect(document.activeElement).toBe(rows(v, "things")[0]);
  key(list, "End");
  expect(document.activeElement).toBe(rows(v, "things")[3]);
  key(list, "ArrowUp");
  key(list, "Enter");
  expect(detail(v, "things").querySelector("h3")!.textContent).toBe("Beta");
  // roving tab stop
  expect(rows(v, "things").map((r) => r.tabIndex)).toEqual([-1, 0, -1, -1]);
  // old sections give their element, shown in the right column under the section title
  tab(v, "old").click();
  const old = v.el.querySelector<HTMLElement>('.zen-settings-panel[data-section="old"]')!;
  expect(old.querySelector("h2")!.textContent).toBe("Old");
  expect(old.textContent).toContain("legacy");
  expect(old.querySelector("[role=listbox]")).toBeNull();
});

test("two-column frame: the selected item is remembered per section for the session", async () => {
  const v = frame(listSection("a", ["Alpha", "Beta"]), listSection("b", ["One", "Two"]));
  await settle();
  rows(v, "a")[1].click();
  tab(v, "b").click();
  await settle();
  rows(v, "b")[1].click();
  tab(v, "a").click();
  await settle();
  v.show(); // the page is shown again: lists re-read, selection kept
  await settle();
  expect(rows(v, "a").map((r) => r.getAttribute("aria-selected"))).toEqual(["false", "true"]);
  expect(rows(v, "b").map((r) => r.getAttribute("aria-selected"))).toEqual(["false", "true"]);
  expect(detail(v, "a").querySelector("h3")!.textContent).toBe("Beta");
});

test("two-column frame: stacks below 720 px, an empty list shows its empty text, a section can say which item is current", async () => {
  let fire: (w: number) => void = () => {};
  vi.stubGlobal("ResizeObserver", class { constructor(cb: (e: unknown[]) => void) { fire = (w) => cb([{ contentRect: { width: w } }]); } observe() {} disconnect() {} });
  const picked: string[] = [];
  const v = frame(
    listSection("empty", [], { empty: "Nothing here." }),
    listSection("cur", ["Alpha", "Beta"], { current: () => "beta", onPick: (i) => picked.push(i.id) }),
  );
  await settle();
  expect(v.el.classList.contains("zen-settings-narrow")).toBe(false);
  fire(700);
  expect(v.el.classList.contains("zen-settings-narrow")).toBe(true);
  fire(721);
  expect(v.el.classList.contains("zen-settings-narrow")).toBe(false);
  expect(detail(v, "empty").textContent).toBe("Nothing here.");
  expect(rows(v, "empty")).toHaveLength(0);
  tab(v, "cur").click();
  await settle();
  expect(rows(v, "cur").map((r) => r.getAttribute("aria-selected"))).toEqual(["false", "true"]); // current() wins over "first"
  expect(picked).toEqual([]); // not picked by the frame itself
  rows(v, "cur")[0].click();
  expect(picked).toEqual(["alpha"]);
});

test("two-column frame: open() selects a section and an item; a section's content can ask the frame to read again; text is never HTML", async () => {
  let names = ["<b>x</b>", "y"];
  const v = frame(listSection("a", ["Alpha"]), {
    id: "b", label: "b",
    items: () => names.map((n) => ({ id: n, label: n })),
    render: (item, right) => {
      const b = document.createElement("button");
      b.textContent = "go";
      b.addEventListener("click", () => { names = ["y", "z"]; b.dispatchEvent(new CustomEvent("zen-settings-reload", { bubbles: true, detail: {} })); });
      right.append(b);
      void item;
    },
  });
  await settle();
  v.open("b", "y");
  await settle();
  expect(tab(v, "b").getAttribute("aria-current")).toBe("true");
  expect(rows(v, "b")[1].getAttribute("aria-selected")).toBe("true");
  expect(rows(v, "b")[0].querySelector("b")).toBeNull();
  detail(v, "b").querySelector("button")!.click();
  await settle();
  expect(rows(v, "b").map((r) => r.dataset.item)).toEqual(["y", "z"]);
  expect(rows(v, "b")[0].getAttribute("aria-selected")).toBe("true"); // the selection stays
});

// ---- 3: appearance

test("appearance preview", async () => {
  const extra = Object.assign(document.createElement("div"), { className: "extra", textContent: "Navigation" });
  const v = frame(appearanceSection(() => document, [extra]));
  await settle();
  expect(rows(v, "appearance").map((r) => r.querySelector(".zen-settings-item-label")!.textContent)).toEqual(["System", "Light", "Dark"]);
  expect(rows(v, "appearance")[0].getAttribute("aria-selected")).toBe("true");
  const preview = () => detail(v, "appearance").querySelector<HTMLElement>(".zen-theme-preview")!;
  expect(preview().dataset.theme).toBe("system");
  expect(preview().querySelector("h4, .zen-theme-preview-title")).not.toBeNull();
  expect(preview().querySelector("a")).not.toBeNull();
  expect(preview().querySelector("code")).not.toBeNull();
  expect(preview().querySelector("p")).not.toBeNull();
  expect(detail(v, "appearance").querySelector(".extra")).not.toBeNull(); // the slot for Navigation (#24)
  // choosing a theme applies it, remembers it, and the preview follows
  rows(v, "appearance")[2].click();
  expect(document.documentElement.dataset.theme).toBe("dark");
  expect(localStorage.getItem(THEME_KEY)).toBe("dark");
  expect(preview().dataset.theme).toBe("dark");
  expect(detail(v, "appearance").querySelector("h3")!.textContent).toBe("Dark");
  rows(v, "appearance")[1].focus();
  key(rows(v, "appearance")[1].parentElement!, "ArrowUp");
  expect(storedTheme()).toBe("system");
  expect(document.documentElement.dataset.theme).toBeUndefined();
  expect(preview().dataset.theme).toBe("system");
  // showing the page again reads the stored theme (another window changed it)
  localStorage.setItem(THEME_KEY, "light");
  v.show();
  await settle();
  expect(rows(v, "appearance")[1].getAttribute("aria-selected")).toBe("true");
  expect(preview().dataset.theme).toBe("light");
});

// ---- 8: developer

test("developer section", async () => {
  const openConsole = vi.fn();
  const v = frame(developerSection({ openConsole }));
  await settle();
  expect(rows(v, "developer").map((r) => r.textContent)).toEqual(["Console"]);
  const d = detail(v, "developer");
  const box = d.querySelector<HTMLInputElement>("input[type=checkbox]")!;
  expect(d.textContent).toContain("Show Console in the Inspector");
  expect(d.textContent).not.toContain("Show developer tab");
  expect(d.querySelector("p")!.textContent!.length).toBeGreaterThan(10); // one line on what the Console shows
  const open = Array.from(d.querySelectorAll("button")).find((b) => b.textContent!.includes("Open Console"))!;
  expect(open.title).toContain("⌥⌘J");
  // off: nothing to open
  expect(box.checked).toBe(false);
  expect(open.disabled).toBe(true);
  box.checked = true;
  box.dispatchEvent(new Event("change"));
  expect(developerEnabled()).toBe(true);
  expect(localStorage.getItem(DEV_KEY)).not.toBeNull(); // the same storage key
  expect(open.disabled).toBe(false);
  open.click();
  expect(openConsole).toHaveBeenCalledTimes(1);
  box.checked = false;
  box.dispatchEvent(new Event("change"));
  expect(developerEnabled()).toBe(false);
  expect(open.disabled).toBe(true);
  // the page reads the setting again when shown
  localStorage.setItem(DEV_KEY, "1");
  v.show();
  await settle();
  expect(detail(v, "developer").querySelector<HTMLInputElement>("input[type=checkbox]")!.checked).toBe(true);
});
