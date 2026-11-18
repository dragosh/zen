import "@zenmd/runtime/css/tokens.css";
import "@zenmd/runtime/css/base.css";
import "@zenmd/kit/styles/shell.css";
import { getApi } from "./api";
import { features } from "./shell/features";
import { createShell } from "./shell/shell";
import { followTheme, installConsoleCapture } from "@zenmd/kit";

installConsoleCapture(); // the Developer panel shows what happened before it was opened
followTheme();
void getApi().then((api) => createShell(document.getElementById("app")!, { api, features }).start());
