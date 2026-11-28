import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { apiPatchMessage } from "./api";
import { createProgressReporter, createWatchedMarker, markWatched, saveProgress } from "./watch-progress";

vi.mock("./api", () => ({ apiPatchMessage: vi.fn().mockResolvedValue({}) }));

describe("saving progress", () => {
  it("leaves the watched flag out, so it cannot unmark a lecture", async () => {
    await saveProgress(7, 0.4);
    const [, , path, body] = vi.mocked(apiPatchMessage).mock.calls.at(-1)!;
    expect(path).toBe("/progress/7");
    expect(body).toMatchObject({ streamId: 7, progress: 0.4 });
    // Absent (undefined), not false: the server treats an explicit false as "unmark".
    expect((body as { watched?: boolean }).watched).toBeUndefined();
  });
});

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
    vi.advanceTimersByTime(5_000);
    r.onPause();
    expect(save).toHaveBeenCalledTimes(3);
    vi.advanceTimersByTime(30_000);
    expect(save).toHaveBeenCalledTimes(3);
  });

  it("holds a pause back until the last report is old enough, and still sends it", () => {
    const save = vi.fn().mockResolvedValue(undefined);
    let f = 0.25;
    const r = createProgressReporter({ fraction: () => f, save });
    r.onPlay();
    f = 0.3;
    vi.advanceTimersByTime(10_000);
    expect(save).toHaveBeenCalledTimes(1);
    f = 0.31;
    r.onPause();
    expect(save).toHaveBeenCalledTimes(1);
    vi.advanceTimersByTime(4_999);
    expect(save).toHaveBeenCalledTimes(1);
    vi.advanceTimersByTime(1);
    expect(save).toHaveBeenCalledTimes(2);
    expect(save).toHaveBeenLastCalledWith(0.31);
    vi.advanceTimersByTime(60_000);
    expect(save).toHaveBeenCalledTimes(2);
  });

  it("sends one report per gap however fast the player pauses and plays", () => {
    const save = vi.fn().mockResolvedValue(undefined);
    let f = 0.25;
    const r = createProgressReporter({ fraction: () => f, save });
    r.onPlay();
    f = 0.3;
    r.onPause();
    expect(save).toHaveBeenCalledTimes(1);
    for (let i = 0; i < 200; i++) {
      f += 0.0001;
      r.onPlay();
      vi.advanceTimersByTime(10);
      r.onPause();
    }
    // 200 loops took 2 s: nothing beyond the first report has gone out yet.
    expect(save).toHaveBeenCalledTimes(1);
    vi.advanceTimersByTime(5_000);
    expect(save).toHaveBeenCalledTimes(2);
    r.dispose();
  });

  it("does not send a held-back pause once playback resumes", () => {
    const save = vi.fn().mockResolvedValue(undefined);
    let f = 0.25;
    const r = createProgressReporter({ fraction: () => f, save });
    r.onPlay();
    f = 0.3;
    r.onPause();
    f = 0.31;
    r.onPlay();
    r.onPause();
    r.onPlay();
    vi.advanceTimersByTime(4_000);
    expect(save).toHaveBeenCalledTimes(1);
    r.dispose();
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

  it("stops doubling at the cap and returns to the interval after a success", async () => {
    const save = vi.fn().mockRejectedValue(new Error("x"));
    let f = 0.5;
    const r = createProgressReporter({ fraction: () => f, save });
    r.onPlay();
    f = 0.6;
    // 10, 20, 40, 80 s between reports, then 80 s however many more fail.
    await vi.advanceTimersByTimeAsync(10_000 + 10_000 + 20_000 + 40_000 + 80_000);
    const failed = save.mock.calls.length;
    await vi.advanceTimersByTimeAsync(80_000);
    expect(save).toHaveBeenCalledTimes(failed + 1);
    await vi.advanceTimersByTimeAsync(80_000);
    expect(save).toHaveBeenCalledTimes(failed + 2);

    save.mockResolvedValue(undefined);
    // The report already scheduled keeps its 80 s; the one it schedules is back to 10 s.
    await vi.advanceTimersByTimeAsync(80_000);
    await vi.advanceTimersByTimeAsync(80_000);
    const recovered = save.mock.calls.length;
    await vi.advanceTimersByTimeAsync(10_000);
    expect(save).toHaveBeenCalledTimes(recovered + 1);
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
