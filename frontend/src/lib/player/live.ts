import type { PlayerLike } from "./types";

/** The parts of video.js' LiveTracker used here. */
export interface LiveTrackerLike {
  on(event: string, fn: () => void): void;
  off(event: string, fn: () => void): void;
  atLiveEdge(): boolean;
  isLive(): boolean;
  seekToLiveEdge(): void;
}

/**
 * Catching up to a live stream is done at more than 1x; once the viewer is back at the
 * edge, staying at that rate would run out of video, so drop to 1x. Returns the
 * function that stops listening.
 */
export function resetRateAtLiveEdge(player: PlayerLike, tracker: LiveTrackerLike): () => void {
  const onEdgeChange = () => {
    if (tracker.atLiveEdge() && player.playbackRate() > 1) player.playbackRate(1);
  };
  tracker.on("liveedgechange", onEdgeChange);
  return () => tracker.off("liveedgechange", onEdgeChange);
}

/**
 * The "seek to live" button is video.js' own and appears with the `liveui` option; this
 * is the same action for a parent (a "Live" badge on the page, a hotkey).
 */
export function seekToLive(tracker: LiveTrackerLike | undefined): void {
  if (tracker?.isLive()) tracker.seekToLiveEdge();
}
