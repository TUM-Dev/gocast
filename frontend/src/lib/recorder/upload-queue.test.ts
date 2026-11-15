import { describe, expect, it } from "vitest";

import { ApiError } from "@/lib/api";
import { MemoryChunkStore, recordingKey, type RecordingMeta } from "./chunk-store";
import { MAX_BACKOFF_MS, UploadQueue, backoff, chunkOutcome, type RecordingServer } from "./upload-queue";

function meta(): RecordingMeta {
  return {
    key: recordingKey(7, "PRES", 3),
    courseId: 1,
    streamId: 7,
    recordingId: 3,
    lectureTitle: "Lecture",
    version: "PRES",
    mimeType: "video/webm",
    uploadPath: "/api/v2/browser-recordings/ticket",
    recorded: [],
    stopped: false,
    createdAt: 0,
    generation: 0,
  };
}

const json = (status: number, body: unknown) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
const ok = () => json(200, {});

/** A server that answers chunks from `answers` in turn, then with 200, and keeps what it took. */
class FakeServer implements RecordingServer {
  received_: number[] = [];
  taken: string[] = [];
  finalized: number[][] = [];
  answers: Array<() => Response | Promise<Response>> = [];
  finalizeErrors: unknown[] = [];

  gaps: Array<[number, number | undefined]> = [];

  async putChunk(_path: string, segment: number, seq: number, data: Blob, gapMs?: number): Promise<Response> {
    this.gaps.push([segment, gapMs]);
    const answer = this.answers.shift();
    const res = answer ? await answer() : ok();
    if (res.ok) {
      this.taken.push(`${segment}/${seq}=${await data.text()}`);
      if (seq === (this.received_[segment] ?? 0)) this.received_[segment] = seq + 1;
    }
    return res;
  }

  async received(): Promise<number[]> {
    return [...this.received_];
  }

  async finalize(_meta: RecordingMeta, expected: number[]): Promise<void> {
    const err = this.finalizeErrors.shift();
    if (err) throw err;
    this.finalized.push(expected);
  }

  abandoned = 0;
  async abandon(): Promise<void> {
    this.abandoned++;
  }
}

const noSleep = { sleep: async () => undefined };

async function record(store: MemoryChunkStore, m: RecordingMeta, queue: UploadQueue, segment: number, seq: number, body: string) {
  await store.put(m.key, segment, seq, new Blob([body]));
  m.recorded[segment] = seq + 1;
  queue.enqueue({ segment, seq, size: body.length });
}

describe("chunkOutcome", () => {
  it("takes a 200", async () => {
    expect(await chunkOutcome(ok())).toEqual({ kind: "ok" });
  });

  it.each([500, 502, 503, 504, 408, 429])("waits out a %d", async (status) => {
    expect(await chunkOutcome(new Response("", { status }))).toEqual({ kind: "retry" });
  });

  it("waits out Traefik's plain-text 404 while the server is down", async () => {
    const res = new Response("404 page not found", { status: 404, headers: { "Content-Type": "text/plain" } });
    expect(await chunkOutcome(res)).toEqual({ kind: "retry" });
  });

  it("waits out JSON that is not the server's error", async () => {
    expect(await chunkOutcome(json(404, { error: "nope" }))).toEqual({ kind: "retry" });
  });

  it("stops on the server's own refusal", async () => {
    expect(await chunkOutcome(json(409, { code: 9, message: "the recording is finalized" }))).toEqual({
      kind: "refused",
      status: 409,
      message: "the recording is finalized",
    });
  });
});

describe("backoff", () => {
  it("doubles up to its cap", () => {
    expect([0, 1, 2, 3, 4, 10].map(backoff)).toEqual([1000, 2000, 4000, 8000, MAX_BACKOFF_MS, MAX_BACKOFF_MS]);
  });
});

describe("UploadQueue", () => {
  it("sends chunks in order, removes each once taken, then finalizes with what was recorded", async () => {
    const store = new MemoryChunkStore();
    const server = new FakeServer();
    const m = meta();
    await store.writeMeta(m);
    const queue = new UploadQueue({ store, meta: m, server, ...noSleep });

    await record(store, m, queue, 0, 0, "a");
    await record(store, m, queue, 0, 1, "b");
    await record(store, m, queue, 1, 0, "x");
    await queue.finish();

    expect(server.taken).toEqual(["0/0=a", "0/1=b", "1/0=x"]);
    expect(server.finalized).toEqual([[2, 1]]);
    expect(queue.current.state).toBe("done");
    expect(await store.list(m.key)).toEqual([]);
    expect(store.metas.has(m.key)).toBe(false);
  });

  it("keeps a chunk through an outage and sends it once the server is back", async () => {
    const store = new MemoryChunkStore();
    const server = new FakeServer();
    const states: string[] = [];
    const m = meta();
    const queue = new UploadQueue({ store, meta: m, server, ...noSleep, onChange: (s) => states.push(s.state) });

    const down = () => {
      throw new TypeError("Failed to fetch");
    };
    const traefik = () => new Response("404 page not found", { status: 404 });
    server.answers = [down, () => new Response("", { status: 502 }), traefik, down];

    await record(store, m, queue, 0, 0, "a");
    await queue.finish();

    expect(server.taken).toEqual(["0/0=a"]);
    expect(states).toContain("retrying");
    expect(queue.current.state).toBe("done");
  });

  it("does not remove a chunk the server has not answered for", async () => {
    const store = new MemoryChunkStore();
    const server = new FakeServer();
    const m = meta();
    let release!: () => void;
    server.answers = [() => new Promise<Response>((resolve) => (release = () => resolve(ok())))];
    const queue = new UploadQueue({ store, meta: m, server, ...noSleep });

    await record(store, m, queue, 0, 0, "a");
    await Promise.resolve();
    expect(await store.get(m.key, 0, 0)).not.toBeNull();
    expect(queue.current.pendingBytes).toBe(1);

    const finished = queue.finish();
    release();
    await finished;
    expect(await store.get(m.key, 0, 0)).toBeNull();
  });

  it("keeps a segment's first chunk, its header, until finalized", async () => {
    const store = new MemoryChunkStore();
    const server = new FakeServer();
    const m = meta();
    const queue = new UploadQueue({ store, meta: m, server, ...noSleep });

    await record(store, m, queue, 0, 0, "a");
    await record(store, m, queue, 0, 1, "b");
    await new Promise((resolve) => setTimeout(resolve));
    expect(server.taken).toEqual(["0/0=a", "0/1=b"]);
    expect((await store.list(m.key)).map((c) => c.seq)).toEqual([0]);

    await queue.finish();
    expect(await store.list(m.key)).toEqual([]);
  });

  it("stops when the server refuses the recording, keeping the chunks", async () => {
    const store = new MemoryChunkStore();
    const server = new FakeServer();
    const m = meta();
    server.answers = [() => json(409, { code: 9, message: "the recording is finalized" })];
    const queue = new UploadQueue({ store, meta: m, server, ...noSleep });

    await record(store, m, queue, 0, 0, "a");
    await queue.finish();

    expect(queue.current.state).toBe("failed");
    expect(queue.current.error).toMatch(/Save a copy/);
    expect(await store.get(m.key, 0, 0)).not.toBeNull();
    expect(server.finalized).toEqual([]);
  });

  it("finishes at once when the server refused the recording before", async () => {
    const store = new MemoryChunkStore();
    const server = new FakeServer();
    const m = meta();
    server.answers = [() => json(409, { code: 9, message: "the recording is finalized" })];
    const queue = new UploadQueue({ store, meta: m, server, ...noSleep });

    await record(store, m, queue, 0, 0, "a");
    await new Promise((resolve) => setTimeout(resolve));
    expect(queue.current.state).toBe("failed");

    // Stopping the recording now must not wait for an upload that is over.
    await queue.finish();
    expect(queue.over).toBe(true);
  });

  it("is over once stopped, and lets go of whoever waits", async () => {
    const store = new MemoryChunkStore();
    const server = new FakeServer();
    const m = meta();
    server.answers = [() => new Promise<Response>(() => undefined)]; // a request that never ends
    const queue = new UploadQueue({ store, meta: m, server, ...noSleep });

    await record(store, m, queue, 0, 0, "a");
    const finished = queue.finish();
    queue.stop();
    await finished;
    expect(queue.over).toBe(true);
    expect(await queue.finish().then(() => "resolved")).toBe("resolved");
  });

  it("resumes after a reload without sending what the server has", async () => {
    const store = new MemoryChunkStore();
    const server = new FakeServer();
    const m = meta();
    m.recorded = [3];
    m.stopped = true;
    for (const [seq, body] of ["a", "b", "c"].entries()) await store.put(m.key, 0, seq, new Blob([body]));
    // The answer for chunk 0 was lost when the tab crashed.
    server.received_ = [1];

    const queue = new UploadQueue({ store, meta: m, server, ...noSleep });
    await queue.resume();
    await queue.finish();

    expect(server.taken).toEqual(["0/1=b", "0/2=c"]);
    expect(server.finalized).toEqual([[3]]);
  });

  it("resumes while the server is away by sending everything again", async () => {
    const store = new MemoryChunkStore();
    const server = new FakeServer();
    server.received = async () => {
      throw new TypeError("Failed to fetch");
    };
    const m = meta();
    m.recorded = [1];
    m.stopped = true;
    await store.put(m.key, 0, 0, new Blob(["a"]));

    const queue = new UploadQueue({ store, meta: m, server, ...noSleep });
    await queue.resume();
    await queue.finish();
    expect(server.taken).toEqual(["0/0=a"]);
  });

  it("finalizes with what arrived when chunks were lost from this browser", async () => {
    const store = new MemoryChunkStore();
    const server = new FakeServer();
    server.finalizeErrors = [new ApiError(409, "chunks of the recording are still missing", true)];
    const m = meta();
    const queue = new UploadQueue({ store, meta: m, server, ...noSleep });

    await record(store, m, queue, 0, 0, "a");
    m.recorded = [2]; // the second chunk never made it to disk
    await queue.finish();

    expect(server.finalized).toEqual([[]]);
    expect(queue.current).toMatchObject({ state: "done", missing: true });
  });

  it("retries finalizing through an outage", async () => {
    const store = new MemoryChunkStore();
    const server = new FakeServer();
    server.finalizeErrors = [new TypeError("Failed to fetch"), new ApiError(503, "unavailable")];
    const m = meta();
    const queue = new UploadQueue({ store, meta: m, server, ...noSleep });

    await record(store, m, queue, 0, 0, "a");
    await queue.finish();
    expect(server.finalized).toEqual([[1]]);
    expect(queue.current.state).toBe("done");
  });

  it("retries finalizing through a proxy's 404, which is not the server's", async () => {
    const store = new MemoryChunkStore();
    const server = new FakeServer();
    server.finalizeErrors = [new ApiError(404, "Not Found"), new ApiError(502, "Bad Gateway")];
    const m = meta();
    const queue = new UploadQueue({ store, meta: m, server, ...noSleep });

    await record(store, m, queue, 0, 0, "a");
    await queue.finish();
    expect(server.finalized).toEqual([[1]]);
    expect(queue.current.state).toBe("done");
  });

  it("sends the gap before a segment with its first chunk", async () => {
    const store = new MemoryChunkStore();
    const server = new FakeServer();
    const m = meta();
    m.gaps = [0, 1500];
    const queue = new UploadQueue({ store, meta: m, server, ...noSleep });

    await record(store, m, queue, 0, 0, "a");
    await record(store, m, queue, 1, 0, "x");
    await record(store, m, queue, 1, 1, "y");
    await queue.finish();
    expect(server.gaps).toEqual([
      [0, 0],
      [1, 1500],
      [1, undefined],
    ]);
  });

  it("throws away a recording of nothing instead of finalizing it", async () => {
    const store = new MemoryChunkStore();
    const server = new FakeServer();
    const m = meta();
    await store.writeMeta(m);
    const queue = new UploadQueue({ store, meta: m, server, ...noSleep });

    await queue.finish();
    expect(server.finalized).toEqual([]);
    expect(server.abandoned).toBe(1);
    expect(queue.current).toMatchObject({ state: "done", empty: true });
    expect(store.metas.has(m.key)).toBe(false);
  });

  it("keeps the recording when the session expired before finalizing", async () => {
    const store = new MemoryChunkStore();
    const server = new FakeServer();
    server.finalizeErrors = [new ApiError(401, "unauthenticated", true)];
    const m = meta();
    await store.writeMeta(m);
    const queue = new UploadQueue({ store, meta: m, server, ...noSleep });

    await record(store, m, queue, 0, 0, "a");
    await queue.finish();
    expect(queue.current.state).toBe("failed");
    expect(queue.current.error).toMatch(/Sign in again/);
    expect(store.metas.has(m.key)).toBe(true);
  });
});
