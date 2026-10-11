import type { PlayerController } from "@/lib/player/controller";

/** Seconds of disagreement the viewer cannot see; beyond it the follower is pulled back. */
export const DRIFT_SECONDS = 1;
/** The fewest milliseconds between two corrections, so a slow follower is not seeked in a loop. */
export const CORRECTION_INTERVAL_MS = 2000;

export interface SyncOptions {
  now?: () => number;
}

/**
 * Keeps `follower` on `primary`: play, pause, speed and position follow the primary,
 * the follower is muted (one soundtrack is enough), and a follower that drifts away
 * while playing is seeked back. Port of web/ts/TUMLiveVjs.ts' syncPlayers, with the
 * difference that only the primary drives -- the follower has no controls -- and that a
 * seek is carried over at once instead of at most every two seconds, which dropped a
 * second seek made soon after the first.
 *
 * Works before either player exists: the controllers queue subscriptions, and what the
 * follower needs once it has a player (muted, position, speed) is applied when it
 * reports its metadata. Returns the function that stops syncing.
 */
export function syncPlayers(primary: PlayerController, follower: PlayerController, options: SyncOptions = {}): () => void {
  const now = options.now ?? Date.now;
  let lastCorrection = -Infinity;
  const off: (() => void)[] = [];

  const align = () => {
    follower.setMuted(true);
    follower.setRate(primary.getRate());
    follower.seek(primary.getCurrentTime());
    lastCorrection = now();
  };

  off.push(
    follower.on("loadedmetadata", () => {
      align();
      if (!primary.isPaused()) void follower.play().catch(() => {});
    }),
  );

  off.push(
    primary.on("play", () => {
      follower.play().catch(() => {
        // Autoplay refused for the follower: better two paused pictures than two that disagree.
        primary.pause();
        follower.pause();
      });
    }),
  );
  off.push(primary.on("pause", () => follower.pause()));
  off.push(primary.on("ended", () => follower.pause()));
  off.push(primary.on("ratechange", () => follower.setRate(primary.getRate())));
  off.push(
    primary.on("seeked", () => {
      follower.seek(primary.getCurrentTime());
      lastCorrection = now();
    }),
  );
  // A stalled primary holds the follower, or it would run ahead of the picture.
  off.push(primary.on("waiting", () => follower.pause()));
  off.push(
    primary.on("playing", () => {
      if (follower.isPaused()) void follower.play().catch(() => {});
    }),
  );
  off.push(
    primary.on("timeupdate", () => {
      if (primary.isPaused()) return;
      const t = now();
      if (t - lastCorrection < CORRECTION_INTERVAL_MS) return;
      if (Math.abs(follower.getCurrentTime() - primary.getCurrentTime()) > DRIFT_SECONDS) {
        follower.seek(primary.getCurrentTime());
        lastCorrection = t;
      }
    }),
  );

  return () => off.forEach((fn) => fn());
}
