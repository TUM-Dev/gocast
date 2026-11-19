/**
 * Which requests the service worker (src/sw.ts) handles. Kept apart from the worker so
 * the tests can import it without a ServiceWorkerGlobalScope.
 */

/**
 * Paths a navigation can reach that are not the SPA: the server answers them itself
 * (redirects to the IdP, the session teardown, files from the static directory) and the
 * shell standing in for them while offline would render a page that does not exist.
 * Saved lectures' media is the service worker's own (lib/offline/media.ts).
 */
const SERVER_OWNED = [/^\/api\//, /^\/saml(\/|$)/, /^\/logout(\/|$)/, /^\/public\//, /^\/offline-media\//];

/** Whether a navigation to `url` is an SPA page, which the shell can answer offline. */
export function isAppNavigation(url: URL): boolean {
  return !SERVER_OWNED.some((pattern) => pattern.test(url.pathname));
}

/**
 * The branding files Go serves at the root. A deployment can override them at runtime
 * (tools.Cfg.Paths.Branding), so their contents are unknown at build time and they
 * cannot be precached.
 */
const BRANDING = new Set([
  "/manifest.json",
  "/favicon.ico",
  "/logo.svg",
  "/icons-192.png",
  "/icons-512.png",
  "/thumb-fallback.png",
]);

export function isBrandingFile(url: URL): boolean {
  return BRANDING.has(url.pathname);
}

/** The URL the shell is precached under; see manifestTransforms in vite.config.ts. */
export const SHELL_URL = "/";
