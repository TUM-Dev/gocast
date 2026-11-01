import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "./api";
import { cutErrorMessage, pollWaveform, waveformDuration, type Waveform } from "./lecture-cut";

const waveform = (durations: Record<string, number>): Waveform => ({
  peaks: new Uint8Array([1]),
  bucketSeconds: 1,
  versionDurations: durations,
  versionsDiffer: false,
  silences: [],
});

describe("waveformDuration", () => {
  it("is the default version's", () => {
    expect(waveformDuration(waveform({ COMB: 3600, CAM: 3590 }))).toBe(3600);
  });

  it("falls back to the longest it could measure", () => {
    expect(waveformDuration(waveform({ CAM: 3590, PRES: 3595 }))).toBe(3595);
  });

  it("is 0 when nothing was measured", () => {
    expect(waveformDuration(waveform({}))).toBe(0);
  });
});

describe("pollWaveform", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("asks again until the timeline is there", async () => {
    const w = waveform({ COMB: 10 });
    const fetch = vi.fn().mockResolvedValueOnce(null).mockResolvedValueOnce(null).mockResolvedValueOnce(w);
    const onReady = vi.fn();
    pollWaveform({ fetch, onReady, onError: vi.fn(), intervalMs: 1000 });

    await vi.advanceTimersByTimeAsync(0);
    expect(onReady).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(2000);
    expect(onReady).toHaveBeenCalledWith(w);
    expect(fetch).toHaveBeenCalledTimes(3);

    await vi.advanceTimersByTimeAsync(5000);
    expect(fetch).toHaveBeenCalledTimes(3);
  });

  it("stops on an error", async () => {
    const err = new ApiError(500, "boom");
    const fetch = vi.fn().mockRejectedValue(err);
    const onError = vi.fn();
    pollWaveform({ fetch, onReady: vi.fn(), onError, intervalMs: 1000 });

    await vi.advanceTimersByTimeAsync(5000);
    expect(onError).toHaveBeenCalledExactlyOnceWith(err);
    expect(fetch).toHaveBeenCalledTimes(1);
  });

  it("gives up when the runner doesn't answer", async () => {
    const fetch = vi.fn().mockResolvedValue(null);
    const onError = vi.fn();
    let t = 0;
    pollWaveform({ fetch, onReady: vi.fn(), onError, intervalMs: 1000, giveUpMs: 3000, now: () => t });

    for (let i = 0; i < 6; i++) {
      t += 1000;
      await vi.advanceTimersByTimeAsync(1000);
    }
    expect(onError).toHaveBeenCalledOnce();
    expect(cutErrorMessage(onError.mock.calls[0][0])).toMatch(/taking too long/);
  });

  it("can be stopped", async () => {
    const fetch = vi.fn().mockResolvedValue(null);
    const stop = pollWaveform({ fetch, onReady: vi.fn(), onError: vi.fn(), intervalMs: 1000 });
    await vi.advanceTimersByTimeAsync(0);
    stop();
    await vi.advanceTimersByTimeAsync(10_000);
    expect(fetch).toHaveBeenCalledTimes(1);
  });
});

describe("cutErrorMessage", () => {
  it("says what happened", () => {
    expect(cutErrorMessage(new ApiError(401, "x"))).toMatch(/session expired/);
    expect(cutErrorMessage(new ApiError(404, "x"))).toMatch(/does not exist/);
    expect(cutErrorMessage(new ApiError(409, "x"))).toMatch(/being converted/);
    expect(cutErrorMessage(new ApiError(503, "x"))).toMatch(/No runner/);
    expect(cutErrorMessage(new ApiError(400, "the cut leaves less than a second"))).toBe("the cut leaves less than a second");
    expect(cutErrorMessage(new Error("?"))).toMatch(/Something went wrong/);
  });
});
