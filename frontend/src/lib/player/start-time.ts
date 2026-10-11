import type { PlayerLike } from "./types";

/**
 * Starts playback at `seconds` (the `?t=` parameter), once the player knows the media.
 * Returns the function that stops listening.
 *
 * iOS ignores a seek made on `loadedmetadata`: it needs it again when it can actually
 * play (https://stackoverflow.com/questions/28823567), so on iOS the seek is repeated
 * once on `canplaythrough`. A new source (a changed `src`) is seeked again, which is
 * why this is not `one()`.
 */
export function applyStartTime(player: PlayerLike, seconds: number, isIOS: boolean): () => void {
  let repeated = false;
  const seek = () => {
    repeated = false;
    player.currentTime(seconds);
  };
  const repeat = () => {
    // Can fire many times during playback.
    if (repeated) return;
    repeated = true;
    player.currentTime(seconds);
  };
  player.on("loadedmetadata", seek);
  if (isIOS) player.on("canplaythrough", repeat);
  return () => {
    player.off("loadedmetadata", seek);
    player.off("canplaythrough", repeat);
  };
}

/** The `?t=` / `?rate=` value, or undefined if it is missing or not a usable number. */
export function parseNumberParam(value: unknown): number | undefined {
  const text = Array.isArray(value) ? value[0] : value;
  if (typeof text !== "string" || text.trim() === "") return undefined;
  const n = Number(text);
  return Number.isFinite(n) && n >= 0 ? n : undefined;
}
