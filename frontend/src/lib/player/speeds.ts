import type { PlayerLike } from "./types";

/** The speeds a viewer gets until they customise them (model/user.go's enabled defaults). */
export const DEFAULT_PLAYBACK_RATES: readonly number[] = [0.5, 0.75, 1, 1.25, 1.5, 1.75, 2];

/**
 * The speeds the menu offers. Safari misbehaves above 2x (issue #1222), so it only
 * gets the speeds up to 2x; `limitToTwo` is `videojs.browser.IS_SAFARI`.
 */
export function menuRates(rates: readonly number[], limitToTwo: boolean): number[] {
  return [...new Set(rates)].filter((r) => r > 0 && (r <= 2 || !limitToTwo)).sort((a, b) => a - b);
}

/**
 * The neighbouring entry of `rates` for the "faster"/"slower" hotkeys, or undefined at
 * the end of the list. A current rate that is not in the list (a `?rate=` of 1.1) steps
 * to the nearest entry in that direction instead of doing nothing.
 */
export function stepRate(rates: readonly number[], current: number, faster: boolean): number | undefined {
  const sorted = [...rates].sort((a, b) => a - b);
  return faster ? sorted.find((r) => r > current) : sorted.reverse().find((r) => r < current);
}

/**
 * Loading a source resets a media element's rate to its *default* rate, so a rate set
 * before the source loads (the stored or `?rate=` one is applied at creation) would
 * silently fall back to 1x. Keeping the default in step with the chosen rate makes it
 * survive that, and a later change of `src`. Returns the function that stops following.
 */
export function keepRateAcrossSources(player: PlayerLike & { defaultPlaybackRate?(rate: number): unknown }): () => void {
  const sync = () => player.defaultPlaybackRate?.(player.playbackRate());
  sync();
  player.on("ratechange", sync);
  return () => player.off("ratechange", sync);
}
