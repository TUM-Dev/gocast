import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "./api";
import { PROGRESS_BACKOFF_MS, PROGRESS_RETRIES, useWatchProgress } from "./use-watch-progress";

const { fetchProgress } = vi.hoisted(() => ({ fetchProgress: vi.fn() }));
vi.mock("./streams", () => ({ fetchProgress }));

const row = (streamId: number, watched = true) => ({ streamId, progress: 1, watched });

/** Runs `load`, letting the retry backoffs elapse on the fake clock. */
async function settled(promise: Promise<void>): Promise<void> {
  await vi.advanceTimersByTimeAsync(PROGRESS_BACKOFF_MS * 10);
  await promise;
}

beforeEach(() => {
  vi.useFakeTimers();
  fetchProgress.mockReset();
});
afterEach(() => vi.useRealTimers());

describe("useWatchProgress", () => {
  it("keeps what was loaded and reports no failure", async () => {
    fetchProgress.mockResolvedValue(new Map([[1, row(1)]]));
    const p = useWatchProgress();

    await settled(p.load([1, 2]));

    expect(p.progress.value.get(1)?.watched).toBe(true);
    expect(p.failed.value).toBe(false);
    expect(fetchProgress).toHaveBeenCalledTimes(1);
  });

  it("retries a transient failure and then succeeds without a notice", async () => {
    fetchProgress
      .mockRejectedValueOnce(new ApiError(503, "busy"))
      .mockResolvedValueOnce(new Map([[1, row(1)]]));
    const p = useWatchProgress();

    await settled(p.load([1]));

    expect(fetchProgress).toHaveBeenCalledTimes(2);
    expect(p.progress.value.get(1)?.watched).toBe(true);
    expect(p.failed.value).toBe(false);
  });

  it("flags a failure that persists, after the retries are used up", async () => {
    fetchProgress.mockRejectedValue(new ApiError(500, "down"));
    const p = useWatchProgress();

    await settled(p.load([1]));

    expect(fetchProgress).toHaveBeenCalledTimes(1 + PROGRESS_RETRIES);
    expect(p.failed.value).toBe(true);
    expect(p.progress.value.size).toBe(0);
  });

  it("asks again for the missing lectures on retry, and clears the flag on success", async () => {
    fetchProgress.mockRejectedValue(new ApiError(500, "down"));
    const p = useWatchProgress();
    await settled(p.load([1, 2]));
    expect(p.failed.value).toBe(true);

    fetchProgress.mockReset();
    fetchProgress.mockResolvedValue(new Map([[2, row(2)]]));
    await settled(p.retry());

    expect(fetchProgress).toHaveBeenCalledWith([1, 2]);
    expect(p.failed.value).toBe(false);
    expect(p.progress.value.get(2)?.watched).toBe(true);
  });

  it("treats a 401 as no progress rather than an error, without retrying", async () => {
    fetchProgress.mockRejectedValue(new ApiError(401, "unauthenticated"));
    const p = useWatchProgress();

    await settled(p.load([1]));

    expect(fetchProgress).toHaveBeenCalledTimes(1);
    expect(p.failed.value).toBe(false);
    expect(p.progress.value.size).toBe(0);
  });

  it("ignores an answer that arrives after a reset", async () => {
    fetchProgress.mockResolvedValue(new Map([[1, row(1)]]));
    const p = useWatchProgress();

    const pending = p.load([1]);
    p.reset();
    await settled(pending);

    expect(p.progress.value.size).toBe(0);
  });
});
