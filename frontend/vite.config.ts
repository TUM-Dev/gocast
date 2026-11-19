import { fileURLToPath, URL } from "node:url";

import vue from "@vitejs/plugin-vue";
import { defineConfig } from "vite";
import { VitePWA } from "vite-plugin-pwa";

// The Go server serves the built app: index.html is written to web/spa/ and embedded
// into the binary, while hashed assets are mounted at /spa-assets/.
const GO_SERVER = process.env.GOCAST_BACKEND ?? "http://localhost:8081";

// Runs the service worker under `vite dev` (`pnpm dev:pwa`). Off by default: a worker
// sits between the browser and HMR, and its caches outlive the edit that filled them.
const PWA_DEV = process.env.GOCAST_PWA_DEV === "1";

export default defineConfig(({ command }) => ({
  base: command === "build" ? "/spa-assets/" : "/",
  plugins: [
    vue(),
    VitePWA({
      strategies: "injectManifest",
      srcDir: "src",
      filename: "sw.ts",
      // Built into web/spa/ like everything else, but Go serves it at /sw.js: a worker
      // under /spa-assets/ could only control /spa-assets/. Registration and scope
      // therefore name the root, not Vite's base.
      buildBase: "/",
      scope: "/",
      // PwaStatus.vue asks before activating an update, and lib/pwa.ts registers.
      registerType: "prompt",
      injectRegister: false,
      // Go serves the manifest, and a deployment's branding can replace it.
      manifest: false,
      injectManifest: {
        globPatterns: ["**/*.{js,css,html,woff2,svg,png}"],
        manifestTransforms: [
          (entries) => ({
            manifest: entries.map((entry) => ({
              ...entry,
              // The list is relative to the worker, which sits at the root, while
              // the files are served under /spa-assets/. The shell is precached as /,
              // a URL Go answers with it; /spa-assets/index.html is a redirect.
              url: entry.url === "index.html" ? "/" : `/spa-assets/${entry.url}`,
            })),
            warnings: [],
          }),
        ],
      },
      devOptions: { enabled: PWA_DEV, type: "module" },
    }),
  ],
  define: {
    __PWA_DEV__: JSON.stringify(PWA_DEV),
  },
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
    // The only chunk above Vite's default 500 kB is video.js (~700 kB), which is one
    // third-party library, cannot be split usefully (the streaming stack is just a
    // sixth of it) and is imported dynamically by VideoPlayer, so it only loads on
    // pages that play video. The limit sits just above it so that any *app* chunk
    // growing past 500 kB, such as a route component pulled back into the entry,
    // still warns.
    chunkSizeWarningLimit: 750,
  },
  server: {
    port: 5173,
    // Listen on every interface so the dev server is reachable from outside the
    // container; harmless when run on the host.
    host: true,
    // Point the browser at this dev server and let everything the SPA does not own
    // fall through to Go. changeOrigin stays false so the host-scoped session cookie
    // survives the proxy hop.
    proxy: {
      "/api": { target: GO_SERVER, changeOrigin: false },
      "/public": { target: GO_SERVER, changeOrigin: false },
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
      "/manifest.json": { target: GO_SERVER, changeOrigin: false },
      "^/icons-\\d+\\.png$": { target: GO_SERVER, changeOrigin: false },
    },
  },
}));
