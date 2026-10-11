import type { PlayerLike } from "./types";

/**
 * The old player (web/ts/TUMLiveVjs.ts) kept the viewer's volume, mute and playback rate
 * in localStorage under these keys. The SPA player reads and writes the very same keys
 * and string formats, so a viewer's settings follow them across both frontends.
 * Live streams have a rate of their own, because a viewer who watches recordings at 2x
 * does not want the live edge at 2x.
 */
export const STORAGE_KEYS = { volume: "volume", muted: "muted", rate: "rate", liveRate: "live_rate" } as const;

// localStorage throws in private windows and with blocked site data; a player that
// cannot remember a setting must still play.
function read(key: string): string | null {
  try {
    return window.localStorage.getItem(key);
  } catch {
    return null;
  }
}

function write(key: string, value: string): void {
  try {
    window.localStorage.setItem(key, value);
  } catch {
    // Nothing to do: the setting just does not persist.
  }
}

export function loadVolume(): number | null {
  const stored = read(STORAGE_KEYS.volume);
  if (stored === null) return null;
  const volume = Number(stored);
  return Number.isFinite(volume) && volume >= 0 && volume <= 1 ? volume : null;
}

export function loadMuted(): boolean | null {
  const stored = read(STORAGE_KEYS.muted);
  return stored === null ? null : stored === "true";
}

/** The stored rate; anything unusable (missing, zero, negative, NaN) is 1x, as before. */
export function loadRate(live: boolean): number {
  const stored = read(live ? STORAGE_KEYS.liveRate : STORAGE_KEYS.rate);
  const rate = stored === null ? 1 : Number(stored);
  return Number.isFinite(rate) && rate > 0 ? rate : 1;
}

/** A rate from the page (the `?rate=` parameter) wins over the stored one, as before. */
export function resolveRate(requested: number | undefined, live: boolean): number {
  return requested !== undefined && Number.isFinite(requested) && requested > 0 ? requested : loadRate(live);
}

/** Applies the stored volume and mute state, and the rate (see {@link resolveRate}). */
export function applyStoredSettings(player: PlayerLike, opts: { live: boolean; rate?: number }): void {
  const volume = loadVolume();
  if (volume !== null) player.volume(volume);
  const muted = loadMuted();
  if (muted !== null) player.muted(muted);
  player.playbackRate(resolveRate(opts.rate, opts.live));
}

/**
 * Writes volume, mute and rate back whenever they change. `ignoreRate` lets hold-to-2x
 * keep its temporary rate out of storage. Returns the function that stops listening.
 */
export function persistSettings(
  player: PlayerLike,
  opts: { live: boolean; ignoreRate?: () => boolean },
): () => void {
  const onVolume = () => {
    write(STORAGE_KEYS.volume, String(player.volume()));
    write(STORAGE_KEYS.muted, String(player.muted()));
  };
  const onRate = () => {
    if (opts.ignoreRate?.()) return;
    write(opts.live ? STORAGE_KEYS.liveRate : STORAGE_KEYS.rate, String(player.playbackRate()));
  };
  player.on("volumechange", onVolume);
  player.on("ratechange", onRate);
  return () => {
    player.off("volumechange", onVolume);
    player.off("ratechange", onRate);
  };
}
