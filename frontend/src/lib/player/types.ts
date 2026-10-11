/**
 * The slice of a video.js player the modules in this directory use. They take this
 * instead of video.js' own `Player` type so that they can be tested against a fake and
 * so that none of them imports video.js (which is loaded on demand, see VideoPlayer.vue).
 * VideoPlayer.vue casts the real player to it once, at the composition boundary.
 */
export interface PlayerLike {
  el(): Element;
  on(event: string | string[], fn: (...args: unknown[]) => void): void;
  off(event: string | string[], fn: (...args: unknown[]) => void): void;
  one(event: string, fn: (...args: unknown[]) => void): void;
  trigger(event: string): void;
  currentTime(): number;
  currentTime(seconds: number): void;
  duration(): number;
  /** video.js' setter for the length the tech reported; it fires `durationchange`. */
  duration(seconds: number): void;
  paused(): boolean;
  ended(): boolean;
  play(): Promise<void> | undefined;
  pause(): void;
  volume(): number;
  volume(level: number): void;
  muted(): boolean;
  muted(muted: boolean): void;
  playbackRate(): number;
  playbackRate(rate: number): void;
  playbackRates(): number[];
  seekable(): TimeRanges;
  buffered(): TimeRanges;
  isFullscreen(): boolean;
  requestFullscreen(): unknown;
  exitFullscreen(): unknown;
}

/** A seconds-based time range, so clip.ts and its tests need no real `TimeRanges`. */
export function timeRanges(...ranges: [number, number][]): TimeRanges {
  return {
    length: ranges.length,
    start: (i: number) => ranges[i][0],
    end: (i: number) => ranges[i][1],
  } as TimeRanges;
}
