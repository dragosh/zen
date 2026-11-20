import "@zenmd/runtime/css/tokens.css";
import "@zenmd/runtime/css/base.css";
import "@zenmd/kit/styles/shell.css";
import { getApi } from "./api";
import { features } from "./shell/features";
import { createShell } from "./shell/shell";
import { followTheme, installConsoleCapture } from "@zenmd/kit";
import { followAccessibility, stepZoom } from "@zenmd/kit/shell/a11y";
import { zoomKey } from "@zenmd/kit/shell/keys";
import { Events, Window } from "@wailsio/runtime";

installConsoleCapture(); // the Developer panel shows what happened before it was opened
followTheme();

// Zoom (Settings › Accessibility, View › Zoom In / Zoom Out / Actual Size): the whole window scales (Window.SetZoom), and
// Go is told the value so the menu items can be greyed at the ends. Outside the app (the browser preview) there is no
// window to scale, and nothing is raised.
const quiet = (call: () => unknown) => { try { void Promise.resolve(call()).catch(() => undefined); } catch { /* not in the app */ } };
const setZoom = (z: number) => {
  quiet(() => Window.SetZoom(z));
  quiet(() => Events.Emit("app:zoom", z));
};
// The options are on <html> (and a stored zoom is set) before the shell mounts, so no window shows the defaults first.
followAccessibility(document, setZoom);

// The View menu sends its events to every window and the one in front steps; ⌘+ ⌘= ⌘− ⌘0 do the same when the menu does not
// take the key. A key and a menu event for the same step within 100 ms are one step.
let last = { dir: 2 as -1 | 0 | 1 | 2, at: 0, from: "" };
function zoom(dir: -1 | 0 | 1, from: "key" | "menu"): void {
  const now = Date.now();
  if (last.dir === dir && last.from !== from && now - last.at < 100) return;
  last = { dir, at: now, from };
  stepZoom(dir, setZoom);
}
document.addEventListener("keydown", (e) => {
  const dir = zoomKey(e);
  if (dir === null) return;
  e.preventDefault();
  zoom(dir, "key");
});
void getApi().then(
  (api) => {
    for (const [event, dir] of [["menu:zoom-in", 1], ["menu:zoom-out", -1], ["menu:zoom-reset", 0]] as const) {
      api.on(event, () => { if (document.hasFocus()) zoom(dir, "menu"); });
    }
    return createShell(document.getElementById("app")!, { api, features }).start();
  },
  (err) => {
    // Inside the app there is no fallback (zenmark-org/zenmd#1): say what went wrong.
    console.error("zen: could not reach the app", err);
    const box = document.createElement("main");
    box.className = "zen-error";
    box.setAttribute("role", "alert");
    const h = document.createElement("h1");
    h.textContent = "Zenmark could not start";
    const p = document.createElement("p");
    p.textContent = err instanceof Error ? err.message : String(err);
    box.append(h, p);
    document.getElementById("app")!.replaceChildren(box);
  },
);
