import { fileURLToPath, URL } from "node:url";

import vue from "@vitejs/plugin-vue";
import { defineConfig } from "vite";

// The Go server serves the built app: index.html is written to web/spa/ and embedded
// into the binary, while hashed assets are mounted at /spa-assets/ so that the legacy
// /static mount is left completely untouched.
const GO_SERVER = "http://localhost:8081";

export default defineConfig({
  base: "/spa-assets/",
  plugins: [vue()],
  resolve: {
    alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) },
  },
  build: {
    outDir: "../web/spa",
    // Left false deliberately: web/spa is a tracked directory (so the go:embed
    // directive in web/router.go resolves on a fresh checkout) and emptying it would
    // delete the marker and .gitignore. Stale hashed assets are cleared by the
    // `prebuild` script instead.
    emptyOutDir: false,
  },
  server: {
    port: 5173,
    // Point the browser at this dev server and let everything the SPA does not own
    // fall through to Go. changeOrigin stays false so the host-scoped session cookie
    // survives the proxy hop and the dev SPA shares a login with the legacy pages.
    proxy: {
      "/api": { target: GO_SERVER, changeOrigin: false },
      "/public": { target: GO_SERVER, changeOrigin: false },
      "/static": { target: GO_SERVER, changeOrigin: false },
      "/login": { target: GO_SERVER, changeOrigin: false },
      "/logout": { target: GO_SERVER, changeOrigin: false },
      "/saml": { target: GO_SERVER, changeOrigin: false },
      "/logo.svg": { target: GO_SERVER, changeOrigin: false },
    },
  },
});
