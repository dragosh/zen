// The only place the shell learns about features.
import { editorFeature } from "../editor";
import "../editor/editor.css";
import { navFeature } from "../nav";
import "../nav/nav.css";
import { pageFeature } from "../page";
import "../page/page.css";
import type { Feature } from "./types";

export const features: Feature[] = [navFeature, pageFeature, editorFeature];
