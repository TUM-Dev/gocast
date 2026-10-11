import type { PlayerLike } from "@/lib/player/types";
import { timeRanges } from "@/lib/player/types";

/**
 * A stand-in for a video.js player: the state the lib/player modules read and write,
 * and an event emitter. `state.time` is the media's own clock; clip.ts replaces
 * `currentTime` and friends on the instance, exactly as it does on a real player.
 */
export function fakePlayer(init: { duration?: number; rates?: number[] } = {}) {
  const handlers = new Map<string, Set<(...args: unknown[]) => void>>();
  const state = {
    time: 0,
    duration: init.duration ?? 100,
    paused: true,
    ended: false,
    volume: 1,
    muted: false,
    rate: 1,
    fullscreen: false,
    rates: init.rates ?? [0.5, 1, 1.5, 2],
    pauseCalls: 0,
  };
  const names = (e: string | string[]) => (Array.isArray(e) ? e : [e]);
  const el = document.createElement("div");

  const player = {
    el: () => el,
    on(event: string | string[], fn: (...args: unknown[]) => void) {
      for (const n of names(event)) {
        if (!handlers.has(n)) handlers.set(n, new Set());
        handlers.get(n)!.add(fn);
      }
    },
    off(event: string | string[], fn: (...args: unknown[]) => void) {
      for (const n of names(event)) handlers.get(n)?.delete(fn);
    },
    one(event: string, fn: (...args: unknown[]) => void) {
      const wrapped = (...args: unknown[]) => {
        player.off(event, wrapped);
        fn(...args);
      };
      player.on(event, wrapped);
    },
    trigger(event: string) {
      for (const fn of [...(handlers.get(event) ?? [])]) fn();
    },
    currentTime(to?: number) {
      if (to === undefined) return state.time;
      state.time = to;
    },
    duration(to?: number) {
      if (to === undefined) return state.duration;
      state.duration = to;
    },
    paused: () => state.paused,
    ended: () => state.ended,
    play() {
      state.paused = false;
      player.trigger("play");
      return Promise.resolve();
    },
    pause() {
      state.paused = true;
      state.pauseCalls++;
      player.trigger("pause");
    },
    volume(to?: number) {
      if (to === undefined) return state.volume;
      state.volume = to;
      player.trigger("volumechange");
    },
    muted(to?: boolean) {
      if (to === undefined) return state.muted;
      state.muted = to;
      player.trigger("volumechange");
    },
    playbackRate(to?: number) {
      if (to === undefined) return state.rate;
      state.rate = to;
      player.trigger("ratechange");
    },
    playbackRates: () => state.rates,
    seekable: () => timeRanges([0, state.duration]),
    buffered: () => timeRanges([0, state.duration / 2]),
    isFullscreen: () => state.fullscreen,
    requestFullscreen() {
      state.fullscreen = true;
    },
    exitFullscreen() {
      state.fullscreen = false;
    },
    handlerCount: () => [...handlers.values()].reduce((n, s) => n + s.size, 0),
  };
  return { player: player as unknown as PlayerLike & { handlerCount(): number }, state, el };
}
