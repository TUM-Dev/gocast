import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { apiPatchMessage } from "./api";
import { createProgressReporter, createWatchedMarker, markWatched } from "./watch-progress";

vi.mock("./api", () => ({ apiPatchMessage: vi.fn().mockResolvedValue({}) }));

describe("marking a lecture watched", () => {
  it("sends full progress with the watched flag", async () => {
    await markWatched(7);
    const [, , path, body] = vi.mocked(apiPatchMessage).mock.calls.at(-1)!;
    expect(path).toBe("/progress/7");
    expect(body).toMatchObject({ streamId: 7, progress: 1, watched: true });
  });

  it("fires once per marker, and again only after a failure", async () => {
    const mark = vi.fn().mockResolvedValueOnce(undefined);
    const m = createWatchedMarker(mark);
    m.fire();
    m.fire();
    await Promise.resolve();
    expect(mark).toHaveBeenCalledTimes(1);

    const failing = vi.fn().mockRejectedValueOnce(new Error("x")).mockResolvedValue(undefined);
    const f = createWatchedMarker(failing);
    f.fire();
    await new Promise((r) => setTimeout(r));
    f.fire();
    f.fire();
    expect(failing).toHaveBeenCalledTimes(2);
  });
});

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
