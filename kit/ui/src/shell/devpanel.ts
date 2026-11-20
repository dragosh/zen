// The Inspector's Console tab (ui/inspector, ui/developer-tab): the log on the left, the selected entry in
// full on the right (panel columns); the source dropdown, the level and text filters, Clear and Copy
// go into the header slot the Inspector gives it. Memory is bounded (MAX_ENTRIES). All text through textContent.
import type { KitHost, LogEntry, LogLevel } from "../host";
import { panelColumns, panelEmpty } from "../panel";
import { copyText } from "./clipboard";
import type { ConsoleCapture } from "./console";
import { button, el } from "./dom";

export const MAX_ENTRIES = 2000;
const LEVELS: LogLevel[] = ["debug", "info", "warn", "error"];
const RANK: Record<LogLevel, number> = { debug: 0, info: 1, warn: 2, error: 3 };
/** Where the chosen minimum level is remembered, per machine. */
export const LEVEL_KEY = "zen.console.level";
/** Debug is detail for tracing: the Console starts at Info and up. */
const DEFAULT_LEVEL: LogLevel = "info";
const FOLLOW_SLACK = 24; // px from the bottom that still counts as "at the bottom"

type Source = "all" | "app" | "plugin" | "console";
/** "console" is the page's own JS console, so it is called Page here. */
const SOURCES: { id: Source; label: string }[] = [
  { id: "all", label: "All" }, { id: "app", label: "App" }, { id: "plugin", label: "Plugins" }, { id: "console", label: "Page" },
];

export type ConsoleTab = {
  /** The two columns; put it in the tab's body. */
  el: HTMLElement;
  /** Loads what the Go side has kept and starts following it and the console. */
  start(): Promise<void>;
  /** Entries kept. */
  count(): number;
  focus(): void;
  destroy(): void;
};

export type ConsoleTabOptions = {
  doc: Document;
  host: Pick<KitHost, "logs" | "onLog">;
  capture: ConsoleCapture;
  /** Where the filters and actions go (the Inspector's header, for this tab). */
  tools: HTMLElement;
};

const pad = (n: number, w = 2) => String(n).padStart(w, "0");
function clock(t: number): string {
  const d = new Date(t);
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}.${pad(d.getMilliseconds(), 3)}`;
}

/** One line for Copy: ISO time, LEVEL, [source], message. */
export function formatEntry(e: LogEntry): string {
  return `${new Date(e.time).toISOString()} ${e.level.toUpperCase()} [${e.source}] ${e.message}`;
}

export function createConsoleTab({ doc, host, capture, tools }: ConsoleTabOptions): ConsoleTab {
  const hasGo = typeof host.logs === "function";
  const sources = hasGo ? SOURCES : SOURCES.filter((s) => s.id === "console");
  let source: Source = sources[0].id;
  const store = (() => { try { return doc.defaultView?.localStorage ?? null; } catch { return null; } })();
  const savedLevel = (): LogLevel => {
    try {
      const v = store?.getItem(LEVEL_KEY);
      return v && (LEVELS as string[]).includes(v) ? (v as LogLevel) : DEFAULT_LEVEL;
    } catch { return DEFAULT_LEVEL; }
  };
  let minLevel: LogLevel = savedLevel();
  let query = "";
  let selected: LogEntry | null = null;

  // ---- tools: the source dropdown first, then the level and text filters, then Clear and Copy

  const sourceFilter = el("select", "zen-console-source");
  sourceFilter.setAttribute("aria-label", "Source");
  for (const s of sources) {
    const o = el("option", "", s.label);
    o.value = s.id;
    sourceFilter.append(o);
  }
  sourceFilter.addEventListener("change", () => { source = sourceFilter.value as Source; rebuild(); });

  const level = el("select", "zen-dev-level-filter");
  level.setAttribute("aria-label", "Minimum level");
  for (const l of LEVELS) {
    const o = el("option", "", l === "debug" ? "All levels" : `${l[0].toUpperCase()}${l.slice(1)}+`);
    o.value = l;
    level.append(o);
  }
  level.value = minLevel;
  level.addEventListener("change", () => {
    minLevel = level.value as LogLevel;
    try { store?.setItem(LEVEL_KEY, minLevel); } catch { /* not remembered */ }
    rebuild();
  });

  const filter = el("input", "zen-dev-text-filter");
  filter.type = "search";
  filter.placeholder = "Filter";
  filter.setAttribute("aria-label", "Filter messages");
  filter.addEventListener("input", () => { query = filter.value.toLowerCase(); rebuild(); });

  const status = el("span", "zen-dev-status");
  status.setAttribute("role", "status");
  const clear = button("Clear", () => { entries = []; select(null); rebuild(); }, { className: "zen-dev-clear" });
  const copy = button("Copy", () => {
    void copyText(doc, visible().map(formatEntry).join("\n")).then((ok) => {
      status.textContent = ok ? "Copied" : "Could not copy";
      doc.defaultView?.setTimeout(() => { status.textContent = ""; }, 1500);
    });
  }, { className: "zen-dev-copy", title: "Copy the entries shown" });
  tools.append(sourceFilter, level, filter, clear, copy, status);

  // ---- columns: the list, then the details

  const cols = panelColumns();
  cols.el.classList.add("zen-console");
  const list = el("div", "zen-dev-list");
  list.setAttribute("role", "listbox");
  list.setAttribute("aria-label", "Log");
  list.setAttribute("aria-live", "off"); // a flood must not be announced
  list.tabIndex = 0;
  const jump = button("Jump to latest", () => { follow = true; jump.hidden = true; toBottom(); }, { className: "zen-dev-jump" });
  jump.hidden = true;
  // The panel's own empty state, padded like every tab (docs/UI.md, UI consistency).
  const empty = panelEmpty("No log entries yet.");
  empty.classList.add("zen-dev-empty");
  cols.left.classList.add("zen-console-list");
  cols.left.append(list, jump);
  const details = el("div", "zen-console-details");
  cols.right.append(details);

  // ---- data

  let entries: LogEntry[] = [];
  let lastGo = 0; // the highest Go-side seq shown; the Go side has one counter for app and plugin

  const matches = (e: LogEntry): boolean =>
    (source === "all" || e.source === source) && RANK[e.level] >= RANK[minLevel] && (query === "" || e.message.toLowerCase().includes(query));
  const visible = (): LogEntry[] => entries.filter(matches);

  const rowOf = new Map<LogEntry, HTMLElement>();
  function row(e: LogEntry): HTMLElement {
    const r = el("div", "zen-dev-row");
    r.setAttribute("role", "option");
    r.setAttribute("aria-selected", String(e === selected));
    r.tabIndex = -1;
    r.dataset.level = e.level;
    r.dataset.source = e.source;
    r.append(el("span", "zen-dev-time", clock(e.time)), el("span", "zen-dev-level", e.level), el("span", "zen-dev-source", e.source), el("span", "zen-dev-msg", e.message));
    r.addEventListener("click", () => select(e));
    r.addEventListener("keydown", (k) => {
      if (k.key === "Enter" || k.key === " ") { k.preventDefault(); select(e); return; }
      const step = k.key === "ArrowDown" ? 1 : k.key === "ArrowUp" ? -1 : 0;
      if (!step) return;
      k.preventDefault();
      const rows = Array.from(list.querySelectorAll<HTMLElement>(".zen-dev-row"));
      const next = rows[rows.indexOf(r) + step];
      if (next) { next.focus(); next.click(); }
    });
    rowOf.set(e, r);
    return r;
  }

  function field(label: string, name: string, value: string, mono = false): HTMLElement {
    const pair = el("div", "zen-panel-prop");
    const dd = el(mono ? "pre" : "dd", mono ? "zen-console-message" : "", value);
    dd.dataset.field = name;
    pair.append(el("dt", "", label), dd);
    return pair;
  }
  function renderDetails(): void {
    if (!selected) {
      const hint = panelEmpty("Select an entry");
      hint.classList.add("zen-console-hint");
      details.replaceChildren(hint);
      return;
    }
    const e = selected;
    const dl = el("dl", "zen-console-fields");
    dl.append(
      field("Time", "time", clock(e.time)),
      field("Source", "source", e.source),
      field("Level", "level", e.level),
      field("Message", "message", e.message, true),
    );
    details.replaceChildren(dl);
  }
  function select(e: LogEntry | null): void {
    selected = e;
    for (const [entry, r] of Array.from(rowOf)) r.setAttribute("aria-selected", String(entry === e));
    renderDetails();
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
    rowOf.clear();
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

  let offs: (() => void)[] = [];
  let started = false;

  renderDetails();
  rebuild();
  return {
    el: cols.el,
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
      add(capture.entries());
      offs.push(capture.subscribe((e) => add([e])));
      entries.sort((a, b) => a.time - b.time || a.seq - b.seq);
      rebuild();
    },
    count: () => entries.length,
    focus: () => list.focus(),
    destroy() {
      for (const off of offs) off();
      offs = [];
      tools.replaceChildren();
      cols.el.remove();
    },
  };
}
