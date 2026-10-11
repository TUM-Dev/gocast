import { describe, expect, it } from "vitest";

import { highestLevel, levelsOf, preferHighestQuality, type QualityLevel, type QualityLevelListLike } from "./quality";

function list(...levels: Partial<QualityLevel>[]) {
  const handlers = new Set<() => void>();
  const out = {
    length: 0,
    on: (_: string, fn: () => void) => handlers.add(fn),
    off: (_: string, fn: () => void) => handlers.delete(fn),
    add(level: Partial<QualityLevel>) {
      (out as unknown as Record<number, QualityLevel>)[out.length++] = { id: String(out.length), bitrate: 0, enabled: true, ...level };
      handlers.forEach((fn) => fn());
    },
  };
  levels.forEach((l) => out.add(l));
  return out as typeof out & QualityLevelListLike;
}

describe("highestLevel", () => {
  it("prefers pixels, then bitrate", () => {
    const a = { id: "a", height: 720, width: 1280, bitrate: 3, enabled: true };
    const b = { id: "b", height: 1080, width: 1920, bitrate: 2, enabled: true };
    const c = { id: "c", height: 1080, width: 1920, bitrate: 5, enabled: true };
    expect(highestLevel([a, b])).toBe(b);
    expect(highestLevel([a, b, c])).toBe(c);
    expect(highestLevel([])).toBeUndefined();
  });
});

describe("preferHighestQuality", () => {
  it("enables only the best rendition, also as renditions arrive", () => {
    const l = list({ height: 360, width: 640 }, { height: 720, width: 1280 });
    const release = preferHighestQuality(l);
    expect(levelsOf(l).map((x) => x.enabled)).toEqual([false, true]);
    l.add({ height: 1080, width: 1920 });
    expect(levelsOf(l).map((x) => x.enabled)).toEqual([false, false, true]);
    release();
    expect(levelsOf(l).every((x) => x.enabled)).toBe(true);
    l.add({ height: 2160, width: 3840 });
    expect(levelsOf(l).map((x) => x.enabled)).toEqual([true, true, true, true]);
  });
});
