import { ref } from "vue";

import { ApiError } from "./api";
import { fetchProgress, type StreamProgress } from "./streams";

/** Retries after the first attempt, so a request is made at most 1 + this many times. */
export const PROGRESS_RETRIES = 2;
/** Wait before the first retry; each further one waits twice as long. */
export const PROGRESS_BACKOFF_MS = 400;

const sleep = (ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms));

/**
 * Asks for the progress, retrying a failure a couple of times with a short backoff:
 * under load the endpoint fails transiently, and a page that gave up on the first
 * failure showed the viewer's watched lectures as unwatched until the next reload.
 *
 * A 401 is not a failure. It means there is no signed-in viewer, and so no progress.
 */
async function fetchWithRetry(ids: number[]): Promise<Map<number, StreamProgress>> {
  for (let attempt = 0; ; attempt++) {
    try {
      return await fetchProgress(ids);
    } catch (err) {
      if (err instanceof ApiError && err.isUnauthenticated) return new Map();
      if (attempt >= PROGRESS_RETRIES) throw err;
      await sleep(PROGRESS_BACKOFF_MS * 2 ** attempt);
    }
  }
}

/**
 * The signed-in viewer's watch progress for the lectures a page lists, and whether
 * loading it failed.
 *
 * A failure used to be swallowed into an empty map, which made every eye icon and the
 * "Hide watched" filter vanish with no sign that anything was wrong. `failed` lets the
 * page say so (see ProgressLoadNotice) while everything else keeps working, and
 * `retry()` asks again for exactly the lectures that are still missing.
 */
export function useWatchProgress() {
  const progress = ref<Map<number, StreamProgress>>(new Map());
  const failed = ref(false);
  const loading = ref(false);

  let missing = new Set<number>();
  // Bumped by reset(): an answer that arrives after the page moved on is dropped.
  let generation = 0;

  async function load(ids: number[]): Promise<void> {
    if (ids.length === 0) return;
    const mine = generation;
    loading.value = true;
    try {
      const loaded = await fetchWithRetry(ids);
      if (mine !== generation) return;
      progress.value = new Map([...progress.value, ...loaded]);
      ids.forEach((id) => missing.delete(id));
      failed.value = missing.size > 0;
    } catch {
      if (mine !== generation) return;
      ids.forEach((id) => missing.add(id));
      failed.value = true;
    } finally {
      if (mine === generation) loading.value = false;
    }
  }

  /** Asks again for the lectures whose progress could not be loaded. */
  function retry(): Promise<void> {
    return load([...missing]);
  }

  function reset(): void {
    generation++;
    progress.value = new Map();
    missing = new Set();
    failed.value = false;
    loading.value = false;
  }

  return { progress, failed, loading, load, retry, reset };
}
