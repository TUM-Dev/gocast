/**
 * Recordings this browser still holds after the tab that made them is gone: closed,
 * crashed, or reloaded. Their recording is over; what is left is to upload the rest
 * and finalize, which any tab of the course can do.
 */

import { abandonBrowserRecording } from "./api";
import type { ChunkStore, RecordingMeta } from "./chunk-store";
import { lockName, server, tryLock } from "./session";
import { UploadQueue, type QueueStatus } from "./upload-queue";

/**
 * The course's recordings in this browser that no tab is working on. One another tab
 * holds is left out: it is being recorded or uploaded there.
 */
export async function unfinishedRecordings(store: ChunkStore, courseId: number): Promise<RecordingMeta[]> {
  const metas = (await store.readMetas()).filter((m) => m.courseId === courseId);
  if (!navigator.locks?.query) return metas;
  const held = new Set((await navigator.locks.query()).held?.map((l) => l.name));
  return metas.filter((m) => !held.has(lockName(m.key)));
}

/**
 * Uploads what is left of a recording and finalizes it. Null when another tab took it
 * up in the meantime.
 */
export async function resumeRecording(
  store: ChunkStore,
  meta: RecordingMeta,
  onChange: (status: QueueStatus) => void,
): Promise<UploadQueue | null> {
  const release = await tryLock(meta.key);
  if (!release) return null;
  meta.stopped = true;
  await store.writeMeta(meta).catch(() => undefined);
  const queue = new UploadQueue({ store, meta, server, onChange });
  await queue.resume();
  void queue.finish().finally(release);
  return queue;
}

/** Throws a recording away, here and on the server. */
export async function discardRecording(store: ChunkStore, meta: RecordingMeta): Promise<void> {
  await abandonBrowserRecording(meta.courseId, meta.streamId, meta.recordingId).catch(() => undefined);
  await store.drop(meta.key);
}

/**
 * What this browser still has of a recording, one file per segment. A segment's first
 * chunk is kept until the recording is finalized, so each file plays; it lacks what
 * the server already has, between that header and the rest.
 */
export async function localCopy(store: ChunkStore, meta: RecordingMeta): Promise<Blob[]> {
  const chunks = await store.list(meta.key);
  const segments = new Map<number, Blob[]>();
  for (const c of chunks) {
    const data = await store.get(meta.key, c.segment, c.seq);
    if (!data) continue;
    if (!segments.has(c.segment)) segments.set(c.segment, []);
    segments.get(c.segment)!.push(data);
  }
  return [...segments.values()].map((parts) => new Blob(parts, { type: meta.mimeType }));
}

/** Hands the browser's copy of a recording to the user as files. */
export async function saveLocalCopy(store: ChunkStore, meta: RecordingMeta): Promise<void> {
  const ext = meta.mimeType.startsWith("video/mp4") ? "mp4" : "webm";
  const files = await localCopy(store, meta);
  files.forEach((blob, i) => {
    const a = document.createElement("a");
    a.href = URL.createObjectURL(blob);
    const part = files.length > 1 ? `-part${i + 1}` : "";
    a.download = `${meta.lectureTitle || "recording"}-${meta.version.toLowerCase()}${part}.${ext}`;
    a.click();
    setTimeout(() => URL.revokeObjectURL(a.href), 60_000);
  });
}
