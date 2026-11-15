/**
 * One lecture being recorded in the browser: the shared screen as the presentation
 * version, the camera as the camera version, the microphone in both, so the versions
 * share their audio as a lecture hall's do.
 *
 * Each version's MediaRecorder hands a chunk every CHUNK_MS, which goes to disk first
 * and then into that version's upload queue. When a track ends (the browser's "Stop
 * sharing", a camera unplugged, the laptop asleep) the version pauses, and sharing
 * again starts a new segment: a new MediaRecorder makes a new file, which the runner
 * joins to the others.
 */

import { abandonBrowserRecording, createBrowserRecording, finalizeBrowserRecording, getBrowserRecording, putChunk } from "./api";
import { recordingKey, type ChunkStore, type RecordingMeta, type RecordingVersion } from "./chunk-store";
import { pickMimeType } from "./mime";
import { UploadQueue, type QueueStatus, type RecordingServer } from "./upload-queue";

export const CHUNK_MS = 2000;

const BITRATES: Record<RecordingVersion, number> = { PRES: 2_500_000, CAM: 1_500_000 };

/** Free space below this gets a warning: about two versions of 90 minutes, all of them waiting. */
export const LOW_STORAGE_BYTES = 3 * 1024 ** 3;

export const server: RecordingServer = {
  putChunk,
  async received(meta) {
    return (await getBrowserRecording(meta.courseId, meta.streamId, meta.recordingId)).received;
  },
  async finalize(meta, expected) {
    await finalizeBrowserRecording(meta.courseId, meta.streamId, meta.recordingId, expected);
  },
  abandon(meta) {
    return abandonBrowserRecording(meta.courseId, meta.streamId, meta.recordingId);
  },
};

/** The lock a tab holds on a recording while it records or uploads it, so another tab leaves it be. */
export const lockName = (key: string) => `browser-recording:${key}`;

export type VersionState = "waiting" | "recording" | "paused" | "ended";

export interface VersionStatus {
  version: RecordingVersion;
  state: VersionState;
  upload: QueueStatus;
}

interface Version {
  version: RecordingVersion;
  /** The video track the version records, null while it waits to be shared again. */
  video: MediaStreamTrack | null;
  meta: RecordingMeta;
  queue: UploadQueue;
  recorder: MediaRecorder | null;
  segment: number;
  seq: number;
  state: VersionState;
  /** Writes of this version's chunks, one after another. */
  writes: Promise<void>;
  releaseLock: () => void;
}

export interface Sources {
  screen: MediaStream | null;
  camera: MediaStream | null;
}

export class RecordingSession {
  private versions: Version[] = [];
  private mic: MediaStreamTrack | null = null;
  private mimeType = "";
  startedAt = 0;
  /** What went wrong storing a chunk, which loses it: the disk is full, most likely. */
  storageError = "";
  /** Thrown away: a chunk still on its way is not kept. */
  private discarded = false;

  constructor(
    private readonly store: ChunkStore,
    private readonly onChange: () => void,
  ) {}

  get status(): VersionStatus[] {
    return this.versions.map((v) => ({ version: v.version, state: v.state, upload: v.queue.current }));
  }

  get recording(): boolean {
    return this.versions.some((v) => v.state === "recording" || v.state === "paused");
  }

  /** Still recording, or chunks still on their way: leaving the page now would stop the upload. */
  get busy(): boolean {
    return this.recording || this.versions.some((v) => !v.queue.over);
  }

  /**
   * Creates the server's recordings and starts recording. The microphone is taken
   * from the camera's stream, or the screen's when there is no camera.
   */
  async start(courseId: number, streamId: number, lectureTitle: string, sources: Sources): Promise<void> {
    const mime = pickMimeType((t) => MediaRecorder.isTypeSupported(t));
    if (!mime) throw new Error("This browser cannot record video.");
    this.mimeType = mime;
    this.mic = sources.camera?.getAudioTracks()[0] ?? sources.screen?.getAudioTracks()[0] ?? null;

    const wanted: Array<[RecordingVersion, MediaStreamTrack | undefined]> = [
      ["PRES", sources.screen?.getVideoTracks()[0]],
      ["CAM", sources.camera?.getVideoTracks()[0]],
    ];
    for (const [version, video] of wanted) {
      if (!video) continue;
      const created = await createBrowserRecording(courseId, streamId, version, mime);
      const id = created.recording!.id;
      const meta: RecordingMeta = {
        key: recordingKey(streamId, version, id),
        courseId,
        streamId,
        recordingId: id,
        lectureTitle,
        version,
        mimeType: mime,
        uploadPath: created.uploadPath,
        recorded: [],
        stopped: false,
        createdAt: Date.now(),
      };
      await this.store.writeMeta(meta);
      const v: Version = {
        version,
        video,
        meta,
        queue: new UploadQueue({ store: this.store, meta, server, onChange: () => this.onChange() }),
        recorder: null,
        segment: 0,
        seq: 0,
        state: "waiting",
        writes: Promise.resolve(),
        releaseLock: await holdLock(meta.key),
      };
      this.versions.push(v);
    }
    this.startedAt = Date.now();
    for (const v of this.versions) this.record(v);
    this.onChange();
  }

  private record(v: Version): void {
    if (!v.video) return;
    const tracks = [v.video, ...(this.mic && this.mic.readyState === "live" ? [this.mic] : [])];
    const recorder = new MediaRecorder(new MediaStream(tracks), {
      mimeType: this.mimeType,
      videoBitsPerSecond: BITRATES[v.version],
      audioBitsPerSecond: 128_000,
    });
    const segment = v.segment;
    recorder.ondataavailable = (event) => {
      if (!event.data.size) return;
      const seq = v.seq++;
      v.writes = v.writes.then(() => this.keep(v, segment, seq, event.data));
    };
    v.video.onended = () => this.trackEnded(v);
    v.recorder = recorder;
    v.state = "recording";
    recorder.start(CHUNK_MS);
  }

  /** A chunk to disk, then to the queue. Only once it is stored is it counted as recorded. */
  private async keep(v: Version, segment: number, seq: number, data: Blob): Promise<void> {
    if (this.discarded) return;
    try {
      await this.store.put(v.meta.key, segment, seq, data);
      v.meta.recorded[segment] = seq + 1;
      // Holes stay 0 rather than null in the JSON.
      for (let i = 0; i < segment; i++) v.meta.recorded[i] ??= 0;
      await this.store.writeMeta(v.meta);
    } catch (err) {
      this.storageError = err instanceof Error ? err.message : String(err);
      this.onChange();
      return;
    }
    v.queue.enqueue({ segment, seq, size: data.size });
  }

  private trackEnded(v: Version): void {
    if (v.state !== "recording" && v.state !== "paused") return;
    this.stopRecorder(v);
    v.video = null;
    v.state = "waiting";
    this.onChange();
  }

  private stopRecorder(v: Version): void {
    const recorder = v.recorder;
    v.recorder = null;
    if (recorder && recorder.state !== "inactive") recorder.stop();
  }

  /** Goes on recording a version whose track ended, with a new track, as a new segment. */
  resumeWith(version: RecordingVersion, video: MediaStreamTrack): void {
    const v = this.versions.find((x) => x.version === version);
    if (!v || v.state !== "waiting") return;
    v.segment++;
    v.seq = 0;
    v.video = video;
    this.record(v);
    this.onChange();
  }

  pause(): void {
    for (const v of this.versions) {
      if (v.state === "recording") {
        v.recorder?.pause();
        v.state = "paused";
      }
    }
    this.onChange();
  }

  resume(): void {
    for (const v of this.versions) {
      if (v.state === "paused") {
        v.recorder?.resume();
        v.state = "recording";
      }
    }
    this.onChange();
  }

  /** Stops recording, and resolves once every version is finalized or refused. */
  async stop(): Promise<void> {
    await Promise.all(
      this.versions.map(async (v) => {
        await this.halt(v);
        v.meta.stopped = true;
        await this.store.writeMeta(v.meta).catch(() => undefined);
      }),
    );
    this.mic?.stop();
    this.onChange();
    await Promise.all(this.versions.map((v) => v.queue.finish()));
    for (const v of this.versions) v.releaseLock();
    this.onChange();
  }

  /** Stops and throws everything away, here and on the server. */
  async discard(): Promise<void> {
    this.discarded = true;
    for (const v of this.versions) v.queue.stop();
    this.mic?.stop();
    await Promise.all(
      this.versions.map(async (v) => {
        // Only once the recorder has handed out its last chunk: written after the drop,
        // it would bring the recording back as unfinished.
        await this.halt(v);
        await abandonBrowserRecording(v.meta.courseId, v.meta.streamId, v.meta.recordingId).catch(() => undefined);
        await this.store.drop(v.meta.key);
        v.releaseLock();
      }),
    );
    this.onChange();
  }

  /**
   * Ends a version's recording and waits for its last chunk, which the recorder hands
   * out after it is told to stop and before its stop event, to be written.
   */
  private async halt(v: Version): Promise<void> {
    const recorder = v.recorder;
    if (recorder && recorder.state !== "inactive") {
      await new Promise<void>((resolve) => {
        recorder.addEventListener("stop", () => resolve(), { once: true });
        this.stopRecorder(v);
      });
    }
    v.video?.stop();
    v.state = "ended";
    await v.writes;
  }

  /** The versions' metas, for saving a copy. */
  get metas(): RecordingMeta[] {
    return this.versions.map((v) => v.meta);
  }
}

/**
 * Holds the Web Lock on a recording until the answer is called. Without Web Locks
 * (very old browsers) nothing is held; recovery in a second tab then only sends
 * chunks twice, which the server takes.
 */
export async function holdLock(key: string): Promise<() => void> {
  if (!navigator.locks) return () => undefined;
  let release!: () => void;
  const held = new Promise<void>((resolve) => (release = resolve));
  await new Promise<void>((acquired) => {
    void navigator.locks.request(lockName(key), () => {
      acquired();
      return held;
    });
  });
  return release;
}

/** Takes the lock on a recording only if no other tab holds it. Null when one does. */
export async function tryLock(key: string): Promise<(() => void) | null> {
  if (!navigator.locks) return () => undefined;
  let release!: () => void;
  const held = new Promise<void>((resolve) => (release = resolve));
  const got = await new Promise<boolean>((answer) => {
    void navigator.locks.request(lockName(key), { ifAvailable: true }, (lock) => {
      answer(lock !== null);
      return lock ? held : undefined;
    });
  });
  return got ? release : null;
}

/** How much the browser lets this site store, after asking to keep it. Null when it doesn't say. */
export async function freeStorage(): Promise<number | null> {
  try {
    await navigator.storage.persist?.();
    const { quota, usage } = await navigator.storage.estimate();
    return quota === undefined ? null : quota - (usage ?? 0);
  } catch {
    return null;
  }
}
