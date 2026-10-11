import type { PlayerLike } from "./types";

/**
 * What the stats panel reads beyond {@link PlayerLike}. Every part is optional because
 * video.js only has them once a source is loaded (the tech, VHS), and not at all on
 * Safari's native HLS engine or in browsers without the playback-quality API: a stat
 * that cannot be read is shown as "n/a", never as 0.
 */
export interface StatsPlayer extends Pick<PlayerLike, "el" | "on" | "off" | "currentTime" | "buffered" | "playbackRate"> {
  videoWidth?(): number;
  videoHeight?(): number;
  getVideoPlaybackQuality?(): { droppedVideoFrames?: number; totalVideoFrames?: number } | undefined;
  /** video.js' `tech(safe)`; `vhs` is the HLS engine, absent on native playback. */
  tech?(safe?: boolean): { vhs?: VhsLike } | undefined;
}

export interface VhsLike {
  /** The player's estimate of the connection, bits per second. */
  bandwidth?: number;
  playlists?: { media?(): { attributes?: { BANDWIDTH?: number } } | undefined };
}

/** The parts of video.js' LiveTracker the panel reads. */
export interface LatencyTrackerLike {
  isLive?(): boolean;
  liveCurrentTime?(): number;
}

export interface PlayerStats {
  /** Decoded picture size, in pixels. */
  resolution?: { width: number; height: number };
  /** Size of the player's box, in CSS pixels. */
  viewport?: { width: number; height: number };
  /** Declared bitrate of the rendition being played, bits per second. */
  bitrate?: number;
  /** The player's measured bandwidth, bits per second. */
  bandwidth?: number;
  /** Seconds of video buffered ahead of the playhead. */
  bufferSeconds?: number;
  dropped?: { dropped: number; total: number };
  /** Seconds behind the live edge; live streams only. */
  latencySeconds?: number;
  rate: number;
}

const positive = (n: number | undefined): n is number => typeof n === "number" && Number.isFinite(n) && n > 0;

/** Seconds buffered ahead of `time` in the range that contains it; 0 if none does. */
export function bufferAhead(ranges: TimeRanges, time: number): number {
  for (let i = 0; i < ranges.length; i++) {
    if (time >= ranges.start(i) - 0.1 && time <= ranges.end(i)) return Math.max(0, ranges.end(i) - time);
  }
  return 0;
}

/** Reads one snapshot. Never throws: a player half-way through loading has gaps, not errors. */
export function collectStats(player: StatsPlayer, tracker?: LatencyTrackerLike): PlayerStats {
  const stats: PlayerStats = { rate: player.playbackRate() };
  const time = player.currentTime();

  const width = player.videoWidth?.();
  const height = player.videoHeight?.();
  if (positive(width) && positive(height)) stats.resolution = { width, height };

  const box = player.el();
  if (box.clientWidth > 0 && box.clientHeight > 0) stats.viewport = { width: box.clientWidth, height: box.clientHeight };

  const vhs = player.tech?.(true)?.vhs;
  const declared = vhs?.playlists?.media?.()?.attributes?.BANDWIDTH;
  if (positive(declared)) stats.bitrate = declared;
  if (positive(vhs?.bandwidth)) stats.bandwidth = vhs.bandwidth;

  stats.bufferSeconds = bufferAhead(player.buffered(), time);

  const quality = player.getVideoPlaybackQuality?.();
  if (quality && typeof quality.droppedVideoFrames === "number" && typeof quality.totalVideoFrames === "number") {
    stats.dropped = { dropped: quality.droppedVideoFrames, total: quality.totalVideoFrames };
  }

  if (tracker?.isLive?.() && tracker.liveCurrentTime) {
    stats.latencySeconds = Math.max(0, tracker.liveCurrentTime() - time);
  }
  return stats;
}

export function formatBits(bitsPerSecond: number | undefined): string {
  if (bitsPerSecond === undefined) return "n/a";
  if (bitsPerSecond >= 1e6) return `${(bitsPerSecond / 1e6).toFixed(2)} Mbit/s`;
  return `${Math.round(bitsPerSecond / 1e3)} kbit/s`;
}

/** The panel's rows, label and value. Live-only rows are left out of a recording's. */
export function statRows(stats: PlayerStats): [string, string][] {
  const size = (s?: { width: number; height: number }) => (s ? `${s.width}x${s.height}` : "n/a");
  const rows: [string, string][] = [
    ["Resolution", size(stats.resolution)],
    ["Player size", size(stats.viewport)],
    ["Stream bitrate", formatBits(stats.bitrate)],
    ["Bandwidth", formatBits(stats.bandwidth)],
    ["Buffer", stats.bufferSeconds === undefined ? "n/a" : `${stats.bufferSeconds.toFixed(1)} s`],
    ["Dropped frames", stats.dropped ? `${stats.dropped.dropped} / ${stats.dropped.total}` : "n/a"],
    ["Speed", `${stats.rate}x`],
  ];
  if (stats.latencySeconds !== undefined) rows.push(["Live latency", `${stats.latencySeconds.toFixed(1)} s`]);
  return rows;
}

export interface StatsPanel {
  toggle(): void;
  isOpen(): boolean;
  dispose(): void;
}

/**
 * A "stats for nerds" overlay, plain DOM inside the player's element so it stays visible
 * in fullscreen (the player's element is what goes fullscreen, not the page around it).
 * It samples once a second while open and does nothing while closed.
 */
export function createStatsPanel(player: StatsPlayer, tracker?: LatencyTrackerLike, intervalMs = 1000): StatsPanel {
  const panel = document.createElement("div");
  panel.className = "vjs-stats-panel";
  panel.setAttribute("role", "region");
  panel.setAttribute("aria-label", "Stats for nerds");
  panel.hidden = true;

  const close = document.createElement("button");
  close.type = "button";
  close.className = "vjs-stats-close";
  close.setAttribute("aria-label", "Close stats");
  close.textContent = "×";
  const list = document.createElement("dl");
  panel.append(close, list);
  player.el().appendChild(panel);

  let timer: ReturnType<typeof setInterval> | null = null;
  const render = () => {
    list.replaceChildren(
      ...statRows(collectStats(player, tracker)).flatMap(([label, value]) => {
        const dt = document.createElement("dt");
        dt.textContent = label;
        const dd = document.createElement("dd");
        dd.textContent = value;
        return [dt, dd];
      }),
    );
  };
  const stop = () => {
    if (timer !== null) clearInterval(timer);
    timer = null;
  };
  const setOpen = (open: boolean) => {
    panel.hidden = !open;
    stop();
    if (open) {
      render();
      timer = setInterval(render, intervalMs);
    }
  };
  close.addEventListener("click", () => setOpen(false));

  return {
    toggle: () => setOpen(panel.hidden),
    isOpen: () => !panel.hidden,
    dispose() {
      stop();
      panel.remove();
    },
  };
}
