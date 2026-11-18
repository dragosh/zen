// @vitest-environment happy-dom
// The built-in sandbox plugin's frontend (builtins/plugins/sandbox/assets), loaded
// as the app loads it: an ES module with a default export, imported by the runtime.
import { existsSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { createFakeApi, type FakeApi } from "../api/fake";
import { installRuntime } from "./runtime";

const DIR = join(dirname(fileURLToPath(import.meta.url)), "../../../builtins/plugins/sandbox/assets");

let api: FakeApi;
let rt: ReturnType<typeof installRuntime>;
/** The plugin script, an ES module with a default export; the runtime gets it through importModule. */
let mod: unknown;
let root: HTMLElement;

beforeEach(async () => {
  document.body.replaceChildren();
  api = createFakeApi();
  api.invokeResult = { service: "running", images: ["python"] };
  rt = installRuntime({ importModule: async () => mod, api, navigate: () => {}, page: () => ({ path: "plugins/sandbox.md", title: "Sandbox" }) });
  root = document.body.appendChild(document.createElement("article"));
  vi.resetModules();
  // @ts-ignore plain browser JavaScript outside src, no types
  mod = await import("../../../builtins/plugins/sandbox/assets/sandbox.js");
  await rt.load({ js: ["/_plugins/sandbox/sandbox.js?v=test"], css: [] });
});
afterEach(() => rt.destroy());

const ATTRS = { image: "python", cmd: "python main.py", file: "main.py" };
const settle = async () => { for (let i = 0; i < 5; i++) await new Promise((r) => setTimeout(r, 0)); };

/** What zen renders for a sandbox block (the wrapper plus the Go plugin's HTML). */
function render(attrs: Record<string, string> = ATTRS, code = "print('hi')\n", setup = ""): HTMLElement {
  const w = document.createElement("div");
  w.setAttribute("data-zen-plugin", "sandbox");
  w.setAttribute("data-zen-name", "sandbox");
  w.setAttribute("data-zen-attrs", JSON.stringify(attrs));
  const box = w.appendChild(document.createElement("div"));
  box.className = "zen-sandbox " + (setup ? "zen-sandbox-setup" : "zen-sandbox-ready");
  if (setup) box.appendChild(document.createElement("div")).textContent = setup;
  const pre = box.appendChild(document.createElement("pre"));
  pre.className = "zen-sandbox-code";
  pre.textContent = code;
  root.replaceChildren(w);
  rt.mountAll(root);
  return w;
}
const q = <T extends Element = HTMLElement>(w: HTMLElement, sel: string) => w.querySelector<T>(sel);
const streamEvt = (call: string, c: object) => api.emit("plugin:stream", { call, chunk: JSON.stringify(c), done: false, error: "" });
const endEvt = (call: string, error = "") => api.emit("plugin:stream", { call, chunk: "", done: true, error });
const click = (w: HTMLElement, sel: string) => q<HTMLButtonElement>(w, sel)!.click();

test("ships a stylesheet that uses only the app's tokens", () => {
  expect(existsSync(join(DIR, "sandbox.css"))).toBe(true);
  const css = readFileSync(join(DIR, "sandbox.css"), "utf8");
  expect(css).not.toMatch(/#[0-9a-f]{3,8}\b|rgb\(|hsl\(/i);
  expect(css).toMatch(/var\(--lint-error\)/);
});

test("shows the code read-only, Run and Stop, and asks for the status", async () => {
  const w = render();
  await settle();
  expect(q(w, "textarea")).toBeNull();
  expect(q(w, ".zen-sandbox-code")!.textContent).toBe("print('hi')\n");
  expect(q<HTMLButtonElement>(w, ".zen-sandbox-run")!.textContent).toBe("Run");
  expect(q<HTMLButtonElement>(w, ".zen-sandbox-stop")!.disabled).toBe(true);
  expect(q(w, ".zen-sandbox-console")).not.toBeNull();
  expect(api.callsTo("pluginInvoke")).toEqual([["sandbox", "plugins/sandbox.md", JSON.stringify({ op: "status" })]]);
});

test("an editable block gets a textarea and runs the edited code", async () => {
  const w = render({ ...ATTRS, editable: "true" });
  await settle();
  const ta = q<HTMLTextAreaElement>(w, "textarea")!;
  expect(ta.value).toBe("print('hi')\n");
  ta.value = "print(2)";
  ta.dispatchEvent(new Event("input"));
  click(w, ".zen-sandbox-run");
  await settle();
  const [id, page, json] = api.callsTo("pluginStream")[0] as [string, string, string];
  expect(page).toBe("plugins/sandbox.md");
  expect(id).toBe("sandbox");
  expect(JSON.parse(json)).toEqual({ op: "run", image: "python", cmd: "python main.py", file: "main.py", code: "print(2)" });
});

test("Run streams output as plain text, stderr tagged, ANSI stripped, then the exit and duration", async () => {
  const w = render();
  await settle();
  click(w, ".zen-sandbox-run");
  await settle();
  expect(JSON.parse((api.callsTo("pluginStream")[0] as string[])[2])).toMatchObject({ op: "run", image: "python", code: "print('hi')\n" });
  expect(q<HTMLButtonElement>(w, ".zen-sandbox-run")!.disabled).toBe(true);
  expect(q<HTMLButtonElement>(w, ".zen-sandbox-stop")!.disabled).toBe(false);
  streamEvt("call-1", { s: "out", t: "\u001b[32mhello\u001b[0m <img src=x onerror=alert(1)>" });
  streamEvt("call-1", { s: "err", t: "boom" });
  streamEvt("call-1", { s: "exit", code: 0, ms: 1530 });
  endEvt("call-1");
  await settle();
  const con = q(w, ".zen-sandbox-console")!;
  expect(con.textContent).toBe("hello <img src=x onerror=alert(1)>\nboom\n");
  expect(con.querySelector("img")).toBeNull();
  expect(con.querySelectorAll(".zen-sandbox-err")).toHaveLength(1);
  expect(con.querySelector(".zen-sandbox-err")!.textContent).toBe("boom\n");
  const st = q(w, ".zen-sandbox-status")!.textContent!;
  expect(st).toContain("exit 0");
  expect(st).toContain("1.5 s");
  expect(q<HTMLButtonElement>(w, ".zen-sandbox-run")!.disabled).toBe(false);
  expect(q<HTMLButtonElement>(w, ".zen-sandbox-stop")!.disabled).toBe(true);
});

test("a non-zero exit and a timeout are reported", async () => {
  const w = render();
  await settle();
  click(w, ".zen-sandbox-run");
  await settle();
  streamEvt("call-1", { s: "exit", code: 3, ms: 20 });
  endEvt("call-1");
  await settle();
  expect(q(w, ".zen-sandbox-status")!.textContent).toContain("exit 3");
  click(w, ".zen-sandbox-run");
  await settle();
  streamEvt("call-2", { s: "exit", code: -1, ms: 30000, timedOut: true });
  endEvt("call-2");
  await settle();
  expect(q(w, ".zen-sandbox-status")!.textContent).toMatch(/timed out/i);
  expect(q(w, ".zen-sandbox-console")!.textContent).toBe(""); // a new run clears the console
});

test("Stop cancels the stream", async () => {
  const w = render();
  await settle();
  click(w, ".zen-sandbox-run");
  await settle();
  click(w, ".zen-sandbox-stop");
  await settle();
  expect(api.callsTo("pluginCancel")).toEqual([["call-1"]]);
  endEvt("call-1", "cancelled");
  await settle();
  expect(q(w, ".zen-sandbox-status")!.textContent).toMatch(/stopped/i);
  expect(q<HTMLButtonElement>(w, ".zen-sandbox-run")!.disabled).toBe(false);
});

test("a stream error (service stopped) is shown in the console", async () => {
  const w = render();
  await settle();
  click(w, ".zen-sandbox-run");
  await settle();
  endEvt("call-1", "The container service is not running. Run `container system start` in a terminal.");
  await settle();
  const err = q(w, ".zen-sandbox-console .zen-sandbox-err")!;
  expect(err.textContent).toContain("container system start");
  expect(q<HTMLButtonElement>(w, ".zen-sandbox-run")!.disabled).toBe(false);
});

test("says so when the service is stopped or the tool is missing", async () => {
  api.invokeResult = { service: "stopped", images: ["python"] };
  let w = render();
  await settle();
  expect(q(w, ".zen-sandbox-note")!.textContent).toContain("container system start");
  api.invokeResult = { service: "missing", images: [] };
  w = render();
  await settle();
  expect(q(w, ".zen-sandbox-note")!.textContent).toMatch(/not installed/i);
});

test("a block that is not configured shows the code and the message but no Run", async () => {
  api.calls.length = 0;
  const w = render(ATTRS, "print(1)", "Add an image to config.yml");
  await settle();
  expect(w.textContent).toContain("Add an image to config.yml");
  expect(q(w, ".zen-sandbox-code")!.textContent).toBe("print(1)");
  expect(q(w, ".zen-sandbox-run")).toBeNull();
  expect(api.callsTo("pluginInvoke")).toHaveLength(0);
});

test("the console is capped at 1 MB", async () => {
  const w = render();
  await settle();
  click(w, ".zen-sandbox-run");
  await settle();
  const line = "x".repeat(100_000);
  for (let i = 0; i < 12; i++) streamEvt("call-1", { s: "out", t: line });
  endEvt("call-1");
  await settle();
  const text = q(w, ".zen-sandbox-console")!.textContent!;
  expect(text.length).toBeLessThan(1_100_000);
  expect(text).toMatch(/truncated/i);
});

test("unmounting a running block cancels its stream", async () => {
  const w = render();
  await settle();
  click(w, ".zen-sandbox-run");
  await settle();
  rt.unmountAll();
  await settle();
  expect(api.callsTo("pluginCancel")).toEqual([["call-1"]]);
  expect(w.isConnected).toBe(true);
});
