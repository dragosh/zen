// The Developer panel's content (ui/developer-tab): source tabs, level and text filters, Clear,
// Copy all and the list of entries. Memory is bounded (MAX_ENTRIES). All text through textContent.
import type { KitHost, LogEntry, LogLevel } from "../host";
import { copyText } from "./clipboard";
import type { ConsoleCapture } from "./console";
import { button, el } from "./dom";

export const MAX_ENTRIES = 2000;
const LEVELS: LogLevel[] = ["debug", "info", "warn", "error"];
const RANK: Record<LogLevel, number> = { debug: 0, info: 1, warn: 2, error: 3 };
const FOLLOW_SLACK = 24; // px from the bottom that still counts as "at the bottom"

type Tab = "all" | "app" | "plugin" | "console";
const TABS: { id: Tab; label: string }[] = [
  { id: "all", label: "All" }, { id: "app", label: "App" }, { id: "plugin", label: "Plugins" }, { id: "console", label: "Console" },
];

export type DevPanel = {
  el: HTMLElement;
  /** Loads what the Go side has kept and starts following it and the console. */
  start(): Promise<void>;
  /** Entries kept. */
  count(): number;
  focus(): void;
  destroy(): void;
};

export type DevPanelOptions = {
  doc: Document;
  host: Pick<KitHost, "logs" | "onLog">;
  capture: ConsoleCapture;
  onClose(): void;
};

const pad = (n: number, w = 2) => String(n).padStart(w, "0");
function clock(t: number): string {
  const d = new Date(t);
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}.${pad(d.getMilliseconds(), 3)}`;
}

/** One line for Copy all: ISO time, LEVEL, [source], message. */
export function formatEntry(e: LogEntry): string {
  return `${new Date(e.time).toISOString()} ${e.level.toUpperCase()} [${e.source}] ${e.message}`;
}

export function createDevPanel({ doc, host, capture, onClose }: DevPanelOptions): DevPanel {
  const root = el("section", "zen-dev-panel");
  root.setAttribute("aria-label", "Developer");
  root.tabIndex = -1;

  const hasGo = typeof host.logs === "function";
  const tabs = hasGo ? TABS : TABS.filter((t) => t.id === "console");
  let tab: Tab = tabs[0].id;
  let minLevel: LogLevel = "debug";
  let query = "";

  // ---- header

  const head = el("div", "zen-dev-head");
  const tablist = el("div", "zen-dev-tabs");
  tablist.setAttribute("role", "tablist");
  tablist.setAttribute("aria-label", "Log source");
  const tabButtons = tabs.map(({ id, label }) => {
    const b = button(label, () => choose(id), { className: "zen-dev-tab" });
    b.setAttribute("role", "tab");
    b.dataset.tab = id;
    return b;
  });
  tablist.append(...tabButtons);
  tablist.addEventListener("keydown", (e) => {
    const step = ({ ArrowRight: 1, ArrowLeft: -1 } as Record<string, number>)[e.key];
    if (step === undefined) return;
    e.preventDefault();
    const i = tabs.findIndex((t) => t.id === tab);
    const next = tabs[(i + step + tabs.length) % tabs.length].id;
    choose(next);
    tabButtons.find((b) => b.dataset.tab === next)!.focus();
  });

  const level = el("select", "zen-dev-level-filter");
  level.setAttribute("aria-label", "Minimum level");
  for (const l of LEVELS) {
    const o = el("option", "", l === "debug" ? "All levels" : `${l[0].toUpperCase()}${l.slice(1)}+`);
    o.value = l;
    level.append(o);
  }
  level.addEventListener("change", () => { minLevel = level.value as LogLevel; rebuild(); });

  const filter = el("input", "zen-dev-text-filter");
  filter.type = "search";
  filter.placeholder = "Filter";
  filter.setAttribute("aria-label", "Filter messages");
  filter.addEventListener("input", () => { query = filter.value.toLowerCase(); rebuild(); });

  const status = el("span", "zen-dev-status");
  status.setAttribute("role", "status");
  const clear = button("Clear", () => { entries = []; rebuild(); }, { className: "zen-dev-clear" });
  const copy = button("Copy all", () => {
    void copyText(doc, visible().map(formatEntry).join("\n")).then((ok) => {
      status.textContent = ok ? "Copied" : "Could not copy";
      doc.defaultView?.setTimeout(() => { status.textContent = ""; }, 1500);
    });
  }, { className: "zen-dev-copy" });
  const close = button("Close", () => onClose(), { className: "ghost zen-dev-close", text: "✕", title: "Close (Esc)" });
  head.append(tablist, level, filter, clear, copy, status, close);

  // ---- list

  const list = el("div", "zen-dev-list");
  list.setAttribute("role", "log");
  list.setAttribute("aria-live", "off"); // a flood must not be announced
  list.tabIndex = 0;
  const jump = button("Jump to latest", () => { follow = true; jump.hidden = true; toBottom(); }, { className: "zen-dev-jump" });
  jump.hidden = true;
  const empty = el("p", "zen-dev-empty", "No log entries yet.");

  root.append(head, list, jump);
  root.addEventListener("keydown", (e) => {
    if (e.key !== "Escape") return;
    e.preventDefault();
    e.stopPropagation();
    onClose();
  });

  // ---- data

  let entries: LogEntry[] = [];
  let lastGo = 0; // the highest Go-side seq shown; the Go side has one counter for app and plugin

  const matches = (e: LogEntry): boolean =>
    (tab === "all" || e.source === tab) && RANK[e.level] >= RANK[minLevel] && (query === "" || e.message.toLowerCase().includes(query));
  const visible = (): LogEntry[] => entries.filter(matches);

  function row(e: LogEntry): HTMLElement {
    const r = el("div", "zen-dev-row");
    r.dataset.level = e.level;
    r.dataset.source = e.source;
    r.append(el("span", "zen-dev-time", clock(e.time)), el("span", "zen-dev-level", e.level), el("span", "zen-dev-source", e.source), el("span", "zen-dev-msg", e.message));
    return r;
  }

  let follow = true;
  function toBottom(): void {
    list.scrollTop = list.scrollHeight;
  }
  list.addEventListener("scroll", () => {
    follow = list.scrollHeight - list.scrollTop - list.clientHeight <= FOLLOW_SLACK;
    jump.hidden = follow;
  });

  function showEmpty(): void {
    if (list.querySelector(".zen-dev-row")) empty.remove();
    else if (!empty.isConnected) list.append(empty);
  }

  function rebuild(): void {
    list.replaceChildren(...visible().map(row));
    showEmpty();
    if (follow) toBottom();
  }

  function add(batch: LogEntry[]): void {
    const fresh: LogEntry[] = [];
    for (const e of batch) {
      if (e.source !== "console") {
        if (e.seq <= lastGo) continue; // already shown (the initial load and an event can overlap)
        lastGo = e.seq;
      }
      fresh.push(e);
    }
    if (fresh.length === 0) return;
    entries.push(...fresh);
    const over = entries.length - MAX_ENTRIES;
    if (over > 0) entries.splice(0, over);
    if (over > 0) return rebuild(); // the oldest rows go
    const rows = fresh.filter(matches).map(row);
    if (rows.length) {
      empty.remove();
      list.append(...rows);
    }
    if (follow) toBottom();
  }

  function choose(id: Tab): void {
    tab = id;
    for (const b of tabButtons) {
      const on = b.dataset.tab === id;
      b.setAttribute("aria-selected", String(on));
      b.tabIndex = on ? 0 : -1;
    }
    rebuild();
  }

  let offs: (() => void)[] = [];
  let started = false;

  choose(tab);
  return {
    el: root,
    async start() {
      if (started) return;
      started = true;
      // Subscribe first so nothing between the load and the subscription is lost; events wait for the load.
      let queue: LogEntry[][] | null = [];
      if (host.onLog) offs.push(host.onLog((b) => { if (queue) queue.push(b); else add(b); }));
      if (host.logs) {
        try {
          add(await host.logs(0));
        } catch { /* the Go side's log is unavailable: the console still works */ }
      }
      for (const b of queue) add(b);
      queue = null;
      const before = capture.entries();
      add(before);
      offs.push(capture.subscribe((e) => add([e])));
      entries.sort((a, b) => a.time - b.time || a.seq - b.seq);
      rebuild();
    },
    count: () => entries.length,
    focus: () => root.focus(),
    destroy() {
      for (const off of offs) off();
      offs = [];
      root.remove();
    },
  };
}
