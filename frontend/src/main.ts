import "./styles/tokens.css";
import "./styles/base.css";
import "./styles/shell.css";
import { getApi } from "./api";
import { features } from "./shell/features";
import { createShell } from "./shell/shell";

void getApi().then((api) => createShell(document.getElementById("app")!, { api, features }).start());
