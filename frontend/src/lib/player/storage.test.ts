import { beforeEach, describe, expect, it } from "vitest";

import { fakePlayer } from "@/test/fake-player";

import { applyStoredSettings, loadMuted, loadRate, loadVolume, persistSettings, resolveRate } from "./storage";

beforeEach(() => localStorage.clear());

describe("stored settings", () => {
  it("reads the old player's keys and formats", () => {
    localStorage.setItem("volume", "0.35");
    localStorage.setItem("muted", "true");
    localStorage.setItem("rate", "1.5");
    localStorage.setItem("live_rate", "1.25");
    expect(loadVolume()).toBe(0.35);
    expect(loadMuted()).toBe(true);
    expect(loadRate(false)).toBe(1.5);
    expect(loadRate(true)).toBe(1.25);
  });

  it("falls back for missing or unusable values", () => {
    expect(loadVolume()).toBeNull();
    expect(loadMuted()).toBeNull();
    expect(loadRate(false)).toBe(1);
    localStorage.setItem("rate", "0");
    localStorage.setItem("live_rate", "abc");
    localStorage.setItem("volume", "7");
    expect(loadRate(false)).toBe(1);
    expect(loadRate(true)).toBe(1);
    expect(loadVolume()).toBeNull();
  });

  it("lets the requested rate win over the stored one", () => {
    localStorage.setItem("rate", "1.5");
    expect(resolveRate(2, false)).toBe(2);
    expect(resolveRate(undefined, false)).toBe(1.5);
    expect(resolveRate(0, false)).toBe(1.5);
  });

  it("survives storage that throws", () => {
    const original = Object.getOwnPropertyDescriptor(globalThis, "localStorage")!;
    Object.defineProperty(globalThis, "localStorage", {
      configurable: true,
      get() {
        throw new Error("blocked");
      },
    });
    try {
      expect(loadRate(false)).toBe(1);
      const { player } = fakePlayer();
      persistSettings(player, { live: false });
      expect(() => player.volume(0.5)).not.toThrow();
    } finally {
      Object.defineProperty(globalThis, "localStorage", original);
    }
  });
});

describe("applyStoredSettings", () => {
  it("applies volume, mute and rate", () => {
    localStorage.setItem("volume", "0.2");
    localStorage.setItem("muted", "false");
    localStorage.setItem("live_rate", "1.25");
    const { player, state } = fakePlayer();
    applyStoredSettings(player, { live: true });
    expect([state.volume, state.muted, state.rate]).toEqual([0.2, false, 1.25]);
  });

  it("prefers the page's rate", () => {
    localStorage.setItem("rate", "1.5");
    const { player, state } = fakePlayer();
    applyStoredSettings(player, { live: false, rate: 2 });
    expect(state.rate).toBe(2);
  });
});

describe("persistSettings", () => {
  it("writes volume, mute and the rate of the right kind", () => {
    const vod = fakePlayer();
    persistSettings(vod.player, { live: false });
    vod.player.volume(0.4);
    vod.player.muted(true);
    vod.player.playbackRate(1.75);
    expect(localStorage.getItem("volume")).toBe("0.4");
    expect(localStorage.getItem("muted")).toBe("true");
    expect(localStorage.getItem("rate")).toBe("1.75");
    expect(localStorage.getItem("live_rate")).toBeNull();

    const live = fakePlayer();
    persistSettings(live.player, { live: true });
    live.player.playbackRate(1.25);
    expect(localStorage.getItem("live_rate")).toBe("1.25");
    expect(localStorage.getItem("rate")).toBe("1.75");
  });

  it("can keep a temporary rate out and stops when asked", () => {
    const { player } = fakePlayer();
    let hold = true;
    const stop = persistSettings(player, { live: false, ignoreRate: () => hold });
    player.playbackRate(2);
    expect(localStorage.getItem("rate")).toBeNull();
    hold = false;
    player.playbackRate(1.5);
    expect(localStorage.getItem("rate")).toBe("1.5");
    stop();
    player.playbackRate(1);
    expect(localStorage.getItem("rate")).toBe("1.5");
  });
});
