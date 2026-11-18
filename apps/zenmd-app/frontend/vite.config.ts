import { defineConfig } from "vitest/config";
import { resolve } from "node:path";
import wails from "@wailsio/runtime/plugins/vite";

// https://vitejs.dev/config/
export default defineConfig({
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
    fs: { allow: [resolve(import.meta.dirname, "../../..")] }, // ../runtime is served too
  },
  plugins: [wails("./bindings")],
  // @zenmd/runtime is the repository's runtime/ (one node_modules, the frontend's); the app bundle takes its sources.
  resolve: {
    alias: [
      { find: /^@zenmd\/runtime$/, replacement: resolve(import.meta.dirname, "../../../runtime/src/index.ts") },
      { find: /^@zenmd\/runtime\//, replacement: resolve(import.meta.dirname, "../../../runtime/src") + "/" },
    ],
  },
  // One vitest run covers the app and the runtime (../runtime), with the one node_modules.
  // The runtime's tests all want a DOM (happy-dom is resolved from here, not from ../runtime, which has no node_modules).
  test: {
    projects: [
      { extends: true, test: { name: "app", include: ["src/**/*.test.ts"] } },
      { extends: true, test: { name: "runtime", environment: "happy-dom", include: ["../../../runtime/src/**/*.test.ts", "../../../runtime/test/**/*.test.ts"] } },
    ],
  },
});
