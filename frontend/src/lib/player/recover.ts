/** The slice of a video.js player that recovery uses. */
export interface RecoverablePlayer {
  on(event: string, fn: () => void): void;
  off(event: string, fn: () => void): void;
  one(event: string, fn: () => void): void;
  error(): { code: number } | null;
  /** Clears the error, which is what lets the next `src` start from a clean state. */
  error(value: null): void;
  src(source: { src: string; type: string }): void;
  currentTime(): number;
  currentTime(seconds: number): void;
  paused(): boolean;
  play(): Promise<void> | undefined;
}

// MediaError codes that a reload cannot fix: the viewer aborted, the media is corrupt,
// or it is encrypted. Everything else (network, "no playable source" and VHS' custom
// errors) is what a server that is briefly gone looks like.
const FATAL_CODES = new Set([1, 3, 5]);

/** Seconds of waiting per attempt: about half a minute in all, to outlast a redeploy. */
export const RECOVERY_DELAYS_MS = [1000, 2000, 4000, 8000, 8000, 8000];

/**
 * Reloads the source after a playback error, at the position and play state the viewer
 * left, so that a restart of the video server shows a stall instead of an error. VHS
 * retries a failed segment on its own only while it has a playlist to fall back to; a
 * failure while the server is down for the first playlist, or for the last rendition, is
 * fatal and leaves a dead player behind.
 *
 * `onGiveUp` runs when the error is not recoverable or the attempts are spent. Returns
 * the function that stops listening.
 */
export function recoverFromErrors(
  player: RecoverablePlayer,
  source: () => { src: string; type: string },
  onGiveUp: () => void,
  delays: readonly number[] = RECOVERY_DELAYS_MS,
): () => void {
  let attempt = 0;
  let timer: ReturnType<typeof setTimeout> | undefined;
  // Taken at the first error of a streak: the retries themselves reset the position.
  let resume: { time: number; playing: boolean } | undefined;

  const reload = () => {
    timer = undefined;
    const { time, playing } = resume!;
    player.error(null);
    player.src(source());
    player.one("loadedmetadata", () => {
      attempt = 0;
      resume = undefined;
      if (time > 0) player.currentTime(time);
      if (playing) void player.play()?.catch(() => {});
    });
  };

  const onError = () => {
    const code = player.error()?.code;
    if (timer !== undefined) return;
    if ((code !== undefined && FATAL_CODES.has(code)) || attempt >= delays.length) {
      onGiveUp();
      return;
    }
    resume ??= { time: player.currentTime(), playing: !player.paused() };
    timer = setTimeout(reload, delays[attempt++]);
  };

  player.on("error", onError);
  return () => {
    player.off("error", onError);
    clearTimeout(timer);
  };
}
