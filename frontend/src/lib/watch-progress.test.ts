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
    let f = 0.25;
    const r = createProgressReporter({ fraction: () => f, save });
    r.onPlay();
    f = 0.3;
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
    r.onPlay();
    f = 0.6;
    r.onPause();
    expect(save).toHaveBeenCalledTimes(1);
    save.mockClear();
    r.onEnded();
    vi.advanceTimersByTime(30_000);
    expect(save).not.toHaveBeenCalled();
  });

  it("doubles the interval after a failure", async () => {
    const save = vi.fn().mockRejectedValue(new Error("x"));
    let f = 0.5;
    const r = createProgressReporter({ fraction: () => f, save });
    r.onPlay();
    f = 0.6;
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

  it("reports nothing for a lecture that is only opened, or restarted at 0:00", () => {
    const save = vi.fn().mockResolvedValue(undefined);
    const r = createProgressReporter({ fraction: () => 0, save });
    // A finished lecture restarts at 0:00 and the viewer plays and pauses without moving,
    // or leaves: neither may overwrite the saved progress with ~0.
    r.onPause();
    r.onPlay();
    r.onPause();
    r.dispose();
    expect(save).not.toHaveBeenCalled();
  });

  it("waits for the position to move by minAdvance, then reports", () => {
    const save = vi.fn().mockResolvedValue(undefined);
    let f = 0;
    const r = createProgressReporter({ fraction: () => f, save, minAdvance: () => 0.01 });
    r.onPlay();
    f = 0.005;
    r.onPause();
    expect(save).not.toHaveBeenCalled();
    r.onPlay();
    f = 0.02;
    r.onPause();
    expect(save).toHaveBeenCalledExactlyOnceWith(0.02);
  });

  it("counts a seek during playback as having engaged", () => {
    const save = vi.fn().mockResolvedValue(undefined);
    let f = 0;
    const r = createProgressReporter({ fraction: () => f, save, minAdvance: () => 0.01 });
    r.onPlay();
    f = 0.7;
    r.onPause();
    expect(save).toHaveBeenCalledExactlyOnceWith(0.7);
  });
});
