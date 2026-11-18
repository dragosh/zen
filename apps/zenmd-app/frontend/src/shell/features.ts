// The only place the shell learns about features (the kit provides these three).
import { editorFeature, navFeature, pageFeature } from "@zenmd/kit";
import "@zenmd/kit/editor/editor.css";
import "@zenmd/kit/nav/nav.css";
import "@zenmd/runtime/css/page.css";
import type { Feature } from "@zenmd/kit";

export const features: Feature[] = [navFeature, pageFeature, editorFeature];
