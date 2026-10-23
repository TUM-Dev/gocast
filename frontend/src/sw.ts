/// <reference lib="webworker" />
/**
 * The service worker. vite-plugin-pwa compiles it into web/spa/sw.js, which Go serves
 * at /sw.js so its scope covers the whole site (see web/router.go). /service-worker.js
 * is a different worker on purpose: it retires the one the server-rendered pages
 * registered and must keep doing so.
 *
 * It makes the app installable and lets the shell load offline. Nothing from the API is
 * cached yet: offline, the pages render and their requests fail, which the app's
 * offline banner explains.
 */
import { clientsClaim } from "workbox-core";
import { cleanupOutdatedCaches, matchPrecache, precacheAndRoute } from "workbox-precaching";
import { registerRoute } from "workbox-routing";
import { NetworkFirst, StaleWhileRevalidate } from "workbox-strategies";

import { OFFLINE_MEDIA_PREFIX, serveOfflineMedia } from "./lib/offline/media";
import { isAppNavigation, isBrandingFile, SHELL_URL } from "./lib/sw-routes";

declare const self: ServiceWorkerGlobalScope;

/** Where `pnpm dev:pwa` keeps what it loaded; unused in a build. */
const DEV_CACHE = "gocast-dev";

// The hashed bundle, plus the shell under SHELL_URL. The list is injected at build time.
precacheAndRoute(self.__WB_MANIFEST);
cleanupOutdatedCaches();

// Saved lectures: decrypted from the device, one segment at a time, and only while the
// licence holds (lib/offline/media.ts). Registered first, so nothing else answers them,
// online or not.
registerRoute(
  ({ url }) => url.origin === self.location.origin && url.pathname.startsWith(OFFLINE_MEDIA_PREFIX),
  ({ url }) => serveOfflineMedia(url),
);

// Navigations go to the network first, and the precached shell answers only when the
// network fails. Do not turn this around into cache-first: the server does more than
// hand out the shell. It redirects a fresh installation to /onboarding, refuses pages
// the user may not see, and writes the error status into the shell's meta tags (see
// serveErrorPage). A cached shell would skip all of that and render the page the URL
// names, for a request the server would have refused.
registerRoute(
  ({ request, url }) => request.mode === "navigate" && isAppNavigation(url),
  async ({ request }) => {
    try {
      const response = await fetch(request);
      // Vite serves the same index.html for every page, so the last one stands in for
      // all of them. Only ok responses: an error page is not the shell.
      if (import.meta.env.DEV && response.ok) {
        const cache = await caches.open(DEV_CACHE);
        await cache.put(SHELL_URL, response.clone());
      }
      return response;
    } catch {
      return (
        (await matchPrecache(SHELL_URL)) ??
        (import.meta.env.DEV ? await caches.match(SHELL_URL, { cacheName: DEV_CACHE }) : undefined) ??
        Response.error()
      );
    }
  },
);

if (import.meta.env.DEV) {
  // `pnpm dev:pwa` has no build, so the precache list is empty and the app is the
  // unbundled modules Vite serves on request (/src/..., /@vite/client, /node_modules/
  // .vite/deps/...). Caching whatever loaded lets the offline reload be tried in dev.
  // Never in a build: there the precache holds exactly the assets of one version.
  registerRoute(
    ({ request, url }) =>
      request.method === "GET" && url.origin === self.location.origin && isAppNavigation(url),
    new NetworkFirst({ cacheName: DEV_CACHE }),
  );
}

registerRoute(
  ({ url }) => url.origin === self.location.origin && isBrandingFile(url),
  new StaleWhileRevalidate({ cacheName: "gocast-branding" }),
);

// A new version waits until the user agrees to reload (PwaStatus.vue): taking over on
// its own would reload a tab in the middle of a lecture.
self.addEventListener("message", (event) => {
  if (event.data?.type === "SKIP_WAITING") void self.skipWaiting();
});

// The first install controls the open tab right away, so it works offline without a
// reload. Updates still wait for SKIP_WAITING above.
clientsClaim();
