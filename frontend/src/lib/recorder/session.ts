/**
 * One lecture being recorded in the browser: the shared screen as the presentation
 * version, the camera as the camera version, the microphone in both, so the versions
 * share their audio as a lecture hall's do.
 *
 * Each version's MediaRecorder hands a chunk every CHUNK_MS, which goes to disk first
 * and then into that version's upload queue. When a track ends (the browser's "Stop
 * sharing", a camera unplugged, the laptop asleep) the version pauses, and sharing
 * again starts a new segment: a new MediaRecorder makes a new file, which the runner
 * joins to the others. The other version goes on meanwhile, so the time a version
 * missed is sent along with its next segment, and the runner fills it with black and
 * silence: both versions keep one timeline, as split view plays them.
 *
 * The audio both record is not the microphone's track itself but an AudioContext's
 * output that the microphone feeds. It is silent while there is no microphone, so
 * every segment has audio, and a microphone turned on again feeds the same track
 * without the recorders noticing.
 */

import { abandonBrowserRecording, createBrowserRecording, finalizeBrowserRecording, getBrowserRecording, putChunk } from "./api";
import { recordingKey, type ChunkStore, type RecordingMeta, type RecordingVersion } from "./chunk-store";
import { pickMimeType } from "./mime";
import { UploadQueue, backoff, isRefusal, type QueueStatus, type RecordingServer } from "./upload-queue";

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
    await finalizeBrowserRecording(meta.courseId, meta.streamId, meta.recordingId, expected, meta.trailingGap);
  },
  abandon(meta) {
    return abandonBrowserRecording(meta.courseId, meta.streamId, meta.recordingId);
  },
};

/** Whether the recording has a microphone: none was given, or the one given stopped. */
export type MicState = "on" | "lost" | "none";

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
  /** When, in recorded time, the version's track ended; see activeMs. */
  endedAt: number;
  /** Writes of this version's chunks, one after another. */
  writes: Promise<void>;
  releaseLock: () => void;
}

export interface Sources {
  screen: MediaStream | null;
  camera: MediaStream | null;
  mic: MediaStreamTrack | null;
}

export class RecordingSession {
  private versions: Version[] = [];
  private mic: MediaStreamTrack | null = null;
  micState: MicState = "none";
  private audio: AudioContext | null = null;
  private audioOut: MediaStreamAudioDestinationNode | null = null;
  private micIn: MediaStreamAudioSourceNode | null = null;
  private mimeType = "";
  startedAt = 0;
  /** Paused time so far, and since when it is paused now: not part of the recording. */
  private pausedTotal = 0;
  private pausedSince = 0;
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

  /** Not stopped yet: recording, paused, or waiting for a source to be shared again. */
  get recording(): boolean {
    return this.versions.some((v) => v.state !== "ended");
  }

  /** Starting: asking for the microphone, creating the lecture and its recordings. */
  private starting = false;

  /**
   * Starting, still recording, or chunks still on their way: leaving the page now would
   * stop the upload, and closing the recorder would lose its controls.
   */
  get busy(): boolean {
    return this.starting || this.recording || this.versions.some((v) => !v.queue.over);
  }

  /** Marks the time from the click until recording runs, or is given up, as busy. */
  setStarting(starting: boolean): void {
    this.starting = starting;
    this.onChange();
  }

  /**
   * Sets up the audio the recorders take. To be called right on the user's click: a
   * browser may not let an AudioContext run that was made after awaiting something.
   */
  prepareAudio(): void {
    if (this.audio) return;
    this.audio = new AudioContext();
    this.audioOut = this.audio.createMediaStreamDestination();
    void this.audio.resume();
  }

  /** Creates the server's recordings and starts recording. */
  async start(courseId: number, streamId: number, lectureTitle: string, sources: Sources): Promise<void> {
    const mime = pickMimeType((t) => MediaRecorder.isTypeSupported(t));
    if (!mime) throw new Error("This browser cannot record video.");
    this.mimeType = mime;
    this.prepareAudio();
    this.setMic(sources.mic);

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
        gaps: [],
        stopped: false,
        createdAt: Date.now(),
        generation: 0,
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
        endedAt: 0,
        writes: Promise.resolve(),
        releaseLock: await holdLock(meta.key),
      };
      this.versions.push(v);
    }
    this.startedAt = Date.now();
    for (const v of this.versions) this.record(v);
    this.onChange();
  }

  /** Feeds the recording's audio from a microphone, or nothing, which makes it silence. */
  private setMic(track: MediaStreamTrack | null): void {
    this.micIn?.disconnect();
    this.micIn = null;
    this.mic = track;
    if (!track || track.readyState !== "live" || !this.audio || !this.audioOut) {
      this.micState = track ? "lost" : "none";
      return;
    }
    this.micIn = this.audio.createMediaStreamSource(new MediaStream([track]));
    this.micIn.connect(this.audioOut);
    this.micState = "on";
    track.addEventListener("ended", () => {
      if (this.mic !== track) return;
      this.micIn?.disconnect();
      this.micIn = null;
      this.micState = "lost";
      this.onChange();
    });
  }

  /** A microphone turned on again, or another one: the recordings go on with it. */
  replaceMic(track: MediaStreamTrack): void {
    const previous = this.mic;
    this.setMic(track);
    if (previous && previous !== track) previous.stop();
    this.onChange();
  }

  /** How long has been recorded, in milliseconds: the time since start, less pauses. */
  private activeMs(): number {
    const pausedNow = this.pausedSince ? Date.now() - this.pausedSince : 0;
    return Date.now() - this.startedAt - this.pausedTotal - pausedNow;
  }

  private record(v: Version): void {
    if (!v.video) return;
    const audio = this.audioOut?.stream.getAudioTracks()[0];
    const tracks = [v.video, ...(audio ? [audio] : [])];
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
    v.endedAt = this.activeMs();
    this.onChange();
  }

  private stopRecorder(v: Version): void {
    const recorder = v.recorder;
    v.recorder = null;
    if (recorder && recorder.state !== "inactive") recorder.stop();
  }

  /**
   * Goes on recording a version whose track ended, with a new track, as a new segment.
   * The time it missed goes with the segment, for the runner to fill.
   */
  resumeWith(version: RecordingVersion, video: MediaStreamTrack): void {
    const v = this.versions.find((x) => x.version === version);
    if (!v || v.state !== "waiting") return;
    v.segment++;
    v.seq = 0;
    v.video = video;
    (v.meta.gaps ??= [])[v.segment] = Math.max(0, Math.round(this.activeMs() - v.endedAt));
    this.record(v);
    // Shared again during a pause: it waits for Resume like the other version, or the
    // two would part.
    if (this.pausedSince) {
      v.recorder?.pause();
      v.state = "paused";
    }
    this.onChange();
  }

  /** Pauses every version together, so they keep one timeline. */
  pause(): void {
    if (!this.pausedSince) this.pausedSince = Date.now();
    for (const v of this.versions) {
      if (v.state === "recording") {
        v.recorder?.pause();
        v.state = "paused";
      }
    }
    this.onChange();
  }

  resume(): void {
    if (this.pausedSince) {
      this.pausedTotal += Date.now() - this.pausedSince;
      this.pausedSince = 0;
    }
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
    // A version whose source was never shared again missed the time since: padded at
    // the end, so both versions end together.
    for (const v of this.versions) {
      if (v.state === "waiting") v.meta.trailingGap = Math.max(0, Math.round(this.activeMs() - v.endedAt));
    }
    await Promise.all(
      this.versions.map(async (v) => {
        await this.halt(v);
        v.meta.stopped = true;
        await this.store.writeMeta(v.meta).catch(() => undefined);
      }),
    );
    this.closeAudio();
    this.onChange();
    await Promise.all(this.versions.map((v) => v.queue.finish()));
    for (const v of this.versions) v.releaseLock();
    this.onChange();
  }

  /**
   * Stops and throws everything away, here and on the server. "pending" when the server
   * could not be reached: see throwAway.
   */
  async discard(): Promise<ThrowAwayResult> {
    this.discarded = true;
    for (const v of this.versions) v.queue.stop();
    this.closeAudio();
    const results = await Promise.all(
      this.versions.map(async (v) => {
        // Only once the recorder has handed out its last chunk: written after the drop,
        // it would bring the recording back as unfinished.
        await this.halt(v);
        const result = await throwAway(this.store, v.meta);
        v.releaseLock();
        return result;
      }),
    );
    this.onChange();
    return results.find((r) => r !== "gone") ?? "gone";
  }

  private closeAudio(): void {
    this.mic?.stop();
    this.micIn?.disconnect();
    void this.audio?.close();
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

export type ThrowAwayResult =
  /** Gone, here and on the server. */
  | "gone"
  /** The server could not be reached: kept here, to be thrown away when it is back. */
  | "pending"
  /** The server keeps it, as a runner has it already: gone here only. */
  | "refused";

/**
 * Tells the server to throw a recording away, then forgets it here. While the server
 * can't be reached the recording is kept here, marked, and the recovery tries again
 * the next time the course's lectures are opened: forgotten here only, the server
 * would finalize what it has after a while and publish what was thrown away.
 */
export async function throwAway(store: ChunkStore, meta: RecordingMeta, attempts = 3): Promise<ThrowAwayResult> {
  for (let attempt = 0; attempt < attempts; attempt++) {
    try {
      await abandonBrowserRecording(meta.courseId, meta.streamId, meta.recordingId);
      await store.drop(meta.key);
      return "gone";
    } catch (err) {
      if (isRefusal(err)) {
        await store.drop(meta.key);
        return err.status === 404 ? "gone" : "refused";
      }
      if (attempt < attempts - 1) await new Promise((resolve) => setTimeout(resolve, backoff(attempt)));
    }
  }
  meta.discarding = true;
  await store.writeMeta(meta).catch(() => undefined);
  return "pending";
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
