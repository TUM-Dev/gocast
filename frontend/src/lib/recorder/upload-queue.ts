/**
 * Uploads one version of a browser recording from the chunk store, in recording order,
 * while it is being recorded and after. A chunk leaves the store only once the server
 * has answered for it. Anything that may be the server or the network being away is
 * retried for as long as it takes; only the server itself refusing a chunk stops it.
 */

import { ApiError } from "@/lib/api";
import type { ChunkStore, RecordingMeta, StoredChunk } from "./chunk-store";

/** What the queue needs of the server, so tests can stand in for it. */
export interface RecordingServer {
  /** gapMs, with a segment's first chunk, is how long recording was interrupted before it. */
  putChunk(uploadPath: string, segment: number, seq: number, data: Blob, gapMs?: number): Promise<Response>;
  /** Per segment, how many chunks from 0 on the server has. */
  received(meta: RecordingMeta): Promise<number[]>;
  finalize(meta: RecordingMeta, expected: number[]): Promise<void>;
  abandon(meta: RecordingMeta): Promise<void>;
}

export type ChunkOutcome = { kind: "ok" } | { kind: "retry" } | { kind: "refused"; status: number; message: string };

/**
 * What a chunk's response means. A 5xx, 408 and 429 are worth waiting out. So is a
 * refusal that is not the server's own JSON: while the server is down, Traefik
 * answers a plain-text 404, and a proxy may answer with a page of its own. Only an
 * error from the server itself stops the upload.
 */
export async function chunkOutcome(res: Response): Promise<ChunkOutcome> {
  if (res.ok) return { kind: "ok" };
  if (res.status >= 500 || res.status === 408 || res.status === 429) return { kind: "retry" };
  const message = await serverErrorMessage(res);
  if (message === null) return { kind: "retry" };
  return { kind: "refused", status: res.status, message };
}

async function serverErrorMessage(res: Response): Promise<string | null> {
  if (!res.headers.get("Content-Type")?.includes("application/json")) return null;
  try {
    const body = (await res.json()) as { code?: unknown; message?: unknown };
    return typeof body.code === "number" && typeof body.message === "string" ? body.message : null;
  } catch {
    return null;
  }
}

/** Capped so the queue catches up soon after the server is back. */
export const MAX_BACKOFF_MS = 15_000;

export function backoff(attempt: number): number {
  return Math.min(1000 * 2 ** attempt, MAX_BACKOFF_MS);
}

export type QueueState =
  /** Nothing to send right now. */
  | "idle"
  | "uploading"
  /** The server or the network is away; chunks wait on disk. */
  | "retrying"
  /** The server refused the recording: see error. */
  | "failed"
  /** Finalized: a runner converts it now. */
  | "done";

export interface QueueStatus {
  state: QueueState;
  pendingChunks: number;
  pendingBytes: number;
  error: string;
  /** Chunks that were recorded but never reached the server, when it was finalized without them. */
  missing: boolean;
  /** Nothing was recorded, so there was nothing to finish: the recording was thrown away. */
  empty: boolean;
}

export interface UploadQueueOptions {
  store: ChunkStore;
  meta: RecordingMeta;
  server: RecordingServer;
  onChange?: (status: QueueStatus) => void;
  sleep?: (ms: number) => Promise<void>;
}

const realSleep = (ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms));

export class UploadQueue {
  private pending: StoredChunk[] = [];
  private running = false;
  private stopped = false;
  private attempt = 0;
  private status: QueueStatus = { state: "idle", pendingChunks: 0, pendingBytes: 0, error: "", missing: false, empty: false };
  /** Resolved by run() when it stops for a reason other than more chunks to come. */
  private settled: Array<() => void> = [];

  readonly meta: RecordingMeta;
  private readonly store: ChunkStore;
  private readonly server: RecordingServer;
  private readonly onChange: (status: QueueStatus) => void;
  private readonly sleep: (ms: number) => Promise<void>;

  constructor(opts: UploadQueueOptions) {
    this.store = opts.store;
    this.meta = opts.meta;
    this.server = opts.server;
    this.onChange = opts.onChange ?? (() => undefined);
    this.sleep = opts.sleep ?? realSleep;
  }

  get current(): QueueStatus {
    return { ...this.status };
  }

  private set(change: Partial<QueueStatus>): void {
    this.status = {
      ...this.status,
      ...change,
      pendingChunks: this.pending.length,
      pendingBytes: this.pending.reduce((sum, c) => sum + c.size, 0),
    };
    this.onChange(this.current);
  }

  /** A chunk that was just stored. */
  enqueue(chunk: StoredChunk): void {
    this.pending.push(chunk);
    this.set({});
    void this.run();
  }

  /**
   * Picks up what the store holds, after a reload or a crash. What the server has
   * already is removed without sending it again: its answer was lost on the way.
   */
  async resume(): Promise<void> {
    const stored = await this.store.list(this.meta.key);
    let received: number[] = [];
    try {
      received = await this.server.received(this.meta);
    } catch {
      // The server is away: sending everything again is harmless, chunks replace themselves.
    }
    this.pending = [];
    for (const c of stored) {
      if (c.seq >= (received[c.segment] ?? 0)) this.pending.push(c);
      else if (c.seq !== 0) await this.store.remove(this.meta.key, c.segment, c.seq);
    }
    // The counts may lag behind: a crash between storing a chunk and writing the meta
    // leaves the chunk uncounted, and finalizing would leave it out, or take a recording
    // with chunks for an empty one and throw it away.
    const counts = [...this.meta.recorded];
    for (const c of stored) counts[c.segment] = Math.max(counts[c.segment] ?? 0, c.seq + 1);
    received.forEach((n, segment) => (counts[segment] = Math.max(counts[segment] ?? 0, n)));
    this.meta.recorded = Array.from(counts, (n) => n ?? 0);
    this.set({});
    void this.run();
  }

  /**
   * Recording ended: finalize once everything is sent. Resolves when the queue is
   * over, at once if it is already: refused, say, while it was being recorded.
   */
  finish(): Promise<void> {
    this.meta.stopped = true;
    if (this.over) return Promise.resolve();
    const done = new Promise<void>((resolve) => this.settled.push(resolve));
    void this.run();
    return done;
  }

  /** Stops sending for good, e.g. when the recording is thrown away. */
  stop(): void {
    this.stopped = true;
    // A send under way finds out when it returns; whoever waits needn't.
    this.settle();
    this.set({});
  }

  /** Finalized, refused or stopped: nothing more will happen. */
  get over(): boolean {
    return this.stopped || this.status.state === "done" || this.status.state === "failed";
  }

  private async run(): Promise<void> {
    if (this.running || this.over) return;
    this.running = true;
    try {
      while (!this.stopped && !this.over) {
        const next = this.pending[0];
        if (!next) {
          if (!this.meta.stopped) break;
          await this.finalize();
          continue;
        }
        await this.send(next);
      }
    } finally {
      this.running = false;
      if (!this.pending.length || this.over) {
        if (!this.over && !this.meta.stopped) this.set({ state: "idle" });
        if (this.over || this.stopped) this.settle();
      }
    }
  }

  private settle(): void {
    const waiting = this.settled;
    this.settled = [];
    waiting.forEach((resolve) => resolve());
  }

  private async send(chunk: StoredChunk): Promise<void> {
    const data = chunk.data ?? (await this.store.get(this.meta.key, chunk.segment, chunk.seq));
    if (!data) {
      // Gone from disk (storage cleared): the server will be short of it, and says so on finalize.
      this.pending.shift();
      this.set({});
      return;
    }
    let outcome: ChunkOutcome;
    try {
      const gap = chunk.seq === 0 ? this.meta.gaps?.[chunk.segment] : undefined;
      outcome = await chunkOutcome(await this.server.putChunk(this.meta.uploadPath, chunk.segment, chunk.seq, data, gap));
    } catch {
      outcome = { kind: "retry" };
    }
    switch (outcome.kind) {
      case "ok":
        this.attempt = 0;
        // A segment's first chunk holds the file's header, which a copy saved from what is
        // left here needs to play: it stays until the recording is finalized.
        if (chunk.seq !== 0) await this.store.remove(this.meta.key, chunk.segment, chunk.seq);
        this.pending.shift();
        this.set({ state: "uploading", error: "" });
        return;
      case "retry":
        this.set({ state: "retrying" });
        await this.sleep(backoff(this.attempt++));
        return;
      case "refused":
        this.set({ state: "failed", error: refusedMessage(outcome.status, outcome.message) });
        return;
    }
  }

  private async finalize(): Promise<void> {
    if (!this.meta.recorded.some((n) => n > 0)) {
      // Stopped before the recorder handed out a chunk: the server would refuse to
      // finish a recording of nothing, and keep asking for one forever.
      try {
        await this.server.abandon(this.meta);
      } catch (err) {
        // Gone already is as good as thrown away.
        if (!(isRefusal(err) && err.status === 404)) {
          await this.finalizeFailed(err);
          return;
        }
      }
      await this.store.drop(this.meta.key);
      this.set({ state: "done", error: "", empty: true });
      return;
    }
    try {
      await this.server.finalize(this.meta, this.meta.recorded);
    } catch (err) {
      if (isRefusal(err) && err.status === 409) {
        // The server is short of chunks. Those still here are sent first; only when
        // none are is the recording finished with what the server has.
        let unsent: StoredChunk[];
        try {
          unsent = await this.unsent();
        } catch (again) {
          await this.finalizeFailed(again);
          return;
        }
        if (unsent.length) {
          this.pending.push(...unsent);
          this.set({});
          return;
        }
        try {
          await this.server.finalize(this.meta, []);
          await this.done(true);
        } catch (again) {
          await this.finalizeFailed(again);
        }
        return;
      }
      await this.finalizeFailed(err);
      return;
    }
    await this.done(false);
  }

  /** The chunks sent again because finalizing found the server short, once each. */
  private resent = new Set<string>();

  /**
   * The chunks stored here that the server does not count, not sent again already. The
   * server counts up to the first chunk it lacks: when that one is gone here too, the
   * ones after it are not counted however often they are sent.
   */
  private async unsent(): Promise<StoredChunk[]> {
    const received = await this.server.received(this.meta);
    const stored = await this.store.list(this.meta.key);
    const unsent = stored.filter(
      (c) => c.seq >= (received[c.segment] ?? 0) && !this.resent.has(`${c.segment}/${c.seq}`),
    );
    for (const c of unsent) this.resent.add(`${c.segment}/${c.seq}`);
    return unsent;
  }

  private async done(missing: boolean): Promise<void> {
    await this.store.drop(this.meta.key);
    this.set({ state: "done", error: "", missing });
  }

  private async finalizeFailed(err: unknown): Promise<void> {
    if (isRefusal(err)) {
      this.set({
        state: "failed",
        error: err.isUnauthenticated
          ? "Your session expired. Sign in again to finish the recording; it is kept in this browser until then."
          : `The recording could not be finished: ${err.message}.`,
      });
      return;
    }
    this.set({ state: "retrying" });
    await this.sleep(backoff(this.attempt++));
  }
}

/**
 * Whether an API call's error is the server refusing it, rather than the server or
 * the network being away, which is worth waiting out like a chunk's.
 */
export function isRefusal(err: unknown): err is ApiError {
  return (
    err instanceof ApiError && err.fromServer && err.status < 500 && err.status !== 408 && err.status !== 429
  );
}

function refusedMessage(status: number, message: string): string {
  if (status === 409) {
    return "The server finished this recording already, without what is still in this browser. Save a copy to keep it.";
  }
  if (status === 404) {
    return "The server no longer knows this recording; it was thrown away. Save a copy to keep what is in this browser.";
  }
  return `The server refused part of the recording: ${message}.`;
}
