import { timestampDate } from "@bufbuild/protobuf/wkt";

import type { StreamWatch } from "./watch";

/** First wait between checks, and the ceiling the backoff grows to. */
export const POLL_FIRST_MS = 15_000;
export const POLL_MAX_MS = 60_000;
/** Give up after this long; transcoding that takes more is not going to land on this page view. */
export const POLL_GIVE_UP_MS = 30 * 60_000;
/**
 * How long after its scheduled end a stream with no recording still counts as "being
 * processed" when the viewer arrived afterwards (they did not see it end). Past that it
 * is an old lecture that has no recording, and polling it would be a request every
 * minute for nothing.
 */
export const RECENT_END_MS = 2 * 60 * 60_000;

/** The wait before check number `attempt` (0-based): 15 s, 30 s, then 60 s. */
export function pollDelay(attempt: number): number {
  return Math.min(POLL_MAX_MS, POLL_FIRST_MS * 2 ** attempt);
}

/**
 * Whether an ended stream without a playable recording is likely to get one soon:
 * the viewer saw it end on this page (`sawEnd`), or its slot ended only recently.
 */
export function expectsRecording(watch: StreamWatch, now: Date, sawEnd: boolean): boolean {
  const s = watch.stream;
  if (!s || s.recording || s.liveNow) return false;
  if (sawEnd) return true;
  if (!s.ended || !s.end) return false;
  return now.getTime() - timestampDate(s.end).getTime() < RECENT_END_MS;
}

export interface RecordingPoller {
  stop(): void;
}

/**
 * Calls `check` on a backoff until it resolves true, `stop()` is called or
 * {@link POLL_GIVE_UP_MS} has passed (`onGiveUp` then runs). A `setTimeout` chain, so a slow check cannot
 * overlap the next one; a check that throws is just a miss.
 */
export function pollForRecording(
  check: () => Promise<boolean>,
  options: { giveUpMs?: number; onGiveUp?: () => void } = {},
): RecordingPoller {
  const giveUpMs = options.giveUpMs ?? POLL_GIVE_UP_MS;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let stopped = false;
  let waited = 0;
  let attempt = 0;

  const stop = () => {
    stopped = true;
    clearTimeout(timer);
  };
  const schedule = () => {
    const delay = pollDelay(attempt++);
    if (waited + delay > giveUpMs) {
      options.onGiveUp?.();
      return;
    }
    timer = setTimeout(async () => {
      waited += delay;
      const done = await check().catch(() => false);
      if (stopped) return;
      if (done) stop();
      else schedule();
    }, delay);
  };
  schedule();
  return { stop };
}
