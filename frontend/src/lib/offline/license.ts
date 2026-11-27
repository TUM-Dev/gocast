/**
 * When a saved lecture may play. The server's licence runs for a week from the last
 * time the device synced; past that, or when the clock was turned back to stretch it,
 * the service worker refuses to decrypt. Shared by the page and the service worker.
 */

/**
 * How far the clock may go back before it counts as turned back: clocks get corrected
 * by a few seconds to minutes all the time, and that must not lock anybody out.
 */
export const CLOCK_TOLERANCE_MS = 5 * 60 * 1000;

export interface Licensed {
  state: string;
  expiresAt: number;
}

/**
 * Whether a saved lecture may play at `now`, given `lastSeen`, the latest time this
 * device has seen so far.
 */
export function isPlayable(lecture: Licensed, now: number, lastSeen: number): boolean {
  return lecture.state === "saved" && now < lecture.expiresAt && now >= lastSeen - CLOCK_TOLERANCE_MS;
}

/** The latest time seen, which only ever moves forward. */
export function advanceLastSeen(lastSeen: number, now: number): number {
  return Math.max(lastSeen, now);
}
