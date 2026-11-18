// @vitest-environment happy-dom
// The built-in quiz plugin's frontend (builtins/plugins/quiz/assets/quiz.js), mounted on the
// markup its Go block hook emits, with the answers checked through a fake ctx.invoke.
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { afterEach, beforeEach, expect, test } from "vitest";
import { createFakeApi, type FakeApi } from "../api/fake";
import { installRuntime } from "./runtime";

const ASSETS = join(dirname(fileURLToPath(import.meta.url)), "../../../builtins/plugins/quiz/assets");

let api: FakeApi;
let rt: ReturnType<typeof installRuntime>;
/** The plugin script, an ES module with a default export; the runtime gets it through importModule. */
let mod: unknown;
let root: HTMLElement;

beforeEach(async () => {
  document.body.replaceChildren();
  api = createFakeApi();
  rt = installRuntime({ importModule: async () => mod, api, navigate: () => {}, page: () => ({ path: "quizzes/http.md", title: "HTTP" }) });
  root = document.body.appendChild(document.createElement("article"));
  // @ts-ignore plain browser JavaScript outside src, no types
  mod = await import("../../../builtins/plugins/quiz/assets/quiz.js");
  await rt.load({ js: ["/_plugins/quiz/quiz.js?v=test"], css: [] });
});
afterEach(() => rt.destroy());

const settle = async () => { for (let i = 0; i < 8; i++) await new Promise((r) => setTimeout(r, 0)); };

/** What zen renders for the test quiz: Go's structure inside the wrapper. */
function render(attrs: Record<string, string> = { id: "http", title: "HTTP basics", pass: "60" }, name = "quiz"): HTMLElement {
  const w = document.createElement("div");
  w.setAttribute("data-zen-plugin", "quiz");
  w.setAttribute("data-zen-name", name);
  w.setAttribute("data-zen-attrs", JSON.stringify(attrs));
  if (name === "quiz") {
    w.innerHTML =
      '<ol class="zq-questions">' +
      '<li class="zq-q zq-single"><div class="zq-prompt">Which method is idempotent?</div><ul class="zq-options"><li>POST</li><li>PUT</li><li>PATCH</li></ul></li>' +
      '<li class="zq-q zq-multi"><div class="zq-prompt">Select the 2xx codes.</div><ul class="zq-options"><li>200</li><li>204</li><li>301</li></ul></li>' +
      '<li class="zq-q zq-text"><div class="zq-prompt">What does TLS stand for?</div></li></ol>';
  } else {
    w.innerHTML = '<div class="zq-results"></div>';
  }
  root.replaceChildren(w);
  rt.mountAll(root);
  return w;
}
const inputs = (w: HTMLElement, q: number) => Array.from(w.querySelectorAll<HTMLInputElement>(".zq-question")[q].querySelectorAll("input"));
const pick = (w: HTMLElement, q: number, label: string) => {
  const input = inputs(w, q).find((i) => i.closest("label")?.textContent === label)!;
  input.click();
};
const type = (w: HTMLElement, q: number, text: string) => {
  const i = inputs(w, q)[0];
  i.value = text;
  i.dispatchEvent(new Event("change", { bubbles: true }));
};
const submit = (w: HTMLElement) => { w.querySelector("form")!.dispatchEvent(new Event("submit", { cancelable: true, bubbles: true })); };
const stored = (id = "http") => JSON.parse(api.callsTo("pluginStorageSet").filter((c) => c[1] === `quiz-${id}`).at(-1)![2] as string);

const ALL_RIGHT = { ok: true, total: 3, score: 100, passed: true, results: [
  { q: 0, correct: true, explanation: "PUT replaces." }, { q: 1, correct: true }, { q: 2, correct: true }] };
const ONE_RIGHT = { ok: true, total: 3, score: 33, passed: false, results: [
  { q: 0, correct: false, explanation: "PUT replaces." }, { q: 1, correct: true }, { q: 2, correct: false }] };

test("builds labelled radio, checkbox and text inputs with DOM APIs", async () => {
  const w = render();
  await settle();
  expect(w.querySelector(".zq-title")?.textContent).toBe("HTTP basics");
  expect(w.querySelector(".zq-summary")?.textContent).toBe("Not attempted yet");
  expect(inputs(w, 0).map((i) => i.type)).toEqual(["radio", "radio", "radio"]);
  expect(inputs(w, 1).map((i) => i.type)).toEqual(["checkbox", "checkbox", "checkbox"]);
  expect(inputs(w, 2).map((i) => i.type)).toEqual(["text"]);
  expect(inputs(w, 2)[0].getAttribute("aria-label")).toBeTruthy();
  for (const i of [...inputs(w, 0), ...inputs(w, 1)]) expect(i.closest("label")).not.toBeNull();
  expect(new Set(inputs(w, 0).map((i) => i.name)).size).toBe(1);
  expect(w.querySelector(".zq-legend")?.textContent).toBe("1. Which method is idempotent?");
  expect(w.querySelector(".zq-outcome")?.getAttribute("aria-live")).toBe("polite");
  expect(w.querySelector("button[type=submit]")?.textContent).toBe("Check");
});

test("page text is never parsed as HTML", async () => {
  const w = document.createElement("div");
  w.setAttribute("data-zen-plugin", "quiz");
  w.setAttribute("data-zen-name", "quiz");
  w.setAttribute("data-zen-attrs", JSON.stringify({ id: "x", title: "<img src=x onerror=alert(1)>" }));
  const ol = w.appendChild(document.createElement("ol"));
  ol.className = "zq-questions";
  const li = ol.appendChild(document.createElement("li"));
  li.className = "zq-q zq-text";
  li.append(Object.assign(document.createElement("div"), { className: "zq-prompt", textContent: "<b>bold</b>?" }));
  root.replaceChildren(w);
  rt.mountAll(root);
  await settle();
  expect(w.querySelector("img")).toBeNull();
  expect(w.querySelector("b")).toBeNull();
  expect(w.querySelector(".zq-legend")?.textContent).toBe("1. <b>bold</b>?");
});

test("Check sends the answers by question index and marks each question", async () => {
  api.invokeResult = ONE_RIGHT;
  const w = render();
  await settle();
  pick(w, 0, "PUT");
  pick(w, 1, "200");
  pick(w, 1, "204");
  type(w, 2, "Transport");
  await settle();
  submit(w);
  await settle();
  const [id, page, payload] = api.callsTo("pluginInvoke")[0] as [string, string, string];
  expect(page).toBe("quizzes/http.md");
  expect(id).toBe("quiz");
  expect(JSON.parse(payload)).toEqual({ op: "check", quiz: "http", page: "quizzes/http.md", answers: { "0": 1, "1": [0, 1], "2": "Transport" } });
  const qs = Array.from(w.querySelectorAll(".zq-question"));
  expect(qs.map((q) => q.querySelector(".zq-status")?.textContent)).toEqual(["Incorrect", "Correct", "Incorrect"]);
  expect(qs.map((q) => q.classList.contains("zq-right"))).toEqual([false, true, false]);
  expect(qs[0].querySelector(".zq-explanation")?.textContent).toBe("PUT replaces.");
  expect(w.querySelector(".zq-outcome")?.textContent).toBe("Score 33% · Not passed (pass mark 60%)");
  expect(inputs(w, 0).every((i) => i.disabled)).toBe(true);
  expect(w.querySelector(".zq-summary")?.textContent).toBe("Best 33% · 1 attempt");
  const s = stored();
  expect(s.version).toBe(1);
  expect(s.best).toBe(33);
  expect(s.attempts).toHaveLength(1);
  expect(s.attempts[0].score).toBe(33);
  expect(s.attempts[0].date).toMatch(/^\d{4}-\d\d-\d\dT/);
  expect(s.answers).toEqual({});
});

test("an unanswered question is shown as not answered", async () => {
  api.invokeResult = { ok: true, total: 3, score: 33, passed: false, results: [{ q: 1, correct: true }] };
  const w = render();
  await settle();
  pick(w, 1, "200");
  submit(w);
  await settle();
  const qs = Array.from(w.querySelectorAll(".zq-question"));
  expect(qs.map((q) => q.querySelector(".zq-status")?.textContent)).toEqual(["Not answered", "Correct", "Not answered"]);
});

test("checking with nothing answered asks for an answer and does not call the backend", async () => {
  const w = render();
  await settle();
  submit(w);
  await settle();
  expect(api.callsTo("pluginInvoke")).toEqual([]);
  expect(w.querySelector(".zq-note")?.textContent).toMatch(/Answer at least one/);
});

test("a backend error is shown and the attempt is not counted", async () => {
  api.invokeResult = () => { throw new Error("this quiz is not known; reload the page"); };
  const w = render();
  await settle();
  pick(w, 0, "PUT");
  submit(w);
  await settle();
  expect(w.querySelector(".zq-note")?.textContent).toMatch(/Could not check: this quiz is not known/);
  expect(w.querySelector(".zq-summary")?.textContent).toBe("Not attempted yet");
  expect(inputs(w, 0)[0].disabled).toBe(false);
});

test("a pass shows the best score; Retry resets the form and keeps the history", async () => {
  api.invokeResult = ALL_RIGHT;
  let w = render();
  await settle();
  pick(w, 0, "PUT");
  submit(w);
  await settle();
  expect(w.querySelector(".zq-outcome")?.textContent).toBe("Score 100% · Passed");
  const retry = w.querySelector<HTMLButtonElement>(".zq-retry")!;
  expect(retry.hidden).toBe(false);
  retry.click();
  await settle();
  expect(inputs(w, 0).every((i) => !i.checked && !i.disabled)).toBe(true);
  expect(w.querySelector(".zq-outcome")?.textContent).toBe("");
  expect(w.querySelector(".zq-status")?.textContent).toBe("");
  api.invokeResult = ONE_RIGHT;
  pick(w, 0, "POST");
  submit(w);
  await settle();
  expect(w.querySelector(".zq-summary")?.textContent).toBe("Best 100% · 2 attempts");
  // reopening the page shows the history
  w = render();
  await settle();
  expect(w.querySelector(".zq-summary")?.textContent).toBe("Best 100% · 2 attempts");
});

test("in-progress answers are saved and come back", async () => {
  let w = render();
  await settle();
  pick(w, 0, "PATCH");
  pick(w, 1, "301");
  type(w, 2, "half done");
  await settle();
  expect(stored().answers).toEqual({ "0": 2, "1": [2], "2": "half done" });
  w = render();
  await settle();
  expect(inputs(w, 0).map((i) => i.checked)).toEqual([false, false, true]);
  expect(inputs(w, 1).map((i) => i.checked)).toEqual([false, false, true]);
  expect(inputs(w, 2)[0].value).toBe("half done");
});

test("the attempts limit stops further tries", async () => {
  api.invokeResult = ONE_RIGHT;
  const w = render({ id: "http", attempts: "1", pass: "60" });
  await settle();
  expect(w.querySelector(".zq-summary")?.textContent).toBe("Not attempted yet · 1 attempt allowed");
  pick(w, 0, "POST");
  submit(w);
  await settle();
  expect(w.querySelector(".zq-summary")?.textContent).toBe("Best 33% · 1 of 1 attempts");
  expect(w.querySelector<HTMLButtonElement>(".zq-retry")!.hidden).toBe(true);
  expect(w.querySelector(".zq-outcome")?.textContent).toMatch(/No attempts left/);
  // and after a reload the form stays locked
  const again = render({ id: "http", attempts: "1", pass: "60" });
  await settle();
  expect(inputs(again, 0).every((i) => i.disabled)).toBe(true);
  expect(again.querySelector<HTMLButtonElement>(".zq-check")!.hidden).toBe(true);
});

test("reveal=each checks one question at a time and scores at the end", async () => {
  api.invokeResult = (_id: string, p: any) => ({
    ok: true, total: 3, score: 0, passed: false,
    results: Object.keys(p.answers).map((k) => ({ q: Number(k), correct: k !== "0", explanation: k === "0" ? "Because." : "" })),
  });
  const w = render({ id: "http", reveal: "each", pass: "60" });
  await settle();
  expect(w.querySelector<HTMLButtonElement>(".zq-check")!.hidden).toBe(true);
  const buttons = Array.from(w.querySelectorAll<HTMLButtonElement>(".zq-check-one"));
  expect(buttons).toHaveLength(3);
  pick(w, 0, "POST");
  buttons[0].click();
  await settle();
  const first = JSON.parse((api.callsTo("pluginInvoke")[0] as string[])[2]);
  expect(first.answers).toEqual({ "0": 0 });
  expect(w.querySelectorAll(".zq-status")[0].textContent).toBe("Incorrect");
  expect(w.querySelector(".zq-outcome")?.textContent).toBe("");
  pick(w, 1, "200");
  pick(w, 1, "204");
  buttons[1].click();
  await settle();
  type(w, 2, "x");
  buttons[2].click();
  await settle();
  expect(w.querySelector(".zq-outcome")?.textContent).toBe("Score 67% · Passed");
  expect(stored().attempts[0].score).toBe(67);
});

test("shuffle changes the order, not the values sent", async () => {
  const orig = Math.random;
  Math.random = () => 0; // a fixed shuffle: every swap takes index 0
  try {
    api.invokeResult = ALL_RIGHT;
    const w = render({ id: "http", shuffle: "true" });
    await settle();
    const labels = inputs(w, 0).map((i) => i.closest("label")!.textContent);
    expect([...labels].sort()).toEqual(["PATCH", "POST", "PUT"]);
    expect(labels).not.toEqual(["POST", "PUT", "PATCH"]);
    pick(w, 0, "PUT");
    submit(w);
    await settle();
    expect(JSON.parse((api.callsTo("pluginInvoke")[0] as string[])[2]).answers["0"]).toBe(1);
  } finally {
    Math.random = orig;
  }
});

test("a damaged stored value is replaced, not fatal", async () => {
  await api.pluginStorageSet("quiz", "quiz-http", JSON.stringify({ version: 7, junk: true }));
  const w = render();
  await settle();
  expect(w.querySelector(".zq-summary")?.textContent).toBe("Not attempted yet");
  expect(w.querySelector(".zq-plugin-error, .zen-plugin-error")).toBeNull();
});

test("clean-up puts the rendered block back", async () => {
  const w = render();
  await settle();
  rt.unmountAll();
  expect(w.querySelector("form")).toBeNull();
  expect(w.querySelector("ol.zq-questions")).not.toBeNull();
});

test("quiz-results lists every stored quiz with best score and attempts", async () => {
  await api.pluginStorageSet("quiz", "quiz-http", JSON.stringify({ version: 1, answers: {}, attempts: [{ date: "2026-01-01T00:00:00Z", score: 60 }, { date: "2026-01-02T00:00:00Z", score: 80 }], best: 80 }));
  await api.pluginStorageSet("quiz", "quiz-zen", JSON.stringify({ version: 1, answers: {}, attempts: [{ date: "2026-01-01T00:00:00Z", score: 40 }], best: 40 }));
  await api.pluginStorageSet("quiz", "other", JSON.stringify({ version: 1, answers: {}, attempts: [], best: 0 }));
  const w = render({}, "quiz-results");
  await settle();
  expect(api.callsTo("pluginStorageKeys")[0]).toEqual(["quiz", "quiz-"]);
  const items = Array.from(w.querySelectorAll("li")).map((li) => li.textContent?.replace(/\s+/g, " ").trim());
  expect(items).toEqual(["http Best 80% · 2 attempts", "zen Best 40% · 1 attempt"]);
});

test("quiz-results with nothing stored says so", async () => {
  const w = render({}, "quiz-results");
  await settle();
  expect(w.textContent).toContain("No quizzes taken yet.");
});

test("the stylesheet uses the app's tokens and no fixed colours", () => {
  const css = readFileSync(join(ASSETS, "quiz.css"), "utf8");
  expect(css).not.toMatch(/#[0-9a-f]{3,8}\b|rgb\(|hsl\(/i);
  expect(css).toMatch(/var\(--lint-error\)/);
  expect(css).toMatch(/var\(--code-meta\)/);
});
