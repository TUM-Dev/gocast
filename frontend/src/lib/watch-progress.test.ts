import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { createProgressReporter } from "./watch-progress";

describe("progress reporter", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("reports every interval while playing and at once on pause", () => {
    const save = vi.fn().mockResolvedValue(undefined);
    const r = createProgressReporter({ fraction: () => 0.25, save });
    r.onPlay();
    vi.advanceTimersByTime(20_000);
    expect(save).toHaveBeenCalledTimes(2);
    r.onPause();
    expect(save).toHaveBeenCalledTimes(3);
    vi.advanceTimersByTime(30_000);
    expect(save).toHaveBeenCalledTimes(3);
  });

  it("stops at the end and skips unknown positions", () => {
    const save = vi.fn().mockResolvedValue(undefined);
    let f: number | undefined;
    const r = createProgressReporter({ fraction: () => f, save });
    r.onPlay();
    vi.advanceTimersByTime(10_000);
    expect(save).not.toHaveBeenCalled();
    f = 0.5;
    r.onEnded();
    vi.advanceTimersByTime(30_000);
    expect(save).not.toHaveBeenCalled();
  });

  it("doubles the interval after a failure", async () => {
    const save = vi.fn().mockRejectedValue(new Error("x"));
    const r = createProgressReporter({ fraction: () => 0.5, save });
    r.onPlay();
    await vi.advanceTimersByTimeAsync(10_000);
    expect(save).toHaveBeenCalledTimes(1);
    // The next report was already scheduled when the first failed; the one after waits 20 s.
    await vi.advanceTimersByTimeAsync(10_000);
    expect(save).toHaveBeenCalledTimes(2);
    await vi.advanceTimersByTimeAsync(10_000);
    expect(save).toHaveBeenCalledTimes(2);
    await vi.advanceTimersByTimeAsync(10_000);
    expect(save).toHaveBeenCalledTimes(3);
    r.dispose();
  });
});
