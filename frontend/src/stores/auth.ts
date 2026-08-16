import { defineStore } from "pinia";
import { ref } from "vue";

import { ApiError, clearToken } from "@/lib/api";
import { fetchCurrentUser, type CurrentUser } from "@/lib/settings";

/**
 * Holds the signed-in user for the lifetime of the page.
 *
 * The server remains the authority on every access decision; this only drives what the
 * interface offers, so route guards built on it are a convenience, not a gate.
 */
export const useAuthStore = defineStore("auth", () => {
  const user = ref<CurrentUser | null>(null);
  const loading = ref(false);
  const loaded = ref(false);
  // The shell and the view it renders both ask on mount; whoever asks while a request
  // is in flight waits for that one, as with the token refresh in lib/api.ts.
  let pending: Promise<CurrentUser | null> | null = null;

  /** Loads the current user once. Returns null when nobody is signed in. */
  async function load(force = false): Promise<CurrentUser | null> {
    if (loaded.value && !force) {
      return user.value;
    }
    if (pending && !force) {
      return pending;
    }

    pending = fetchUser();
    try {
      return await pending;
    } finally {
      pending = null;
    }
  }

  async function fetchUser(): Promise<CurrentUser | null> {
    loading.value = true;
    try {
      user.value = await fetchCurrentUser();
    } catch (err) {
      if (err instanceof ApiError && err.isUnauthenticated) {
        user.value = null;
      } else {
        throw err;
      }
    } finally {
      loading.value = false;
      loaded.value = true;
    }

    return user.value;
  }

  function reset(): void {
    user.value = null;
    loaded.value = false;
    clearToken();
  }

  return { user, loading, loaded, load, reset };
});

/**
 * Sends the browser to the server-rendered login page, returning here afterwards.
 * The parameter is `return`, matching getRedirectUrl in web/user.go, which stashes it
 * in a cookie so the redirect survives an external SAML round trip.
 */
export function redirectToLogin(): void {
  const target = encodeURIComponent(window.location.pathname + window.location.search);
  window.location.assign(`/login?return=${target}`);
}
