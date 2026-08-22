/**
 * Streams — one lecture, live or recorded — and the caller's watch progress in them.
 *
 * The generated types stop at this boundary: timestamps become Dates and absent
 * messages become undefined, so nothing above here handles a protobuf Timestamp or
 * tests an id for zero.
 */

import { timestampDate } from "@bufbuild/protobuf/wkt";

import { create } from "@bufbuild/protobuf";

import {
  GetProgressBatchResponseSchema,
  StreamProgressSchema,
  UpdateProgressRequestSchema,
  type Download as DownloadMessage,
  type Stream as StreamMessage,
} from "@/gen/server/apiv2_pb";
import { apiGetMessage, apiPatchMessage } from "./api";

export interface Download {
  friendlyName: string;
  downloadUrl: string;
}

export interface Stream {
  id: number;
  name: string;
  description: string;
  courseId: number;
  start: Date;
  end: Date;
  /** Seconds. Zero for a lecture that has not been recorded yet. */
  duration: number;
  liveNow: boolean;
  recording: boolean;
  /** Scheduled, with no recording to watch yet. */
  isPlanned: boolean;
  /** Starting soon enough to offer the waiting room. */
  isComingUp: boolean;
  /**
   * False for a private lecture. Only a course administrator is ever sent one, so
   * this marks the lectures the rest of the course cannot see.
   */
  isPubliclyVisible: boolean;
  /** Direct playlist URL, offered for use in an external player. */
  hlsUrl: string;
  /**
   * The room a lecture is held in, as a campus room code such as `5502.EG.001`.
   * Empty for a lecture that is not held in one.
   */
  roomCode: string;
  downloads: Download[];
}

export interface StreamProgress {
  streamId: number;
  /** Fraction watched, 0 to 1. */
  progress: number;
  watched: boolean;
}

/** True when the lecture has a name of its own rather than only a date. */
export function hasName(stream: Stream): boolean {
  return stream.name !== "";
}

/** True when the lecture starts on today's date. */
export function isToday(stream: Stream): boolean {
  const today = new Date();
  return (
    stream.start.getFullYear() === today.getFullYear() &&
    stream.start.getMonth() === today.getMonth() &&
    stream.start.getDate() === today.getDate()
  );
}

/** Whole minutes until the lecture starts; negative once it has. */
export function minutesUntilStart(stream: Stream, now = new Date()): number {
  return Math.round((stream.start.valueOf() - now.valueOf()) / 60_000);
}

/** `1:23:45`, as the server-rendered page formatted it. */
export function formatDuration(seconds: number): string {
  return new Date(seconds * 1000).toISOString().slice(11, 19);
}

/**
 * Whether a room code is one the campus map can resolve. Copied from
 * web/ts/utilities/lectureHallValidator.ts: the field also holds free text for rooms
 * that are not on it, and linking those produces a dead link.
 */
export function isMappedRoom(roomCode: string): boolean {
  return /^\d{4}\.[A-Z0-9]{2}\.[A-Z0-9]{3,4}$/.test(roomCode);
}

/** The campus map entry for a room. */
export function roomUrl(roomCode: string): string {
  return `https://nav.tum.de/room/${roomCode}`;
}

/**
 * The watch page, which Go still renders — so every link to it is a full navigation
 * rather than a client-side route.
 */
export function watchUrl(slug: string, streamId: number): string {
  return `/w/${slug}/${streamId}`;
}

/**
 * Thumbnail for a lecture: the live frame while it is streaming, the generated one
 * afterwards, chosen server-side.
 *
 * Used as an `<img>` source, which sends no `Authorization` header — this works
 * because the API also accepts the session cookie. That fallback is documented as
 * temporary in apiv2/server/authorization.go, and every thumbnail on the site breaks
 * on the day it is removed.
 */
export function thumbnailUrl(slug: string, streamId: number): string {
  return `/api/v2/streams/${encodeURIComponent(slug)}/${streamId}/thumbs`;
}

export function parseDownload(message: DownloadMessage): Download {
  return { friendlyName: message.friendlyName, downloadUrl: message.downloadUrl };
}

export function parseStream(message: StreamMessage): Stream {
  return {
    id: message.id,
    name: message.name,
    description: message.description,
    courseId: message.courseId,
    // Always set by the server: ParseStreamToProto writes both unconditionally.
    start: message.start ? timestampDate(message.start) : new Date(0),
    end: message.end ? timestampDate(message.end) : new Date(0),
    duration: message.duration,
    liveNow: message.liveNow,
    recording: message.recording,
    isPlanned: message.isPlanned,
    isComingUp: message.isComingUp,
    isPubliclyVisible: message.isPubliclyVisible,
    hlsUrl: message.hlsUrl,
    roomCode: message.roomCode,
    downloads: message.downloads.map(parseDownload),
  };
}

/**
 * Watch progress for the given lectures, keyed by stream id.
 *
 * The response is sparse and in no particular order — the server returns rows the user
 * actually has, not one per id asked for — so this is a map rather than a list. The
 * old page zipped the two by index, which only held while every lecture had a row.
 *
 * Requires a signed-in user, and the endpoint rejects an empty list, so an empty
 * request is answered here instead of being sent.
 */
export async function fetchProgress(streamIds: number[]): Promise<Map<number, StreamProgress>> {
  if (streamIds.length === 0) {
    return new Map();
  }

  const query = streamIds.map((id) => `streamIds=${id}`).join("&");
  const res = await apiGetMessage(GetProgressBatchResponseSchema, `/progress?${query}`);

  return new Map(
    res.progressBatch.map((p) => [
      p.streamId,
      { streamId: p.streamId, progress: p.progress, watched: p.watched },
    ]),
  );
}

/**
 * Marks a lecture watched or unwatched.
 *
 * The endpoint takes a progress fraction as well, and a lecture marked watched by hand
 * is recorded as watched in full — the same thing the old page's menu item did.
 */
export async function setWatched(streamId: number, watched: boolean): Promise<StreamProgress> {
  const res = await apiPatchMessage(
    UpdateProgressRequestSchema,
    StreamProgressSchema,
    `/progress/${streamId}`,
    create(UpdateProgressRequestSchema, { streamId, watched, progress: watched ? 1 : 0 }),
  );

  return { streamId: res.streamId, progress: res.progress, watched: res.watched };
}
