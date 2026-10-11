import { onScopeDispose } from "vue";

import type { PlayerController } from "@/lib/player/controller";
import { reportSeek } from "@/lib/watch-actions";

/** A scrub fires several seeks; only where the viewer settles is worth counting. */
export const SEEK_DEBOUNCE_MS = 4000;
/** How long after load a first `seeked` is taken to be the start-time jump, not the viewer. */
const INITIAL_SEEK_WINDOW_MS = 3000;

/**
 * Reports where a viewer jumps to, for the seek heatmap. One report per seek, debounced.
 *
 * The first `seeked` is skipped: it is the player moving to `?t=` or the saved progress
 * (the old SeekLogger did the same). That skip is lifted after a few seconds so a page
 * with no initial seek still counts the viewer's first jump. Attach it to ONE player; the
 * old page attached it twice (a second SeekLogger), which double-counted.
 *
 * `offset` is the clip's start in seconds: the server counts positions in the recording,
 * the controller speaks clip time.
 */
export function useSeekReporting(
  controller: Pick<PlayerController, "on" | "getCurrentTime">,
  streamId: number,
  options: { offset?: number; enabled?: () => boolean } = {},
): () => void {
  let skipping = true;
  let timer: ReturnType<typeof setTimeout> | undefined;
  const lift = setTimeout(() => (skipping = false), INITIAL_SEEK_WINDOW_MS);

  const off = controller.on("seeked", () => {
    if (options.enabled && !options.enabled()) return;
    if (skipping) {
      skipping = false;
      return;
    }
    clearTimeout(timer);
    timer = setTimeout(() => {
      // Reporting is best effort; a failed count must never surface to the viewer.
      void reportSeek(streamId, controller.getCurrentTime() + (options.offset ?? 0)).catch(() => {});
    }, SEEK_DEBOUNCE_MS);
  });

  const stop = () => {
    off();
    clearTimeout(timer);
    clearTimeout(lift);
  };
  onScopeDispose(stop);
  return stop;
}
