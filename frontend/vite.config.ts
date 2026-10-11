import { fileURLToPath, URL } from "node:url";

import vue from "@vitejs/plugin-vue";
import { defineConfig } from "vite";

// The Go server serves the built app: index.html is written to web/spa/ and embedded
// into the binary, while hashed assets are mounted at /spa-assets/ so that the legacy
// /static mount is left completely untouched.
const GO_SERVER = process.env.GOCAST_BACKEND ?? "http://localhost:8081";

export default defineConfig(({ command }) => ({
  base: command === "build" ? "/spa-assets/" : "/",
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
    // Listen on every interface so the dev server is reachable from outside the
    // container; harmless when run on the host.
    host: true,
    // Point the browser at this dev server and let everything the SPA does not own
    // fall through to Go. changeOrigin stays false so the host-scoped session cookie
    // survives the proxy hop and the dev SPA shares a login with the legacy pages.
    proxy: {
      "/api": { target: GO_SERVER, changeOrigin: false },
      "/public": { target: GO_SERVER, changeOrigin: false },
      "/static": { target: GO_SERVER, changeOrigin: false },
      "/login": {
        target: GO_SERVER,
        changeOrigin: false,
        // GET /login is an SPA page, so Vite serves it: proxied, Go answers with the
        // built shell, whose /spa-assets/ scripts do not exist here and the page stays
        // blank. Only the form's POST goes to Go. This skips the server's redirect-cookie
        // hook, so in dev a login lands on / rather than the page that asked for it.
        bypass: (req) => (req.method === "GET" ? "/index.html" : undefined),
      },
      "/logout": { target: GO_SERVER, changeOrigin: false },
      "/saml": { target: GO_SERVER, changeOrigin: false },
      "/logo.svg": { target: GO_SERVER, changeOrigin: false },
      "/favicon.ico": { target: GO_SERVER, changeOrigin: false },
    },
  },
}));
