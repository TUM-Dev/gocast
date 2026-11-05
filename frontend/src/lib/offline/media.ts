/**
 * The segments of saved lectures, and how the service worker plays them.
 *
 * Segments are stored as the runner encrypted them, in the Cache Storage rather than
 * OPFS: it is there in every browser with service workers, Safari included, the worker
 * reads it directly, and it is under the same quota and persistence as OPFS. They are
 * decrypted one at a time, in the worker, when the player asks for them, so a playable
 * file never exists on the device. Shared by the page and the service worker.
 */

import { deleteKey, getKey, getLastSeen, getLecture, putLastSeen } from "./db";
import { advanceLastSeen, isPlayable } from "./license";
import { hexToBytes, localPlaylist } from "./playlist";

/** Where the player finds saved lectures; only the service worker answers these. */
export const OFFLINE_MEDIA_PREFIX = "/offline-media/";

const MEDIA_CACHE = "gocast-offline-media";

/** The base URL of a saved lecture's playlist and segments. */
export function mediaBase(streamId: number): string {
  return `${OFFLINE_MEDIA_PREFIX}${streamId}`;
}

export function playlistUrl(streamId: number): string {
  return `${mediaBase(streamId)}/playlist.m3u8`;
}

/** The Cache Storage key of a stored segment. Never fetched, only matched. */
function storeKey(packageId: number, index: number): string {
  return `/offline-store/${packageId}/${index}`;
}

export async function putSegment(packageId: number, index: number, body: ArrayBuffer): Promise<void> {
  const cache = await caches.open(MEDIA_CACHE);
  await cache.put(storeKey(packageId, index), new Response(body, { headers: { "Content-Type": "application/octet-stream" } }));
}

/**
 * The Cache Storage key of a package's thumbnail. Under the package's prefix, so
 * deletePackage removes it with the segments.
 */
function thumbnailKey(packageId: number): string {
  return `/offline-store/${packageId}/thumbnail`;
}

/** Keeps a saved lecture's thumbnail, for the offline page, which can't ask the server. */
export async function putThumbnail(packageId: number, body: Blob): Promise<void> {
  const cache = await caches.open(MEDIA_CACHE);
  await cache.put(thumbnailKey(packageId), new Response(body, { headers: { "Content-Type": body.type || "image/jpeg" } }));
}

export async function getThumbnail(packageId: number): Promise<Blob | undefined> {
  const stored = await (await caches.open(MEDIA_CACHE)).match(thumbnailKey(packageId));
  return stored ? stored.blob() : undefined;
}

/** Removes a package's segments, thumbnail and key from the device. */
export async function deletePackage(packageId: number): Promise<void> {
  const cache = await caches.open(MEDIA_CACHE);
  const prefix = `/offline-store/${packageId}/`;
  for (const req of await cache.keys()) {
    if (new URL(req.url).pathname.startsWith(prefix)) await cache.delete(req);
  }
  await deleteKey(packageId);
}

const MEDIA_PATH = /^\/offline-media\/(\d+)\/(?:(playlist)\.m3u8|(\d+)\.ts)$/;

function refuse(status: number): Response {
  return new Response(null, { status });
}

/**
 * Answers a request for a saved lecture's playlist or segment: 403 once the licence
 * has run out or the clock was turned back, 404 for what the device does not have.
 */
export async function serveOfflineMedia(url: URL): Promise<Response> {
  const m = MEDIA_PATH.exec(url.pathname);
  if (!m) return refuse(404);
  const lecture = await getLecture(Number(m[1]));
  if (!lecture) return refuse(404);

  const now = Date.now();
  const lastSeen = await getLastSeen();
  if (!isPlayable(lecture, now, lastSeen)) return refuse(403);
  await putLastSeen(advanceLastSeen(lastSeen, now));

  if (m[2]) {
    const body = localPlaylist(mediaBase(lecture.streamId), lecture.targetDuration, lecture.segments.map((s) => s.duration));
    return new Response(body, { headers: { "Content-Type": "application/vnd.apple.mpegurl", "Cache-Control": "no-store" } });
  }

  const index = Number(m[3]);
  const segment = lecture.segments[index];
  const key = await getKey(lecture.packageId);
  if (!segment || !key) return refuse(404);
  const stored = await (await caches.open(MEDIA_CACHE)).match(storeKey(lecture.packageId, index));
  if (!stored) return refuse(404);

  const plain = await crypto.subtle.decrypt({ name: "AES-CBC", iv: hexToBytes(segment.iv) }, key, await stored.arrayBuffer());
  return new Response(plain, { headers: { "Content-Type": "video/mp2t", "Cache-Control": "no-store" } });
}
