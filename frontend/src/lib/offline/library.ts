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

export interface CourseGroup<T> {
  /** The course's slug, or its name for a lecture saved without one. */
  key: string;
  name: string;
  lectures: T[];
}

/**
 * Saved lectures by course, the way a course page lists them: courses by name, each
 * course's lectures in the order they took place.
 */
export function groupByCourse<T extends Pick<OfflineLectureRecord, "courseSlug" | "courseName" | "start">>(
  lectures: readonly T[],
): CourseGroup<T>[] {
  const groups = new Map<string, CourseGroup<T>>();
  for (const l of lectures) {
    const key = l.courseSlug || l.courseName;
    let group = groups.get(key);
    if (!group) {
      group = { key, name: l.courseName || "Other lectures", lectures: [] };
      groups.set(key, group);
    }
    group.lectures.push(l);
  }
  const out = [...groups.values()];
  for (const g of out) g.lectures.sort((a, b) => a.start - b.start);
  return out.sort((a, b) => a.name.localeCompare(b.name));
}

/** How long the saved copy plays: the cut when there is one, else the whole recording. */
export function playableSeconds(
  l: Pick<OfflineLectureRecord, "durationSeconds" | "startOffset" | "endOffset">,
): number | undefined {
  const clip = clipOf(l);
  const end = clip?.end ?? l.durationSeconds;
  if (end === undefined) return undefined;
  const seconds = end - (clip?.start ?? 0);
  return seconds > 0 ? seconds : undefined;
}

/**
 * How far an unfinished lecture has come, 0 to 1: the live progress while it downloads,
 * else what the record says was stored before the download stopped.
 */
export function downloadFraction(
  l: Pick<OfflineLectureRecord, "downloadedSegments" | "segments">,
  live: number | undefined,
): number {
  if (live !== undefined) return live;
  return l.segments.length > 0 ? l.downloadedSegments / l.segments.length : 0;
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
