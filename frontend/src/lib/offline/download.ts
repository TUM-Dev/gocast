import { timestampDate } from "@bufbuild/protobuf/wkt";

import { OfflineLectureState, type OfflineLecture } from "@/gen/server/apiv2_pb";
import { thumbnailUrl } from "@/lib/streams";
import { getOfflineLecture } from "./api";
import { putKey, putLecture, type OfflineLectureRecord } from "./db";
import { putSegment, putThumbnail } from "./media";
import { parsePackagePlaylist } from "./playlist";

/**
 * Bringing a saved lecture onto the device: wait for the runner to package it, then
 * fetch the package from the edge, segment by segment, into the device's storage. The
 * page has to stay open meanwhile; an interrupted download resumes where it stopped.
 */

/** An error meant for the viewer. */
export class OfflineError extends Error {
  constructor(
    message: string,
    /** The package is gone for good: saving again starts over. */
    readonly final = false,
  ) {
    super(message);
    this.name = "OfflineError";
  }
}

/** How often to ask whether the runner is done. Packaging copies, it doesn't encode. */
const POLL_MS = 3000;

export function recordFromLecture(l: OfflineLecture, now = Date.now()): OfflineLectureRecord {
  return {
    streamId: l.stream?.id ?? 0,
    packageId: l.packageId,
    state: "preparing",
    kind: l.kind,
    title: l.stream?.displayName || l.stream?.name || "Lecture",
    courseName: l.course?.name ?? "",
    courseSlug: l.course?.slug ?? "",
    start: l.stream?.start ? timestampDate(l.stream.start).getTime() : 0,
    durationSeconds: l.stream?.duration,
    startOffset: l.startOffset,
    endOffset: l.endOffset,
    targetDuration: 0,
    segments: [],
    downloadedSegments: 0,
    sizeBytes: Number(l.sizeBytes),
    expiresAt: 0,
    savedAt: now,
    completed: false,
  };
}

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

/** Waits until the package is ready, and returns the answer that carries the licence. */
export async function waitUntilReady(
  packageId: number,
  fetchLecture: (id: number) => Promise<OfflineLecture> = getOfflineLecture,
  wait: (ms: number) => Promise<unknown> = sleep,
): Promise<OfflineLecture> {
  for (;;) {
    const l = await fetchLecture(packageId);
    switch (l.state) {
      case OfflineLectureState.READY:
        if (l.license) return l;
        throw new OfflineError("The server didn't hand out the lecture's licence.", true);
      case OfflineLectureState.PREPARING:
        await wait(POLL_MS);
        break;
      case OfflineLectureState.REVOKED:
        throw new OfflineError("This lecture can no longer be saved offline.", true);
      default:
        throw new OfflineError(l.error || "The lecture couldn't be prepared for offline viewing.", true);
    }
  }
}

/**
 * Asks the browser to keep what is stored, rather than evict it when space runs low,
 * and checks there is room for `bytes` more. Browsers that answer neither are let try.
 */
export async function ensureSpace(bytes: number): Promise<void> {
  if (!navigator.storage) return;
  await navigator.storage.persist?.().catch(() => false);
  const { quota, usage } = (await navigator.storage.estimate?.().catch(() => undefined)) ?? {};
  if (quota !== undefined && usage !== undefined && quota - usage < bytes * 1.1) {
    throw new OfflineError("There isn't enough space on this device to save the lecture.");
  }
}

/**
 * Fetches a lecture's thumbnail into the device's storage and returns it. Decoration
 * only: without one the offline page shows a plain tile, so a failure is undefined.
 */
export async function fetchThumbnail(record: Pick<OfflineLectureRecord, "packageId" | "streamId" | "courseSlug">): Promise<Blob | undefined> {
  if (!record.courseSlug) return undefined;
  try {
    const res = await fetch(thumbnailUrl(record.courseSlug, record.streamId));
    if (!res.ok) return undefined;
    const blob = await res.blob();
    if (!blob.type.startsWith("image/")) return undefined;
    await putThumbnail(record.packageId, blob);
    return blob;
  } catch {
    return undefined;
  }
}

/**
 * Downloads a ready package into the device's storage and returns the saved record.
 * The key is imported once, as a non-extractable CryptoKey: from then on the device
 * can decrypt with it but nothing can read it back.
 */
export async function downloadPackage(
  record: OfflineLectureRecord,
  lecture: OfflineLecture,
  onProgress: (fraction: number) => void,
): Promise<OfflineLectureRecord> {
  const license = lecture.license;
  if (!license) throw new OfflineError("The server didn't hand out the lecture's licence.", true);

  const key = await crypto.subtle.importKey("raw", new Uint8Array(license.key), { name: "AES-CBC" }, false, ["decrypt"]);
  await putKey(record.packageId, key);

  const res = await fetch(license.playlistUrl);
  if (!res.ok) throw new OfflineError("The lecture couldn't be downloaded. Please try again.");
  const playlist = parsePackagePlaylist(await res.text(), license.playlistUrl);

  let r: OfflineLectureRecord = {
    ...record,
    state: "downloading",
    targetDuration: playlist.targetDuration,
    segments: playlist.segments.map((s) => ({ duration: s.duration, iv: s.iv })),
    sizeBytes: Number(lecture.sizeBytes),
  };
  await putLecture(r);

  const total = playlist.segments.length;
  for (let i = r.downloadedSegments; i < total; i++) {
    const seg = await fetch(playlist.segments[i].url);
    if (!seg.ok) throw new OfflineError("The download was interrupted. Save the lecture again to resume.");
    await putSegment(r.packageId, i, await seg.arrayBuffer());
    r = { ...r, downloadedSegments: i + 1 };
    await putLecture(r);
    onProgress(r.downloadedSegments / total);
  }
  await fetchThumbnail(r);

  r = {
    ...r,
    state: "saved",
    expiresAt: license.expiresAt ? timestampDate(license.expiresAt).getTime() : 0,
    savedAt: Date.now(),
  };
  await putLecture(r);
  return r;
}
