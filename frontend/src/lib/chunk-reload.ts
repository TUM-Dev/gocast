/**
 * Route components are separate files with hashed names. After a deploy the files an
 * open tab still points at are gone and the server answers 404, so the first
 * navigation to a page that tab has not visited yet fails. Loading the page afresh
 * fetches the new index.html and with it the new names.
 */

const RELOAD_KEY = "gocast:chunk-reload";

/**
 * A second failure inside this window is a real outage (or a broken deploy) rather
 * than a stale tab, and reloading again would loop forever.
 */
const RELOAD_WINDOW_MS = 60_000;

/** The wording differs per browser; these are Chromium, Firefox and Safari. */
const CHUNK_ERROR =
  /Failed to fetch dynamically imported module|error loading dynamically imported module|Importing a module script failed|Unable to preload CSS/i;

export function isChunkLoadError(err: unknown): boolean {
  return err instanceof Error && CHUNK_ERROR.test(err.message);
}

/**
 * Loads `target` from the server if this tab has not just done so already, and says
 * whether it did. Storage can be unavailable (private windows, blocked site data); then
 * the loop guard is missing, so the safe answer is not to reload.
 */
export function reloadOnce(
  target: string,
  assign: (url: string) => void = (url) => window.location.assign(url),
  now: () => number = Date.now,
): boolean {
  try {
    const last = Number(sessionStorage.getItem(RELOAD_KEY));
    if (last && now() - last < RELOAD_WINDOW_MS) return false;
    sessionStorage.setItem(RELOAD_KEY, String(now()));
  } catch {
    return false;
  }
  assign(target);
  return true;
}
