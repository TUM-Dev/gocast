import { registerSW } from "virtual:pwa-register";
import { ref } from "vue";

/**
 * The service worker's state, for PwaStatus.vue. Module-level like download-notice.ts:
 * the worker is registered once per tab, from main.ts, and the banner only reads.
 */

/** A new version is installed and waits for the user to reload into it. */
export const updateAvailable = ref(false);

/** Whether the browser has a network connection, as far as it can tell. */
export const online = ref(typeof navigator === "undefined" ? true : navigator.onLine);

let activateUpdate: () => Promise<void> = async () => {};

/** Activates the waiting version; the page reloads once it controls the tab. */
export function applyUpdate(): Promise<void> {
  updateAvailable.value = false;
  return activateUpdate();
}

export function dismissUpdate(): void {
  updateAvailable.value = false;
}

/**
 * How often an open tab asks for a new worker. Browsers only check on navigation, and a
 * tab of this app can stay on one lecture for hours.
 */
const UPDATE_CHECK_MS = 60 * 60 * 1000;

export function watchConnectivity(): void {
  window.addEventListener("online", () => (online.value = true));
  window.addEventListener("offline", () => (online.value = false));
}

/**
 * Whether this is the installed app, with a service worker in control: the only place
 * lectures are saved offline. In a browser tab the storage is the first thing evicted
 * and nothing would bring the viewer back to the saved lecture, so the feature is not
 * offered there.
 */
export const installed = ref(false);

/**
 * The display modes the installed app runs in. The manifest asks for minimal-ui, which
 * browsers that lack it replace with standalone.
 */
const INSTALLED_MODES = ["standalone", "minimal-ui", "fullscreen", "window-controls-overlay"];

/** Whether the page runs as an installed app rather than in a browser tab. */
export function runsInstalled(win: Window = window): boolean {
  // Safari on iOS has no display-mode for a home screen app, only this.
  if ((win.navigator as Navigator & { standalone?: boolean }).standalone) return true;
  return INSTALLED_MODES.some((mode) => win.matchMedia?.(`(display-mode: ${mode})`).matches);
}

export function watchInstalled(): void {
  if (!("serviceWorker" in navigator)) return;
  const update = () => (installed.value = !!navigator.serviceWorker.controller && runsInstalled());
  update();
  navigator.serviceWorker.addEventListener("controllerchange", update);
  for (const mode of INSTALLED_MODES) {
    window.matchMedia?.(`(display-mode: ${mode})`).addEventListener?.("change", update);
  }
}

export async function registerServiceWorker(): Promise<void> {
  if (!("serviceWorker" in navigator)) return;

  if (import.meta.env.DEV && !__PWA_DEV__) {
    // A worker from an earlier `pnpm dev:pwa` stays registered in the browser and keeps
    // answering requests from its caches; plain `pnpm dev` takes it out of the way.
    const registrations = await navigator.serviceWorker.getRegistrations();
    await Promise.all(registrations.map((registration) => registration.unregister()));
    return;
  }

  const updateSW = registerSW({
    immediate: true,
    onNeedRefresh: () => (updateAvailable.value = true),
    onRegisteredSW: (_url, registration) => {
      if (!registration) return;
      setInterval(() => {
        // An update check while offline only fails; the next interval tries again.
        if (navigator.onLine) registration.update().catch(() => {});
      }, UPDATE_CHECK_MS);
    },
  });
  activateUpdate = () => updateSW(true);
}
