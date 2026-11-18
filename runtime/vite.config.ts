// Builds the browser runtime into runtime/dist/zen.js (a classic IIFE with no imports, for <script src>) and
// runtime/dist/zen.css. Embedded in Go by runtime/embed.go, used by `zenmd build` and the app. The only
// vite is the app frontend's (apps/zenmd-app/frontend; runtime has no node_modules): run it with `npm run build` here or there.
// A plain object, so this file imports nothing and needs no package of its own.
import { resolve } from "node:path";

export default {
  build: {
    outDir: resolve(import.meta.dirname, "dist"),
    emptyOutDir: true,
    minify: true,
    cssCodeSplit: false,
    lib: {
      entry: resolve(import.meta.dirname, "src/static/static.ts"),
      name: "zenStatic",
      formats: ["iife"],
      fileName: () => "zen.js",
      cssFileName: "zen",
    },
  },
};
