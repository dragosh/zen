// Builds the runtime of `zenmd build` into dist/static/zen.js (a classic IIFE with no imports) and
// dist/static/zen.css. Run after the app build, which empties dist:
//   vite build --config src/static/vite.static.config.ts
import { defineConfig } from "vite";
import { resolve } from "node:path";

export default defineConfig({
  build: {
    outDir: resolve(import.meta.dirname, "../../dist/static"),
    emptyOutDir: true,
    minify: true,
    cssCodeSplit: false,
    lib: {
      entry: resolve(import.meta.dirname, "static.ts"),
      name: "zenStatic",
      formats: ["iife"],
      fileName: () => "zen.js",
      cssFileName: "zen",
    },
  },
});
