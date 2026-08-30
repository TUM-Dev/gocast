import { fileURLToPath, URL } from "node:url";

import vue from "@vitejs/plugin-vue";
import { defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) },
  },
  test: {
    environment: "happy-dom",
    setupFiles: ["src/test/setup.ts"],
    // e2e/ holds Playwright specs, which need a browser and a running server. Vitest
    // would otherwise collect them by name and fail on the import.
    include: ["src/**/*.test.ts"],
    // Generated code is exercised through the modules that use it, not directly.
    coverage: { exclude: ["src/gen/**"] },
  },
});
