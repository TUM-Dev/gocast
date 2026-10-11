import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { fakePlayer } from "@/test/fake-player";

import { bufferAhead, collectStats, createStatsPanel, formatBits, statRows, type StatsPlayer } from "./stats";
import { timeRanges } from "./types";

function statsPlayer(extra: Partial<StatsPlayer> = {}) {
  const { player, state, el } = fakePlayer();
  return { player: Object.assign(player, extra) as unknown as StatsPlayer, state, el };
}

describe("bufferAhead", () => {
  it("measures from the playhead to the end of the range holding it", () => {
    expect(bufferAhead(timeRanges([0, 10], [20, 40]), 25)).toBe(15);
    expect(bufferAhead(timeRanges([0, 10]), 4)).toBe(6);
  });
  it("is 0 outside every range", () => {
    expect(bufferAhead(timeRanges([0, 10]), 15)).toBe(0);
    expect(bufferAhead(timeRanges(), 0)).toBe(0);
  });
});

describe("collectStats", () => {
  it("reads resolution, bitrate, bandwidth, buffer and dropped frames", () => {
    const { player, state } = statsPlayer({
      videoWidth: () => 1280,
      videoHeight: () => 720,
      getVideoPlaybackQuality: () => ({ droppedVideoFrames: 3, totalVideoFrames: 900 }),
      tech: () => ({ vhs: { bandwidth: 4_500_000, playlists: { media: () => ({ attributes: { BANDWIDTH: 2_800_000 } }) } } }),
    });
    state.time = 10; // buffered() is [0, 50] for duration 100
    expect(collectStats(player)).toMatchObject({
      resolution: { width: 1280, height: 720 },
      bitrate: 2_800_000,
      bandwidth: 4_500_000,
      bufferSeconds: 40,
      dropped: { dropped: 3, total: 900 },
      rate: 1,
    });
  });

  it("leaves out what the player cannot tell (native HLS, no tech, no quality API)", () => {
    const { player } = statsPlayer({ videoWidth: () => 0, videoHeight: () => 0, tech: () => ({}) });
    const stats = collectStats(player);
    expect(stats.resolution).toBeUndefined();
    expect(stats.bitrate).toBeUndefined();
    expect(stats.bandwidth).toBeUndefined();
    expect(stats.dropped).toBeUndefined();
    expect(stats.latencySeconds).toBeUndefined();
  });

  it("reports the latency of a live stream, and only of a live one", () => {
    const { player, state } = statsPlayer();
    state.time = 90;
    expect(collectStats(player, { isLive: () => true, liveCurrentTime: () => 96.5 }).latencySeconds).toBe(6.5);
    expect(collectStats(player, { isLive: () => false, liveCurrentTime: () => 96.5 }).latencySeconds).toBeUndefined();
    expect(collectStats(player, { isLive: () => true, liveCurrentTime: () => 80 }).latencySeconds).toBe(0);
  });
});

describe("formatting", () => {
  it("formats bit rates", () => {
    expect(formatBits(2_800_000)).toBe("2.80 Mbit/s");
    expect(formatBits(640_000)).toBe("640 kbit/s");
    expect(formatBits(undefined)).toBe("n/a");
  });

  it("shows n/a for gaps and a latency row for live streams only", () => {
    const rows = Object.fromEntries(statRows({ rate: 1.5, bufferSeconds: 2.34 }));
    expect(rows).toMatchObject({ Resolution: "n/a", Bandwidth: "n/a", Buffer: "2.3 s", "Dropped frames": "n/a", Speed: "1.5x" });
    expect(rows["Live latency"]).toBeUndefined();
    expect(Object.fromEntries(statRows({ rate: 1, latencySeconds: 7 }))["Live latency"]).toBe("7.0 s");
  });
});

describe("createStatsPanel", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("is closed until toggled, refreshes while open and stops when closed", () => {
    const { player, state, el } = statsPlayer({ videoWidth: () => 640, videoHeight: () => 360 });
    const panel = createStatsPanel(player, undefined, 1000);
    const root = el.querySelector<HTMLElement>(".vjs-stats-panel")!;
    expect(root.hidden).toBe(true);
    expect(vi.getTimerCount()).toBe(0);

    panel.toggle();
    expect(root.hidden).toBe(false);
    expect(root.textContent).toContain("640x360");
    state.rate = 2;
    vi.advanceTimersByTime(1000);
    expect(root.textContent).toContain("2x");

    root.querySelector<HTMLButtonElement>(".vjs-stats-close")!.click();
    expect(panel.isOpen()).toBe(false);
    expect(vi.getTimerCount()).toBe(0);
  });

  it("removes itself on dispose", () => {
    const { player, el } = statsPlayer();
    const panel = createStatsPanel(player);
    panel.toggle();
    panel.dispose();
    expect(el.querySelector(".vjs-stats-panel")).toBeNull();
    expect(vi.getTimerCount()).toBe(0);
  });
});
