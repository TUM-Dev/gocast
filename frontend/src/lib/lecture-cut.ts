/**
 * What the cut editor asks of the server: the audio level timeline of a lecture, which a
 * runner builds on request, and the cut itself.
 */

import { fromJson, type JsonValue } from "@bufbuild/protobuf";

import {
  AbandonLectureCutResponseSchema,
  CutLectureResponseSchema,
  GetLectureCutResponseSchema,
  GetLectureWaveformResponseSchema,
  LectureCutMode,
  CutLectureRequestSchema,
} from "@/gen/server/apiv2_pb";
import { ApiError, apiFetch, apiGetMessage, apiPost, apiPostMessage } from "./api";
import type { TranscodingProgress } from "./lecture-content";
import type { WatchSilence } from "./watch";

export { LectureCutMode };

export interface Waveform {
  /** One level per bucket, 0 (-60 dBFS or less) to 255 (0 dBFS). */
  peaks: Uint8Array;
  bucketSeconds: number;
  /** Seconds, by version (COMB, CAM, PRES). */
  versionDurations: Record<string, number>;
  /** The versions differ in length by more than one cut can span. */
  versionsDiffer: boolean;
  silences: WatchSilence[];
}

const lecturePath = (courseId: number, streamId: number) => `/courses/${courseId}/streams/${streamId}`;

/** The timeline, or null while a runner has not delivered it. */
export async function fetchWaveform(courseId: number, streamId: number): Promise<Waveform | null> {
  const res = await apiGetMessage(GetLectureWaveformResponseSchema, `${lecturePath(courseId, streamId)}/waveform`);
  if (!res.ready) return null;
  return {
    peaks: res.peaks,
    bucketSeconds: res.bucketSeconds,
    versionDurations: res.versionDurations,
    versionsDiffer: res.versionsDiffer,
    silences: res.silences,
  };
}

export async function requestWaveform(courseId: number, streamId: number): Promise<void> {
  await apiPost(`${lecturePath(courseId, streamId)}/waveform`, {});
}

/**
 * The length the editor works with: the default version's, which is what the person
 * watches. A version the runner could not measure is not listed, so fall back to the
 * longest of those it could.
 */
export function waveformDuration(w: Waveform): number {
  const comb = w.versionDurations["COMB"];
  if (comb !== undefined && comb > 0) return comb;
  return Math.max(0, ...Object.values(w.versionDurations));
}

export const WAVEFORM_POLL_MS = 3000;
/** A runner decodes the audio of a lecture in a minute or two; this is for one that never answers. */
export const WAVEFORM_GIVE_UP_MS = 15 * 60 * 1000;

export interface WaveformPoll {
  fetch: () => Promise<Waveform | null>;
  onReady: (w: Waveform) => void;
  /** A failed request, or the runner not answering in time. Polling has stopped. */
  onError: (err: unknown) => void;
  intervalMs?: number;
  giveUpMs?: number;
  now?: () => number;
}

/** Asks until the timeline is there. Returns the function that stops it. */
export function pollWaveform(poll: WaveformPoll): () => void {
  const interval = poll.intervalMs ?? WAVEFORM_POLL_MS;
  const giveUp = poll.giveUpMs ?? WAVEFORM_GIVE_UP_MS;
  const now = poll.now ?? Date.now;
  const started = now();
  let stopped = false;
  let timer: ReturnType<typeof setTimeout> | undefined;

  async function tick(): Promise<void> {
    try {
      const w = await poll.fetch();
      if (stopped) return;
      if (w) {
        stopped = true;
        poll.onReady(w);
        return;
      }
    } catch (err) {
      if (stopped) return;
      stopped = true;
      poll.onError(err);
      return;
    }
    if (now() - started > giveUp) {
      stopped = true;
      poll.onError(new Error("timed out"));
      return;
    }
    timer = setTimeout(tick, interval);
  }

  timer = setTimeout(tick, 0);
  return () => {
    stopped = true;
    if (timer !== undefined) clearTimeout(timer);
  };
}

/** Starts the cut. Resolves with the lecture the cut recording belongs to. */
export async function cutLecture(
  courseId: number,
  streamId: number,
  mode: LectureCutMode,
  discard: { start: number; end: number }[],
): Promise<number> {
  const res = await apiPostMessage(CutLectureRequestSchema, CutLectureResponseSchema, `${lecturePath(courseId, streamId)}/cut`, {
    $typeName: "protobuf.CutLectureRequest",
    courseId,
    streamId,
    mode,
    discard: discard.map((d) => ({ $typeName: "protobuf.LectureCutRange" as const, start: d.start, end: d.end })),
  });
  return res.streamId;
}

/**
 * A cut that is not finished. It stays open until every version is cut, so that one that
 * failed can be cut again with the same ranges, or the cut given up.
 */
export interface OpenCut {
  /** The lecture that gets the cut recording: the one asked about when it is replaced. */
  targetStreamId: number;
  /** A new lecture is made, and the recording is not replaced. */
  duplicate: boolean;
  /** The versions being cut or failed. None once all are done: the cut is being finished. */
  progresses: TranscodingProgress[];
}

/** The cut of the lecture that is not finished, or null. */
export async function fetchOpenCut(courseId: number, streamId: number): Promise<OpenCut | null> {
  const res = await apiGetMessage(GetLectureCutResponseSchema, `${lecturePath(courseId, streamId)}/cut`);
  if (!res.open) return null;
  return {
    targetStreamId: res.targetStreamId,
    duplicate: res.duplicate,
    progresses: res.progresses.map((p) => ({ version: p.version, progress: p.progress, failed: p.failed, error: p.error })),
  };
}

/** Cuts the versions that failed again, with the ranges of the cut. */
export async function retryCut(courseId: number, streamId: number): Promise<void> {
  await apiPost(`${lecturePath(courseId, streamId)}/cut/retry`, {});
}

/**
 * Gives up the cut. Resolves with the versions of a replaced recording that were not cut
 * and still hold what the cut would have removed.
 */
export async function abandonCut(courseId: number, streamId: number): Promise<string[]> {
  const json = await apiFetch<JsonValue>(`${lecturePath(courseId, streamId)}/cut`, { method: "DELETE" });
  return fromJson(AbandonLectureCutResponseSchema, json, { ignoreUnknownFields: true }).leftUncut;
}

/** The server's message for a refusal, as a sentence. */
function sentence(message: string, fallback: string): string {
  return message ? message.charAt(0).toUpperCase() + message.slice(1) + "." : fallback;
}

/** What to tell the person when one of these calls fails. */
export function cutErrorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.isUnauthenticated) return "Your session expired. Please sign in again.";
    switch (err.status) {
      case 404:
        return "This lecture does not exist or you do not administer it.";
      case 409:
        // Several things are refused with this, and the server says which.
        return sentence(err.message, "The lecture is being converted. Wait until it is done.");
      case 503:
        return "No runner is available right now. Please try again in a moment.";
      default:
        return err.message;
    }
  }
  if (err instanceof Error && err.message === "timed out") {
    return "The timeline is taking too long to build. Please try again later.";
  }
  return "Something went wrong. Please try again.";
}
