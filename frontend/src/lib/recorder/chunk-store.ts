/**
 * Where a browser recording waits on the lecturer's disk until the server has it.
 *
 * Every chunk is written here before it is uploaded and removed only once the server
 * answered for it, so neither a network outage nor a crashed tab loses more than the
 * chunk being recorded. The OPFS store writes through a worker, the only place a write
 * can be flushed to disk before it is answered.
 */

export type RecordingVersion = "PRES" | "CAM";

/** What is known about one version of a recording, next to its chunks. */
export interface RecordingMeta {
  /** The local name, unique per server recording. */
  key: string;
  courseId: number;
  streamId: number;
  recordingId: number;
  lectureTitle: string;
  version: RecordingVersion;
  mimeType: string;
  /** Where its chunks go; the ticket in it is the upload's only credential. */
  uploadPath: string;
  /** Per segment, how many chunks were recorded. */
  recorded: number[];
  /** Per segment, how long recording was interrupted before it, in milliseconds. */
  gaps?: number[];
  /** How long, in milliseconds, recording was interrupted at the end: never shared again. */
  trailingGap?: number;
  /** Thrown away, but the server was away to hear it: to be told again. */
  discarding?: boolean;
  /** Counts the writes of the meta, so the newer of its two files can be told; see writeMeta. */
  generation: number;
  /** Recording ended; what is left is to upload the rest and finalize. */
  stopped: boolean;
  createdAt: number;
}

export interface StoredChunk {
  segment: number;
  seq: number;
  size: number;
  /** The chunk itself, when it could not be stored: it is uploaded from memory. */
  data?: Blob;
}

export interface ChunkStore {
  /** Resolves once the chunk is on disk. */
  put(key: string, segment: number, seq: number, data: Blob): Promise<void>;
  get(key: string, segment: number, seq: number): Promise<Blob | null>;
  remove(key: string, segment: number, seq: number): Promise<void>;
  /** The chunks stored for a recording, in the order they were recorded. */
  list(key: string): Promise<StoredChunk[]>;
  writeMeta(meta: RecordingMeta): Promise<void>;
  readMetas(): Promise<RecordingMeta[]>;
  /** Removes a recording, chunks and meta. */
  drop(key: string): Promise<void>;
}

export function recordingKey(streamId: number, version: RecordingVersion, recordingId: number): string {
  return `${streamId}-${version}-${recordingId}`;
}

const ROOT = "browser-recordings";
/**
 * The meta is written to these by turns: a write is a truncate and then the bytes, and
 * a crash between the two must not leave the only copy empty. Reading takes the newer
 * of the two that parses.
 */
const META_FILES = ["meta-0.json", "meta-1.json"];

/** Fixed width, so that names sort in recording order. */
const segmentName = (segment: number) => String(segment).padStart(6, "0");
const chunkName = (seq: number) => String(seq).padStart(8, "0");

function byRecordingOrder(a: StoredChunk, b: StoredChunk): number {
  return a.segment - b.segment || a.seq - b.seq;
}

/**
 * Whether this browser has what OPFSChunkStore needs, as far as the page can tell: the
 * sync access handle it writes with exists only inside a worker, so whether writes
 * work is up to OPFSChunkStore.probe.
 */
export function opfsSupported(): boolean {
  return (
    typeof navigator !== "undefined" &&
    typeof navigator.storage?.getDirectory === "function" &&
    typeof Worker !== "undefined"
  );
}

interface WorkerReply {
  id: number;
  error?: string;
}

/**
 * The origin private file system, written through opfs-worker.ts and read here: reads
 * and removals need no flush.
 */
export class OPFSChunkStore implements ChunkStore {
  private worker: Worker | null = null;
  private nextId = 0;
  private waiting = new Map<number, { resolve: () => void; reject: (e: Error) => void }>();

  private send(path: string[], data: ArrayBuffer): Promise<void> {
    if (!this.worker) {
      this.worker = new Worker(new URL("./opfs-worker.ts", import.meta.url), { type: "module" });
      this.worker.onmessage = (event: MessageEvent<WorkerReply>) => {
        const w = this.waiting.get(event.data.id);
        if (!w) return;
        this.waiting.delete(event.data.id);
        if (event.data.error) w.reject(new Error(event.data.error));
        else w.resolve();
      };
    }
    const id = this.nextId++;
    return new Promise((resolve, reject) => {
      this.waiting.set(id, { resolve, reject });
      this.worker!.postMessage({ id, path: [ROOT, ...path], data }, [data]);
    });
  }

  private async dir(path: string[], create = false): Promise<FileSystemDirectoryHandle | null> {
    let dir = await navigator.storage.getDirectory();
    for (const name of [ROOT, ...path]) {
      try {
        dir = await dir.getDirectoryHandle(name, { create });
      } catch {
        return null;
      }
    }
    return dir;
  }

  /** Writes and removes a file the way chunks are written; rejects when that doesn't work. */
  async probe(): Promise<void> {
    await this.send([".probe"], new Uint8Array([1]).buffer);
    const root = await this.dir([]);
    await root?.removeEntry(".probe").catch(() => undefined);
  }

  async put(key: string, segment: number, seq: number, data: Blob): Promise<void> {
    await this.send([key, segmentName(segment), chunkName(seq)], await data.arrayBuffer());
  }

  async get(key: string, segment: number, seq: number): Promise<Blob | null> {
    const dir = await this.dir([key, segmentName(segment)]);
    try {
      return dir ? await (await dir.getFileHandle(chunkName(seq))).getFile() : null;
    } catch {
      return null;
    }
  }

  async remove(key: string, segment: number, seq: number): Promise<void> {
    const dir = await this.dir([key, segmentName(segment)]);
    await dir?.removeEntry(chunkName(seq)).catch(() => undefined);
  }

  async list(key: string): Promise<StoredChunk[]> {
    const dir = await this.dir([key]);
    if (!dir) return [];
    const chunks: StoredChunk[] = [];
    for await (const [segName, segHandle] of dir.entries()) {
      const segment = Number(segName);
      if (segHandle.kind !== "directory" || !Number.isInteger(segment)) continue;
      for await (const [name, handle] of (segHandle as FileSystemDirectoryHandle).entries()) {
        const seq = Number(name);
        if (handle.kind !== "file" || !Number.isInteger(seq)) continue;
        const file = await (handle as FileSystemFileHandle).getFile();
        chunks.push({ segment, seq, size: file.size });
      }
    }
    return chunks.sort(byRecordingOrder);
  }

  async writeMeta(meta: RecordingMeta): Promise<void> {
    meta.generation = (meta.generation ?? 0) + 1;
    const data = new TextEncoder().encode(JSON.stringify(meta));
    await this.send([meta.key, META_FILES[meta.generation % 2]], data.buffer as ArrayBuffer);
  }

  async readMetas(): Promise<RecordingMeta[]> {
    const root = await this.dir([], true);
    if (!root) return [];
    const metas: RecordingMeta[] = [];
    for await (const [, handle] of root.entries()) {
      if (handle.kind !== "directory") continue;
      const newest = await newestMeta(handle as FileSystemDirectoryHandle);
      // A recording whose meta was never written has nothing to resume from.
      if (newest) metas.push(newest);
    }
    return metas.sort((a, b) => a.createdAt - b.createdAt);
  }

  async drop(key: string): Promise<void> {
    const root = await this.dir([]);
    await root?.removeEntry(key, { recursive: true }).catch(() => undefined);
  }
}

async function newestMeta(dir: FileSystemDirectoryHandle): Promise<RecordingMeta | null> {
  let newest: RecordingMeta | null = null;
  for (const name of META_FILES) {
    try {
      const meta = JSON.parse(await (await (await dir.getFileHandle(name)).getFile()).text()) as RecordingMeta;
      if (!newest || meta.generation > newest.generation) newest = meta;
    } catch {
      // Missing, or cut short by a crash: the other one is the meta.
    }
  }
  return newest;
}

/** A store in memory, for tests. */
export class MemoryChunkStore implements ChunkStore {
  readonly chunks = new Map<string, Map<string, Blob>>();
  readonly metas = new Map<string, RecordingMeta>();

  private of(key: string): Map<string, Blob> {
    let m = this.chunks.get(key);
    if (!m) this.chunks.set(key, (m = new Map()));
    return m;
  }

  async put(key: string, segment: number, seq: number, data: Blob): Promise<void> {
    this.of(key).set(`${segment}/${seq}`, data);
  }

  async get(key: string, segment: number, seq: number): Promise<Blob | null> {
    return this.of(key).get(`${segment}/${seq}`) ?? null;
  }

  async remove(key: string, segment: number, seq: number): Promise<void> {
    this.of(key).delete(`${segment}/${seq}`);
  }

  async list(key: string): Promise<StoredChunk[]> {
    return [...this.of(key).entries()]
      .map(([name, blob]) => {
        const [segment, seq] = name.split("/").map(Number);
        return { segment, seq, size: blob.size };
      })
      .sort(byRecordingOrder);
  }

  async writeMeta(meta: RecordingMeta): Promise<void> {
    this.metas.set(meta.key, structuredClone(meta));
  }

  async readMetas(): Promise<RecordingMeta[]> {
    return [...this.metas.values()].map((m) => structuredClone(m));
  }

  async drop(key: string): Promise<void> {
    this.chunks.delete(key);
    this.metas.delete(key);
  }
}
