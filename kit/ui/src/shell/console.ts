// Captures the window's console (log, info, warn, error, debug), uncaught errors and unhandled
// rejections for the Developer panel (ui/developer-tab). The real console is still called with the
// same arguments, and nothing here logs, so the capture cannot feed itself.
import type { LogEntry, LogLevel } from "../host";

export const CONSOLE_CAP = 2000;
const MAX_MESSAGE = 4000;

export type ConsoleCapture = {
  /** What is kept, oldest first (at most the cap). */
  entries(): LogEntry[];
  /** Calls fn for every new entry; returns the unsubscribe. */
  subscribe(fn: (e: LogEntry) => void): () => void;
  /** Puts the console back and stops listening. */
  uninstall(): void;
};

const METHODS: [keyof Console, LogLevel][] = [
  ["debug", "debug"], ["log", "info"], ["info", "info"], ["warn", "warn"], ["error", "error"],
];

function text(v: unknown, seen = new WeakSet<object>()): string {
  if (typeof v === "string") return v;
  if (v instanceof Error) return v.stack || `${v.name}: ${v.message}`;
  if (v === undefined) return "undefined";
  if (typeof v === "function") return `[function ${v.name || "anonymous"}]`;
  if (typeof v === "object" && v !== null) {
    try {
      return JSON.stringify(v, (_k, x) => {
        if (typeof x === "object" && x !== null) {
          if (seen.has(x)) return "[circular]";
          seen.add(x);
        }
        return typeof x === "bigint" ? String(x) : x;
      }) ?? String(v);
    } catch {
      return Object.prototype.toString.call(v);
    }
  }
  return String(v);
}

function join(args: unknown[]): string {
  const s = args.map((a) => text(a)).join(" ");
  return s.length > MAX_MESSAGE ? `${s.slice(0, MAX_MESSAGE)}…` : s;
}

const KEY = Symbol.for("zenmd.consoleCapture");
type Holder = { [KEY]?: ConsoleCapture };

/** Starts capturing in `win`; a second call on the same window returns the first capture. */
export function installConsoleCapture(win: Window & typeof globalThis = window, cap = CONSOLE_CAP): ConsoleCapture {
  const holder = win as unknown as Holder;
  if (holder[KEY]) return holder[KEY]!;
  const kept: LogEntry[] = [];
  const subs = new Set<(e: LogEntry) => void>();
  let seq = 0;
  const record = (level: LogLevel, message: string): void => {
    const e: LogEntry = { seq: ++seq, time: Date.now(), level, source: "console", message };
    kept.push(e);
    if (kept.length > cap) kept.splice(0, kept.length - cap);
    for (const fn of Array.from(subs)) {
      try { fn(e); } catch { /* a broken subscriber must not break console.log */ }
    }
  };

  const originals = new Map<keyof Console, unknown>();
  for (const [name, level] of METHODS) {
    const orig = win.console[name] as (...a: unknown[]) => void;
    originals.set(name, orig);
    (win.console as unknown as Record<string, unknown>)[name] = function (this: unknown, ...args: unknown[]) {
      try { record(level, join(args)); } catch { /* never break the real console */ }
      return orig.apply(win.console, args);
    };
  }
  const wrapped = new Map(METHODS.map(([n]) => [n, win.console[n]]));

  const onError = (e: Event): void => {
    const ev = e as ErrorEvent;
    const where = ev.filename ? ` (${ev.filename}${ev.lineno ? `:${ev.lineno}` : ""})` : "";
    record("error", `${ev.error ? text(ev.error) : ev.message}${ev.error ? "" : where}`.slice(0, MAX_MESSAGE));
  };
  const onRejection = (e: Event): void => {
    record("error", `Unhandled rejection: ${text((e as PromiseRejectionEvent).reason)}`.slice(0, MAX_MESSAGE));
  };
  win.addEventListener("error", onError);
  win.addEventListener("unhandledrejection", onRejection);

  const capture: ConsoleCapture = {
    entries: () => kept.slice(),
    subscribe(fn) {
      subs.add(fn);
      return () => void subs.delete(fn);
    },
    uninstall() {
      originals.forEach((orig, name) => {
        // Leave a console someone else wrapped on top of ours alone.
        if (win.console[name] === wrapped.get(name)) (win.console as unknown as Record<string, unknown>)[name] = orig;
      });
      win.removeEventListener("error", onError);
      win.removeEventListener("unhandledrejection", onRejection);
      subs.clear();
      delete holder[KEY];
    },
  };
  holder[KEY] = capture;
  return capture;
}
