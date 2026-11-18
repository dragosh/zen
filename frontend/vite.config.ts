import { defineConfig } from "vitest/config";
import wails from "@wailsio/runtime/plugins/vite";

// https://vitejs.dev/config/
export default defineConfig({
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
  plugins: [wails("./bindings")],
  // One vitest run covers the app and the runtime (../runtime), with the one node_modules.
  test: {
    include: ["src/**/*.test.ts", "../runtime/src/**/*.test.ts", "../runtime/test/**/*.test.ts"],
  },
});
