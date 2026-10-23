/**
 * What a device keeps about its saved lectures, in IndexedDB: the lectures, their keys
 * and the latest time seen. The segments themselves are in the Cache Storage
 * (media.ts). Shared by the page and the service worker, which both open it.
 */

export const OFFLINE_DB = "gocast-offline";
const DB_VERSION = 1;

const LECTURES = "lectures";
const KEYS = "keys";
const META = "meta";

export interface OfflineSegment {
  duration: number;
  iv: string;
}

/**
 * A lecture saved, or being saved, on this device. One per stream: saving a lecture
 * again replaces it.
 */
export interface OfflineLectureRecord {
  streamId: number;
  packageId: number;
  /** "preparing" on the server, "downloading" to the device, then "saved". */
  state: "preparing" | "downloading" | "saved";
  /** The WatchViewKind that was saved. */
  kind: number;
  title: string;
  courseName: string;
  courseSlug: string;
  /** When the lecture took place, epoch ms. */
  start: number;
  durationSeconds?: number;
  /** The cut, in seconds. */
  startOffset?: number;
  endOffset?: number;
  targetDuration: number;
  segments: OfflineSegment[];
  downloadedSegments: number;
  sizeBytes: number;
  /** When the licence runs out unless renewed, epoch ms. */
  expiresAt: number;
  savedAt: number;
  /**
   * Whether the server has been told the download is complete, so it deletes its copy.
   * Retried at every sync until it is: a package left ready on the server expires
   * after a day, and with it the licence.
   */
  completed: boolean;
}

let opening: Promise<IDBDatabase> | null = null;

function open(): Promise<IDBDatabase> {
  if (!opening) {
    opening = new Promise((resolve, reject) => {
      const req = indexedDB.open(OFFLINE_DB, DB_VERSION);
      req.onupgradeneeded = () => {
        const db = req.result;
        db.createObjectStore(LECTURES, { keyPath: "streamId" });
        db.createObjectStore(KEYS);
        db.createObjectStore(META);
      };
      req.onsuccess = () => resolve(req.result);
      req.onerror = () => {
        opening = null;
        reject(req.error);
      };
    });
  }
  return opening;
}

function run<T>(store: string, mode: IDBTransactionMode, op: (s: IDBObjectStore) => IDBRequest<T>): Promise<T> {
  return open().then(
    (db) =>
      new Promise<T>((resolve, reject) => {
        const tx = db.transaction(store, mode);
        const req = op(tx.objectStore(store));
        tx.oncomplete = () => resolve(req.result);
        tx.onerror = () => reject(tx.error);
        tx.onabort = () => reject(tx.error);
      }),
  );
}

export function listLectures(): Promise<OfflineLectureRecord[]> {
  return run(LECTURES, "readonly", (s) => s.getAll() as IDBRequest<OfflineLectureRecord[]>);
}

export function getLecture(streamId: number): Promise<OfflineLectureRecord | undefined> {
  return run(LECTURES, "readonly", (s) => s.get(streamId) as IDBRequest<OfflineLectureRecord | undefined>);
}

export function putLecture(lecture: OfflineLectureRecord): Promise<void> {
  return run(LECTURES, "readwrite", (s) => s.put(lecture)).then(() => undefined);
}

export function deleteLecture(streamId: number): Promise<void> {
  return run(LECTURES, "readwrite", (s) => s.delete(streamId)).then(() => undefined);
}

/**
 * The key of a package. It is stored as a CryptoKey made non-extractable on import:
 * IndexedDB keeps the object, and neither the page nor anything reading the database
 * can get the bytes back out.
 */
export function getKey(packageId: number): Promise<CryptoKey | undefined> {
  return run(KEYS, "readonly", (s) => s.get(packageId) as IDBRequest<CryptoKey | undefined>);
}

export function putKey(packageId: number, key: CryptoKey): Promise<void> {
  return run(KEYS, "readwrite", (s) => s.put(key, packageId)).then(() => undefined);
}

export function deleteKey(packageId: number): Promise<void> {
  return run(KEYS, "readwrite", (s) => s.delete(packageId)).then(() => undefined);
}

export async function getLastSeen(): Promise<number> {
  return ((await run(META, "readonly", (s) => s.get("lastSeen") as IDBRequest<number | undefined>)) ?? 0);
}

export function putLastSeen(t: number): Promise<void> {
  return run(META, "readwrite", (s) => s.put(t, "lastSeen")).then(() => undefined);
}
