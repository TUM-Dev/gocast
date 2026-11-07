/** The slice of a video.js player that recovery uses. */
export interface RecoverablePlayer {
  on(event: string, fn: () => void): void;
  off(event: string, fn: () => void): void;
  error(): { code: number } | null;
  /** Clears the error, which is what lets the next `src` start from a clean state. */
  error(value: null): void;
  src(source: { src: string; type: string }): void;
  currentTime(): number;
  currentTime(seconds: number): void;
  paused(): boolean;
  play(): Promise<void> | undefined;
}

export interface Recovery {
  /** Forgets the streak and everything pending, for a player that is given a new source. */
  reset(): void;
  /** Stops listening. */
  dispose(): void;
}

// MediaError codes that a reload cannot fix: the viewer aborted or the media is
// encrypted. Everything else is what a server that is briefly gone looks like, decode
// errors included: VHS reports a segment cut short by a restart as one. Corrupt media
// costs one retry cycle before the message.
const FATAL_CODES = new Set([1, 5]);

/** Milliseconds to wait per attempt: about half a minute in all, to outlast a redeploy. */
export const RECOVERY_DELAYS_MS = [1000, 2000, 4000, 8000, 8000, 8000];

/**
 * Reloads the source after a playback error, at the position and play state the viewer
 * left, so that a restart of the video server shows a stall instead of an error. VHS
 * retries a failed segment on its own only while it has a playlist to fall back to; a
 * failure while the server is down for the first playlist, or for the last rendition, is
 * fatal and leaves a dead player behind.
 *
 * A reload only counts as recovered once the seek back to the saved position has
 * completed (or, from the start, once the media can play): the metadata arriving proves
 * nothing, since the segment at the saved position may be the one that keeps failing.
 * Until then the attempts keep counting and the saved position is kept.
 *
 * A live stream resumes at the live edge: its saved position has left the live window
 * by the time the server is back.
 *
 * `onGiveUp` runs when the error is not recoverable or the attempts are spent, and
 * leaves a clean slate for whatever the player does next. iOS
 * ignores a seek made on `loadedmetadata` and needs it again on `canplaythrough` (see
 * start-time.ts); that handler is registered after start-time's, so the saved position
 * wins over the `?t=` one.
 */
export function recoverFromErrors(
  player: RecoverablePlayer,
  source: () => { src: string; type: string },
  onGiveUp: () => void,
  {
    delays = RECOVERY_DELAYS_MS,
    isIOS = false,
    live = false,
  }: { delays?: readonly number[]; isIOS?: boolean; live?: boolean } = {},
): Recovery {
  let attempt = 0;
  let timer: ReturnType<typeof setTimeout> | undefined;
  // Taken at the first error of a streak: the retries themselves reset the position.
  let resume: { time: number; playing: boolean } | undefined;
  // Listeners of the reload in progress.
  let pending: (() => void)[] = [];

  const clearPending = () => {
    for (const stop of pending) stop();
    pending = [];
  };
  const listen = (event: string, fn: () => void) => {
    player.on(event, fn);
    pending.push(() => player.off(event, fn));
  };

  const reload = () => {
    timer = undefined;
    clearPending();
    const { time, playing } = resume!;
    const recovered = () => {
      attempt = 0;
      resume = undefined;
      clearPending();
    };
    player.error(null);
    player.src(source());
    listen("loadedmetadata", () => {
      if (time > 0) player.currentTime(time);
      if (playing) void player.play()?.catch(() => {});
      listen(time > 0 ? "seeked" : "canplay", recovered);
    });
    if (isIOS && time > 0) {
      let repeated = false;
      listen("canplaythrough", () => {
        if (repeated) return;
        repeated = true;
        player.currentTime(time);
      });
    }
  };

  const reset = () => {
    clearTimeout(timer);
    timer = undefined;
    clearPending();
    attempt = 0;
    resume = undefined;
  };

  const onError = () => {
    if (timer !== undefined) return;
    const code = player.error()?.code;
    if ((code !== undefined && FATAL_CODES.has(code)) || attempt >= delays.length) {
      reset();
      onGiveUp();
      return;
    }
    resume ??= { time: live ? 0 : player.currentTime(), playing: !player.paused() };
    timer = setTimeout(reload, delays[attempt++]);
  };

  player.on("error", onError);
  return {
    reset,
    dispose() {
      player.off("error", onError);
      reset();
    },
  };
}
