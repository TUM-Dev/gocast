import { describe, expect, it, vi } from "vitest";

import { fakePlayer } from "@/test/fake-player";

import { resetRateAtLiveEdge, seekToLive, type LiveTrackerLike } from "./live";

function tracker(atEdge: boolean, live = true) {
  const handlers = new Set<() => void>();
  const t = {
    on: (_: string, fn: () => void) => handlers.add(fn),
    off: (_: string, fn: () => void) => handlers.delete(fn),
    atLiveEdge: () => atEdge,
    isLive: () => live,
    seekToLiveEdge: vi.fn(),
    emit: () => handlers.forEach((fn) => fn()),
    setEdge: (v: boolean) => (atEdge = v),
  };
  return t satisfies LiveTrackerLike & Record<string, unknown>;
}

describe("resetRateAtLiveEdge", () => {
  it("goes back to 1x at the edge, only if sped up", () => {
    const { player, state } = fakePlayer();
    const t = tracker(false);
    resetRateAtLiveEdge(player, t);
    state.rate = 1.5;
    t.emit();
    expect(state.rate).toBe(1.5);
    t.setEdge(true);
    t.emit();
    expect(state.rate).toBe(1);
    state.rate = 0.5;
    t.emit();
    expect(state.rate).toBe(0.5);
  });

  it("stops listening when released", () => {
    const { player, state } = fakePlayer();
    const t = tracker(true);
    const stop = resetRateAtLiveEdge(player, t);
    stop();
    state.rate = 2;
    t.emit();
    expect(state.rate).toBe(2);
  });
});

describe("seekToLive", () => {
  it("seeks to the edge of a live stream only", () => {
    const live = tracker(false);
    seekToLive(live);
    expect(live.seekToLiveEdge).toHaveBeenCalledOnce();
    const vod = tracker(false, false);
    seekToLive(vod);
    expect(vod.seekToLiveEdge).not.toHaveBeenCalled();
    expect(() => seekToLive(undefined)).not.toThrow();
  });
});
