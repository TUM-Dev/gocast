import { beforeEach, describe, expect, it, vi } from "vitest";

import type { PlayerController, PlayerEvent } from "@/lib/player/controller";
import { deferredController } from "./controller";
import { DEFAULT_RATIO, RATIO_KEY, clampRatio, loadRatio, presetFor, saveRatio } from "./ratio";
import { CORRECTION_INTERVAL_MS, syncPlayers } from "./sync";

function fakeController(init: { time?: number; paused?: boolean; rate?: number } = {}) {
  const handlers = new Map<PlayerEvent, Set<() => void>>();
  const state = { time: init.time ?? 0, paused: init.paused ?? true, rate: init.rate ?? 1, muted: false };
  const api = {
    isMuted: () => state.muted,
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

  it("does not pause the primary when a pause interrupted the follower's play (a seek that stalls)", async () => {
    const a = fakeController({ paused: false });
    const b = fakeController();
    const aborted = new Error("The play() request was interrupted by a call to pause()");
    aborted.name = "AbortError";
    b.raw.play.mockRejectedValue(aborted);
    syncPlayers(a.api, b.api);
    a.fire("play");
    await Promise.resolve();
    await Promise.resolve();
    expect(a.raw.pause).not.toHaveBeenCalled();
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

describe("seeking the pair", () => {
  it("carries every seek of the primary to the follower, including quick successive ones", () => {
    let clock = 0;
    const a = fakeController({ time: 5, paused: false });
    const b = fakeController({ time: 5, paused: false });
    syncPlayers(a.api, b.api, { now: () => clock });
    // The seekbar: a click, then a drag that ends moments later.
    for (const t of [30, 31, 80]) {
      a.state.time = t;
      a.fire("seeked");
      clock += 10;
      expect(b.state.time).toBe(t);
    }
  });

  it("follows a seek made through the controller (hotkeys, seek()) the same way", () => {
    const a = fakeController({ time: 0, paused: true });
    const b = fakeController({ time: 0, paused: true });
    syncPlayers(a.api, b.api);
    // seek() moves the primary; the player then reports it.
    a.api.seek(120);
    a.fire("seeked");
    expect(b.raw.seek).toHaveBeenLastCalledWith(120);
    expect(b.state.paused).toBe(true);
  });

  it("keeps a playing follower playing across a seek, holding it only while the primary buffers", () => {
    const a = fakeController({ time: 0, paused: false });
    const b = fakeController({ time: 0, paused: false });
    syncPlayers(a.api, b.api);
    a.state.time = 50;
    a.fire("seeking");
    a.fire("waiting");
    expect(b.state.paused).toBe(true);
    a.fire("seeked");
    a.fire("playing");
    expect(b.state.time).toBe(50);
    expect(b.state.paused).toBe(false);
  });

  it("passes clip time through unchanged: both players have the same clip", () => {
    // The clip's clock starts at 0 on both; the follower is seeked on that clock too.
    const a = fakeController({ time: 0 });
    const b = fakeController({ time: 0 });
    syncPlayers(a.api, b.api);
    a.state.time = 12.5;
    a.fire("seeked");
    expect(b.raw.seek).toHaveBeenLastCalledWith(12.5);
  });

  it("pulls a follower that fell behind a seek back even right after the last correction", () => {
    let clock = 1000;
    const a = fakeController({ time: 10, paused: false });
    const b = fakeController({ time: 10, paused: false });
    syncPlayers(a.api, b.api, { now: () => clock });
    a.state.time = 60;
    a.fire("seeked");
    b.state.time = 10; // the follower's seek was lost (it had no metadata yet)
    clock += CORRECTION_INTERVAL_MS + 1;
    a.fire("timeupdate");
    expect(b.state.time).toBe(60);
  });
});

describe("the follower's sound", () => {
  it("is muted once it has metadata and stays muted when something unmutes it", () => {
    const a = fakeController();
    const b = fakeController();
    syncPlayers(a.api, b.api);
    b.fire("loadedmetadata");
    expect(b.state.muted).toBe(true);
    b.state.muted = false; // a stored setting, a source reload
    b.fire("volumechange");
    expect(b.state.muted).toBe(true);
  });

  it("is untouched by the primary's volume changes", () => {
    const a = fakeController();
    const b = fakeController();
    syncPlayers(a.api, b.api);
    b.fire("loadedmetadata");
    a.state.muted = false;
    a.fire("volumechange");
    expect(b.state.muted).toBe(true);
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
