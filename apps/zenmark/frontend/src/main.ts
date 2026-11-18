import "@zenmd/runtime/css/tokens.css";
import "@zenmd/runtime/css/base.css";
import "@zenmd/kit/styles/shell.css";
import { getApi } from "./api";
import { features } from "./shell/features";
import { createShell } from "./shell/shell";
import { followTheme, installConsoleCapture } from "@zenmd/kit";

installConsoleCapture(); // the Developer panel shows what happened before it was opened
followTheme();
void getApi().then(
  (api) => createShell(document.getElementById("app")!, { api, features }).start(),
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
