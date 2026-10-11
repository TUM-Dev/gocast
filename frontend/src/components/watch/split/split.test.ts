import { beforeEach, describe, expect, it, vi } from "vitest";

import type { PlayerController, PlayerEvent } from "@/lib/player/controller";
import { deferredController } from "./controller";
import { DEFAULT_RATIO, RATIO_KEY, clampRatio, loadRatio, presetFor, saveRatio } from "./ratio";
import { CORRECTION_INTERVAL_MS, syncPlayers } from "./sync";

function fakeController(init: { time?: number; paused?: boolean; rate?: number } = {}) {
  const handlers = new Map<PlayerEvent, Set<() => void>>();
  const state = { time: init.time ?? 0, paused: init.paused ?? true, rate: init.rate ?? 1, muted: false };
  const api = {
    play: vi.fn(async () => {
      state.paused = false;
    }),
    pause: vi.fn(() => {
      state.paused = true;
    }),
    seek: vi.fn((t: number) => {
      state.time = t;
    }),
    getCurrentTime: () => state.time,
    isPaused: () => state.paused,
    getRate: () => state.rate,
    setRate: vi.fn((r: number) => {
      state.rate = r;
    }),
    setMuted: vi.fn((m: boolean) => {
      state.muted = m;
    }),
    on(event: PlayerEvent, fn: () => void) {
      if (!handlers.has(event)) handlers.set(event, new Set());
      handlers.get(event)!.add(fn);
      return () => handlers.get(event)!.delete(fn);
    },
  };
  const fire = (e: PlayerEvent) => handlers.get(e)?.forEach((fn) => fn());
  return { api: api as unknown as PlayerController, raw: api, state, fire, count: () => [...handlers.values()].reduce((n, s) => n + s.size, 0) };
}

describe("syncPlayers", () => {
  it("makes the follower copy play, pause, rate and seeks", () => {
    const a = fakeController();
    const b = fakeController();
    syncPlayers(a.api, b.api);
    a.fire("play");
    expect(b.raw.play).toHaveBeenCalled();
    a.state.rate = 1.5;
    a.fire("ratechange");
    expect(b.state.rate).toBe(1.5);
    a.state.time = 42;
    a.fire("seeked");
    expect(b.state.time).toBe(42);
    a.fire("pause");
    expect(b.raw.pause).toHaveBeenCalled();
  });

  it("applies the follower's setup once it has metadata: muted, aligned, playing if the primary is", () => {
    const a = fakeController({ time: 10, paused: false, rate: 2 });
    const b = fakeController();
    syncPlayers(a.api, b.api);
    b.fire("loadedmetadata");
    expect(b.state).toMatchObject({ muted: true, time: 10, rate: 2 });
    expect(b.raw.play).toHaveBeenCalled();
  });

  it("pauses both when the follower may not play", async () => {
    const a = fakeController();
    const b = fakeController();
    b.raw.play.mockRejectedValue(new Error("blocked"));
    syncPlayers(a.api, b.api);
    a.fire("play");
    await Promise.resolve();
    await Promise.resolve();
    expect(a.raw.pause).toHaveBeenCalled();
    expect(b.raw.pause).toHaveBeenCalled();
  });

  it("holds the follower while the primary stalls and resumes it after", () => {
    const a = fakeController({ paused: false });
    const b = fakeController({ paused: false });
    syncPlayers(a.api, b.api);
    a.fire("waiting");
    expect(b.state.paused).toBe(true);
    a.fire("playing");
    expect(b.raw.play).toHaveBeenCalled();
  });

  it("corrects drift, but not more often than the interval, and not while paused", () => {
    let clock = 10_000;
    const a = fakeController({ time: 100, paused: false });
    const b = fakeController({ time: 90, paused: false });
    syncPlayers(a.api, b.api, { now: () => clock });
    a.fire("timeupdate");
    expect(b.state.time).toBe(100);
    b.state.time = 80;
    clock += CORRECTION_INTERVAL_MS - 1;
    a.fire("timeupdate");
    expect(b.state.time).toBe(80);
    clock += 2;
    a.fire("timeupdate");
    expect(b.state.time).toBe(100);
    a.state.paused = true;
    b.state.time = 0;
    clock += 10_000;
    a.fire("timeupdate");
    expect(b.state.time).toBe(0);
  });

  it("ignores drift below the threshold and stops when disposed", () => {
    const a = fakeController({ time: 100, paused: false });
    const b = fakeController({ time: 99.5, paused: false });
    const stop = syncPlayers(a.api, b.api, { now: () => 1e9 });
    a.fire("timeupdate");
    expect(b.raw.seek).not.toHaveBeenCalled();
    stop();
    expect(a.count() + b.count()).toBe(0);
  });
});

describe("deferredController", () => {
  it("answers like an empty controller until there is a target, then forwards", () => {
    let target: PlayerController | null = null;
    const { api, retarget } = deferredController(() => target);
    expect(api.isPaused()).toBe(true);
    expect(api.getRate()).toBe(1);
    const t = fakeController();
    const fn = vi.fn();
    const off = api.on("pause", fn);
    target = t.api;
    retarget();
    t.fire("pause");
    expect(fn).toHaveBeenCalledOnce();
    api.seek(5);
    expect(t.state.time).toBe(5);
    off();
    t.fire("pause");
    expect(fn).toHaveBeenCalledOnce();
  });
});

describe("ratio", () => {
  beforeEach(() => localStorage.clear());
  it("clamps and persists", () => {
    expect(clampRatio(140)).toBe(100);
    expect(clampRatio(-3)).toBe(0);
    expect(clampRatio(NaN)).toBe(DEFAULT_RATIO);
    expect(loadRatio()).toBe(DEFAULT_RATIO);
    saveRatio(75);
    expect(localStorage.getItem(RATIO_KEY)).toBe("75");
    expect(loadRatio()).toBe(75);
    expect(presetFor(75)?.label).toBe("Focus camera");
    expect(presetFor(60)).toBeUndefined();
  });
  it("survives unusable storage", () => {
    const spy = vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("blocked");
    });
    expect(loadRatio()).toBe(DEFAULT_RATIO);
    spy.mockRestore();
  });
});
