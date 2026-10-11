import { effectScope } from "vue";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { PlayerEvent } from "@/lib/player/controller";
import { SEEK_DEBOUNCE_MS, useSeekReporting } from "./useSeekReporting";

const reportSeek = vi.hoisted(() => vi.fn());
vi.mock("@/lib/watch-actions", () => ({ reportSeek }));

function fakeController() {
  const handlers = new Map<PlayerEvent, () => void>();
  let time = 0;
  return {
    controller: {
      on: (e: PlayerEvent, fn: () => void) => {
        handlers.set(e, fn);
        return () => handlers.delete(e);
      },
      getCurrentTime: () => time,
    },
    seeked(t: number) {
      time = t;
      handlers.get("seeked")?.();
    },
    handlers,
  };
}

beforeEach(() => {
  vi.useFakeTimers();
  reportSeek.mockReset().mockResolvedValue(undefined);
});
afterEach(() => vi.useRealTimers());

describe("useSeekReporting", () => {
  it("skips the initial seek, then reports once per settled seek", () => {
    const f = fakeController();
    useSeekReporting(f.controller, 5);
    f.seeked(30); // the jump to ?t=
    f.seeked(60);
    f.seeked(90); // scrubbing: only where it settles counts
    vi.advanceTimersByTime(SEEK_DEBOUNCE_MS);
    expect(reportSeek).toHaveBeenCalledTimes(1);
    expect(reportSeek).toHaveBeenCalledWith(5, 90);
  });

  it("counts a first seek after the initial window", () => {
    const f = fakeController();
    useSeekReporting(f.controller, 5);
    vi.advanceTimersByTime(3500);
    f.seeked(10);
    vi.advanceTimersByTime(SEEK_DEBOUNCE_MS);
    expect(reportSeek).toHaveBeenCalledWith(5, 10);
  });

  it("adds the clip offset", () => {
    const f = fakeController();
    useSeekReporting(f.controller, 5, { offset: 100 });
    vi.advanceTimersByTime(3500);
    f.seeked(10);
    vi.advanceTimersByTime(SEEK_DEBOUNCE_MS);
    expect(reportSeek).toHaveBeenCalledWith(5, 110);
  });

  it("stops listening and drops a pending report with its scope", () => {
    const f = fakeController();
    const scope = effectScope();
    scope.run(() => useSeekReporting(f.controller, 5));
    vi.advanceTimersByTime(3500);
    f.seeked(10);
    scope.stop();
    vi.advanceTimersByTime(SEEK_DEBOUNCE_MS);
    expect(reportSeek).not.toHaveBeenCalled();
    expect(f.handlers.size).toBe(0);
  });

  it("swallows a failed report", async () => {
    reportSeek.mockRejectedValue(new Error("down"));
    const f = fakeController();
    useSeekReporting(f.controller, 5);
    vi.advanceTimersByTime(3500);
    f.seeked(10);
    vi.advanceTimersByTime(SEEK_DEBOUNCE_MS);
    await Promise.resolve();
  });
});
