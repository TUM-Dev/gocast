/** One rendition of an adaptive stream, as video.js' quality-levels list reports it. */
export interface QualityLevel {
  id: string;
  width?: number;
  height?: number;
  bitrate: number;
  enabled: boolean;
}

export interface QualityLevelListLike {
  length: number;
  [index: number]: QualityLevel;
  on(event: string, fn: () => void): void;
  off(event: string, fn: () => void): void;
}

export type QualityPreference = "auto" | "highest";

/** The rendition with the most pixels, then the most bits: "highest" for a viewer. */
export function highestLevel(levels: readonly QualityLevel[]): QualityLevel | undefined {
  const pixels = (l: QualityLevel) => (l.height ?? 0) * (l.width ?? 0);
  return levels.reduce<QualityLevel | undefined>(
    (best, l) => (!best || pixels(l) > pixels(best) || (pixels(l) === pixels(best) && l.bitrate > best.bitrate) ? l : best),
    undefined,
  );
}

export function levelsOf(list: QualityLevelListLike): QualityLevel[] {
  return Array.from({ length: list.length }, (_, i) => list[i]);
}

/**
 * Pins playback to the highest rendition (the player's own bandwidth-based choice is
 * "auto", the default) and returns the function that releases it. Renditions are added
 * one by one as the playlist is parsed, so the choice is redone on each addition.
 * There is deliberately no menu for this: the old player had none.
 */
export function preferHighestQuality(list: QualityLevelListLike): () => void {
  const apply = () => {
    const levels = levelsOf(list);
    const best = highestLevel(levels);
    if (!best) return;
    // VHS only picks among the enabled renditions, so enabling just one pins it.
    for (const level of levels) level.enabled = level === best;
  };
  list.on("addqualitylevel", apply);
  apply();
  return () => {
    list.off("addqualitylevel", apply);
    for (const level of levelsOf(list)) level.enabled = true;
  };
}
