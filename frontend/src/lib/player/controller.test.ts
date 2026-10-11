import { describe, expect, it, vi } from "vitest";

import { fakePlayer } from "@/test/fake-player";

import { createController } from "./controller";
import type { QualityLevel, QualityLevelListLike } from "./quality";

function attached(parts = {}) {
  const f = fakePlayer({ duration: 100 });
  const c = createController();
  c.attach(f.player, parts);
  return { ...f, c, api: c.api };
}

describe("createController", () => {
  it("does nothing, and says so plainly, before a player exists", async () => {
    const { api } = createController();
    await api.play();
    api.seek(5);
    api.setRate(2);
    expect(api.getCurrentTime()).toBe(0);
    expect(api.isPaused()).toBe(true);
    expect(api.getRate()).toBe(1);
    expect(api.getQualityLevels()).toEqual([]);
  });

  it("drives play, pause, seek and rate", async () => {
    const { api, state } = attached();
    await api.play();
    expect(state.paused).toBe(false);
    api.togglePlay();
    expect(state.paused).toBe(true);
    api.seek(30);
    expect(api.getCurrentTime()).toBe(30);
    api.seekBy(-50);
    expect(state.time).toBe(0);
    api.seekBy(500);
    expect(state.time).toBe(100);
    api.setRate(1.5);
    expect(api.getRate()).toBe(1.5);
    api.setVolume(3);
    expect(state.volume).toBe(1);
    api.setMuted(true);
    expect(api.isMuted()).toBe(true);
    expect(api.getDuration()).toBe(100);
  });

  it("subscribes before the player exists and unsubscribes", () => {
    const f = fakePlayer();
    const c = createController();
    const fn = vi.fn();
    const off = c.api.on("seeked", fn);
    c.attach(f.player, {});
    f.player.trigger("seeked");
    expect(fn).toHaveBeenCalledOnce();
    off();
    f.player.trigger("seeked");
    expect(fn).toHaveBeenCalledOnce();
  });

  it("stops listening to the player when detached", () => {
    const { player, c, api } = attached();
    const fn = vi.fn();
    api.on("pause", fn);
    c.detach();
    player.pause();
    expect(fn).not.toHaveBeenCalled();
    expect(player.handlerCount()).toBe(0);
  });

  it("seeks to the live edge through the tracker", () => {
    const tracker = { on: vi.fn(), off: vi.fn(), atLiveEdge: () => false, isLive: () => true, seekToLiveEdge: vi.fn() };
    const { api } = attached({ liveTracker: tracker });
    api.seekToLive();
    expect(tracker.seekToLiveEdge).toHaveBeenCalledOnce();
  });

  it("lists quality levels and pins the highest on request", () => {
    const levels: QualityLevel[] = [
      { id: "a", height: 360, width: 640, bitrate: 1, enabled: true },
      { id: "b", height: 720, width: 1280, bitrate: 2, enabled: true },
    ];
    const list = Object.assign(levels, { on: vi.fn(), off: vi.fn() }) as unknown as QualityLevelListLike;
    const { api } = attached({ qualityLevels: list });
    expect(api.getQualityLevels()).toHaveLength(2);
    api.setQualityPreference("highest");
    expect(levels.map((l) => l.enabled)).toEqual([false, true]);
    api.setQualityPreference("auto");
    expect(levels.map((l) => l.enabled)).toEqual([true, true]);
  });
});
