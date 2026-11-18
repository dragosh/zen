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
  // @zenmd/runtime and @zenmd/kit are the repository's runtime/ and kit/ui (one node_modules, this one); the app bundle takes their sources.
  resolve: {
    alias: [
      { find: /^@zenmd\/runtime$/, replacement: resolve(import.meta.dirname, "../../../runtime/src/index.ts") },
      { find: /^@zenmd\/runtime\//, replacement: resolve(import.meta.dirname, "../../../runtime/src") + "/" },
      { find: /^@zenmd\/kit$/, replacement: resolve(import.meta.dirname, "../../../kit/ui/src/index.ts") },
      { find: /^@zenmd\/kit\//, replacement: resolve(import.meta.dirname, "../../../kit/ui/src") + "/" },
    ],
  },
  // One vitest run covers the app, the runtime and the kit (../runtime), with the one node_modules.
  // The runtime's tests all want a DOM (happy-dom is resolved from here, not from ../runtime, which has no node_modules).
  test: {
    projects: [
      { extends: true, test: { name: "app", include: ["src/**/*.test.ts"] } },
      { extends: true, test: { name: "runtime", environment: "happy-dom", include: ["../../../runtime/src/**/*.test.ts", "../../../runtime/test/**/*.test.ts"] } },
      // The kit's UI (kit/ui); its tests borrow the app's fake api, as the runtime's do.
      { extends: true, test: { name: "kit", include: ["../../../kit/ui/src/**/*.test.ts", "../../../kit/ui/test/**/*.test.ts"] } },
    ],
  },
});
