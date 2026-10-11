import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { StreamWatch } from "./watch";
import { expectsRecording, pollDelay, pollForRecording } from "./watch-recording-poll";

const NOW = new Date("2026-10-11T12:00:00Z");
const watch = (stream: Record<string, unknown>) => ({ stream, views: [] }) as unknown as StreamWatch;
const endedAgo = (minutes: number) => timestampFromDate(new Date(NOW.getTime() - minutes * 60_000));

describe("pollDelay", () => {
  it("backs off from 15 s to a minute", () => {
    expect([0, 1, 2, 3, 10].map(pollDelay)).toEqual([15_000, 30_000, 60_000, 60_000, 60_000]);
  });
});

describe("expectsRecording", () => {
  it("expects one when the viewer saw the stream end", () => {
    expect(expectsRecording(watch({ ended: true, end: endedAgo(600) }), NOW, true)).toBe(true);
  });
  it("expects one for a recently ended stream the viewer arrived late to", () => {
    expect(expectsRecording(watch({ ended: true, end: endedAgo(20) }), NOW, false)).toBe(true);
  });
  it("does not poll for an old lecture that never had a recording", () => {
    expect(expectsRecording(watch({ ended: true, end: endedAgo(60 * 24) }), NOW, false)).toBe(false);
  });
  it("does not poll once there is a recording or while live", () => {
    expect(expectsRecording(watch({ ended: true, recording: true, end: endedAgo(5) }), NOW, true)).toBe(false);
    expect(expectsRecording(watch({ liveNow: true }), NOW, true)).toBe(false);
  });
});

describe("pollForRecording", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("checks on the backoff and stops when the check succeeds", async () => {
    const check = vi.fn().mockResolvedValueOnce(false).mockResolvedValueOnce(true);
    pollForRecording(check);
    await vi.advanceTimersByTimeAsync(14_999);
    expect(check).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(1);
    expect(check).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(30_000);
    expect(check).toHaveBeenCalledTimes(2);
    await vi.advanceTimersByTimeAsync(10 * 60_000);
    expect(check).toHaveBeenCalledTimes(2);
  });

  it("treats a throwing check as a miss and keeps going", async () => {
    const check = vi.fn().mockRejectedValueOnce(new Error("x")).mockResolvedValue(true);
    pollForRecording(check);
    await vi.advanceTimersByTimeAsync(15_000 + 30_000);
    expect(check).toHaveBeenCalledTimes(2);
  });

  it("stops when asked", async () => {
    const check = vi.fn().mockResolvedValue(false);
    pollForRecording(check).stop();
    await vi.advanceTimersByTimeAsync(5 * 60_000);
    expect(check).not.toHaveBeenCalled();
  });

  it("gives up after the limit", async () => {
    const check = vi.fn().mockResolvedValue(false);
    const onGiveUp = vi.fn();
    pollForRecording(check, { giveUpMs: 5 * 60_000, onGiveUp });
    await vi.advanceTimersByTimeAsync(60 * 60_000);
    expect(onGiveUp).toHaveBeenCalledTimes(1);
    // Waits of 15, 30, 60, 60, 60 and 60 s reach 285 s; the next would pass 300 s.
    expect(check).toHaveBeenCalledTimes(6);
  });
});
