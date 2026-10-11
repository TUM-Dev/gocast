import type { PlayerController, PlayerEvent } from "@/lib/player/controller";

type Target = () => PlayerController | null | undefined;

const COMMANDS = [
  "play",
  "pause",
  "togglePlay",
  "seek",
  "seekBy",
  "seekToLive",
  "getCurrentTime",
  "getDuration",
  "isPaused",
  "getRate",
  "setRate",
  "getVolume",
  "setVolume",
  "isMuted",
  "setMuted",
  "getQualityLevels",
  "setQualityPreference",
] as const;

const FALLBACKS: Partial<Record<(typeof COMMANDS)[number], unknown>> = {
  play: Promise.resolve(),
  getCurrentTime: 0,
  getDuration: NaN,
  isPaused: true,
  getRate: 1,
  getVolume: 1,
  isMuted: false,
  getQualityLevels: [],
};

/**
 * A {@link PlayerController} for a player that is not there yet (or ever, or not any more):
 * commands go to whatever `target` returns when they are called, and do what a
 * controller does before its player exists when there is none. `on` is the exception to
 * "when called": a subscription made early is kept and attached once a target appears,
 * which is what lets a page subscribe in setup, before the split view has mounted.
 *
 * Call `retarget()` when the target becomes available or is replaced.
 */
export function deferredController(target: Target): { api: PlayerController; retarget(): void } {
  interface Sub {
    event: PlayerEvent;
    fn: () => void;
    off: (() => void) | null;
  }
  const subs = new Set<Sub>();

  const attach = (sub: Sub) => {
    sub.off?.();
    sub.off = target()?.on(sub.event, sub.fn) ?? null;
  };

  const api = {} as Record<string, unknown>;
  for (const name of COMMANDS) {
    api[name] = (...args: unknown[]) => {
      const t = target();
      if (!t) return FALLBACKS[name];
      return (t[name] as (...a: unknown[]) => unknown)(...args);
    };
  }
  api.on = (event: PlayerEvent, fn: () => void) => {
    const sub: Sub = { event, fn, off: null };
    subs.add(sub);
    attach(sub);
    return () => {
      sub.off?.();
      subs.delete(sub);
    };
  };

  return {
    api: api as unknown as PlayerController,
    retarget: () => subs.forEach(attach),
  };
}
