import type { LiveTrackerLike } from "./live";
import { seekToLive } from "./live";
import type { QualityLevel, QualityLevelListLike, QualityPreference } from "./quality";
import { levelsOf, preferHighestQuality } from "./quality";
import type { PlayerLike } from "./types";

/** The player events a parent can subscribe to. All are plain notifications. */
export const PLAYER_EVENTS = [
  "play",
  "pause",
  "ended",
  "seeking",
  "seeked",
  "waiting",
  "playing",
  "ratechange",
  "volumechange",
  "timeupdate",
  "loadedmetadata",
  "durationchange",
] as const;
export type PlayerEvent = (typeof PLAYER_EVENTS)[number];

/** What a parent that coordinates players (the split view) drives. Times are clip time. */
export interface PlayerController {
  play(): Promise<void>;
  pause(): void;
  togglePlay(): void;
  /** Seconds, on the clip's clock if there is a clip. */
  seek(seconds: number): void;
  seekBy(delta: number): void;
  seekToLive(): void;
  getCurrentTime(): number;
  getDuration(): number;
  isPaused(): boolean;
  getRate(): number;
  setRate(rate: number): void;
  getVolume(): number;
  setVolume(volume: number): void;
  isMuted(): boolean;
  setMuted(muted: boolean): void;
  getQualityLevels(): QualityLevel[];
  setQualityPreference(preference: QualityPreference): void;
  /** Subscribes; works before the player exists. Returns the unsubscribe function. */
  on(event: PlayerEvent, fn: () => void): () => void;
}

/**
 * The imperative surface of a player. It exists apart from the component because it
 * outlives the video.js instance: video.js loads on demand, so a parent can subscribe
 * before a player exists, and a changed source or clip rebuilds pieces under it.
 * Commands before the player exists do nothing.
 */
export function createController() {
  let player: PlayerLike | null = null;
  let levels: QualityLevelListLike | null = null;
  let tracker: LiveTrackerLike | undefined;
  let release: (() => void) | null = null;
  const listeners = new Map<PlayerEvent, Set<() => void>>();
  const dispatchers = new Map<PlayerEvent, () => void>();

  const dispatcher = (event: PlayerEvent) => () => listeners.get(event)?.forEach((fn) => fn());

  const api: PlayerController = {
    play: async () => {
      await player?.play();
    },
    pause: () => player?.pause(),
    togglePlay() {
      if (!player) return;
      if (player.paused()) void player.play();
      else player.pause();
    },
    seek: (seconds) => player?.currentTime(Math.max(0, seconds)),
    seekBy(delta) {
      if (!player) return;
      const duration = player.duration();
      const target = Math.max(0, player.currentTime() + delta);
      player.currentTime(Number.isFinite(duration) ? Math.min(target, duration) : target);
    },
    seekToLive: () => seekToLive(tracker),
    getCurrentTime: () => player?.currentTime() ?? 0,
    getDuration: () => player?.duration() ?? NaN,
    isPaused: () => player?.paused() ?? true,
    getRate: () => player?.playbackRate() ?? 1,
    setRate: (rate) => player?.playbackRate(rate),
    getVolume: () => player?.volume() ?? 1,
    setVolume: (volume) => player?.volume(Math.min(1, Math.max(0, volume))),
    isMuted: () => player?.muted() ?? false,
    setMuted: (muted) => player?.muted(muted),
    getQualityLevels: () => (levels ? levelsOf(levels) : []),
    setQualityPreference(preference) {
      release?.();
      release = null;
      if (levels && preference === "highest") release = preferHighestQuality(levels);
    },
    on(event, fn) {
      let set = listeners.get(event);
      if (!set) listeners.set(event, (set = new Set()));
      set.add(fn);
      return () => set.delete(fn);
    },
  };

  return {
    api,
    attach(p: PlayerLike, parts: { qualityLevels?: QualityLevelListLike; liveTracker?: LiveTrackerLike }) {
      player = p;
      levels = parts.qualityLevels ?? null;
      tracker = parts.liveTracker;
      for (const event of PLAYER_EVENTS) {
        const fn = dispatcher(event);
        dispatchers.set(event, fn);
        p.on(event, fn);
      }
    },
    detach() {
      release?.();
      release = null;
      if (player) for (const [event, fn] of dispatchers) player.off(event, fn);
      dispatchers.clear();
      player = null;
      levels = null;
      tracker = undefined;
    },
  };
}
