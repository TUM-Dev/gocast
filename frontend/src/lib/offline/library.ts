import type { Clip } from "@/lib/player/clip";
import type { OfflineLectureRecord } from "./db";

/** What the offline page shows about saved lectures. */

/** The stored cut as a clip, as offsetsToClip does for the watch page. */
export function clipOf(l: Pick<OfflineLectureRecord, "startOffset" | "endOffset">): Clip | undefined {
  const start = l.startOffset ?? 0;
  const end = l.endOffset;
  const hasEnd = end !== undefined && end > start;
  if (start <= 0 && !hasEnd) return undefined;
  return hasEnd ? { start, end } : { start };
}

export function formatSize(bytes: number): string {
  if (bytes >= 1e9) return `${(bytes / 1e9).toFixed(1)} GB`;
  if (bytes >= 1e6) return `${Math.round(bytes / 1e6)} MB`;
  return `${Math.max(1, Math.round(bytes / 1e3))} kB`;
}

const DAY_MS = 24 * 60 * 60 * 1000;

/** How long a saved lecture keeps playing without the device going online. */
export function expiryText(expiresAt: number, now: number): string {
  const left = expiresAt - now;
  if (left <= 0) return "Expired. Go online to renew it.";
  const days = Math.floor(left / DAY_MS);
  if (days >= 1) return `Plays offline for ${days} more day${days === 1 ? "" : "s"}`;
  const hours = Math.max(1, Math.floor(left / (60 * 60 * 1000)));
  return `Plays offline for ${hours} more hour${hours === 1 ? "" : "s"}`;
}
