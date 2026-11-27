import type { OfflineLectureRecord } from "./db";
import { isPlayable } from "./license";
import { playlistUrl } from "./media";

/**
 * Whether the watch page plays a view of a lecture from the device rather than the
 * network: when exactly that view is saved, its licence holds, and a service worker is
 * there to answer the request. Playing the saved copy saves the bandwidth even online.
 * The clock check is the service worker's; this only avoids offering a copy it is
 * certain to refuse.
 */
export function localSource(
  lecture: OfflineLectureRecord | undefined,
  kind: number,
  controlled: boolean,
  now = Date.now(),
): string | null {
  if (!lecture || !controlled || lecture.kind !== kind) return null;
  if (!isPlayable(lecture, now, 0)) return null;
  return playlistUrl(lecture.streamId);
}
