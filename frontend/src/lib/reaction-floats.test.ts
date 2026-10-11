import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { announceOwnReaction, createReactionFloats, FLOAT_MS, MAX_FLOATS, onOwnReaction } from "./reaction-floats";

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

describe("createReactionFloats", () => {
  it("shows a received emoji and removes it when its animation is over", () => {
    const f = createReactionFloats(() => 0.5);
    f.receive("👍");
    expect(f.floats.value.map((x) => x.emoji)).toEqual(["👍"]);
    expect(f.floats.value[0].left).toBe(50);
    vi.advanceTimersByTime(FLOAT_MS);
    expect(f.floats.value).toEqual([]);
  });

  it("keeps the floats inside the picture", () => {
    for (const r of [0, 0.999999]) {
      const f = createReactionFloats(() => r);
      f.receive("👍");
      expect(f.floats.value[0].left).toBeGreaterThanOrEqual(0);
      expect(f.floats.value[0].left).toBeLessThanOrEqual(100);
    }
  });

  it("caps the number of concurrent floats and drops the rest", () => {
    const f = createReactionFloats();
    for (let i = 0; i < MAX_FLOATS + 5; i++) f.receive("🎉");
    expect(f.floats.value).toHaveLength(MAX_FLOATS);
    vi.advanceTimersByTime(FLOAT_MS);
    f.receive("🎉");
    expect(f.floats.value).toHaveLength(1);
  });

  it("shows the viewer's own reaction at once and skips the server's copy of it once", () => {
    const f = createReactionFloats();
    f.own("👍");
    expect(f.floats.value).toHaveLength(1);
    f.receive("👍"); // the echo
    expect(f.floats.value).toHaveLength(1);
    f.receive("👍"); // somebody else's
    expect(f.floats.value).toHaveLength(2);
  });

  it("does not skip other emoji, nor an echo that never came", () => {
    const f = createReactionFloats();
    f.own("👍");
    f.receive("🎉");
    expect(f.floats.value).toHaveLength(2);
    vi.advanceTimersByTime(10_000); // the server coalesced ours away
    f.receive("👍");
    expect(f.floats.value).toHaveLength(1);
  });

  it("stops its timers on dispose", () => {
    const f = createReactionFloats();
    f.receive("👍");
    f.dispose();
    expect(f.floats.value).toEqual([]);
    expect(vi.getTimerCount()).toBe(0);
  });
});

describe("own reaction announcements", () => {
  it("reach the listeners of that stream only, until unsubscribed", () => {
    const a = vi.fn();
    const b = vi.fn();
    const offA = onOwnReaction(1, a);
    onOwnReaction(2, b);
    announceOwnReaction(1, "👍");
    expect(a).toHaveBeenCalledWith("👍");
    expect(b).not.toHaveBeenCalled();
    offA();
    announceOwnReaction(1, "👍");
    expect(a).toHaveBeenCalledTimes(1);
  });
});
