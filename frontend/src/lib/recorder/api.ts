/** The browser recording RPCs, and the chunk upload beside the gateway. */

import { create } from "@bufbuild/protobuf";

import {
  BrowserRecordingSchema,
  CreateBrowserRecordingRequestSchema,
  CreateBrowserRecordingResponseSchema,
  FinalizeBrowserRecordingRequestSchema,
  ListBrowserRecordingsResponseSchema,
  type BrowserRecording,
  type CreateBrowserRecordingResponse,
} from "@/gen/server/apiv2_pb";
import { apiDelete, apiGetMessage, apiPostMessage } from "@/lib/api";
import type { RecordingVersion } from "./chunk-store";

const base = (courseId: number, streamId: number) => `/courses/${courseId}/streams/${streamId}/browser-recordings`;

export function createBrowserRecording(
  courseId: number,
  streamId: number,
  version: RecordingVersion,
  mimeType: string,
): Promise<CreateBrowserRecordingResponse> {
  return apiPostMessage(
    CreateBrowserRecordingRequestSchema,
    CreateBrowserRecordingResponseSchema,
    base(courseId, streamId),
    create(CreateBrowserRecordingRequestSchema, { courseId, streamId, version, mimeType }),
  );
}

export function getBrowserRecording(courseId: number, streamId: number, id: number): Promise<BrowserRecording> {
  return apiGetMessage(BrowserRecordingSchema, `${base(courseId, streamId)}/${id}`);
}

export async function listBrowserRecordings(courseId: number, streamId: number): Promise<BrowserRecording[]> {
  return (await apiGetMessage(ListBrowserRecordingsResponseSchema, base(courseId, streamId))).recordings;
}

/** With `expected`, refused with 409 until the server has that many chunks of each segment. */
export function finalizeBrowserRecording(
  courseId: number,
  streamId: number,
  id: number,
  expected: number[] = [],
): Promise<BrowserRecording> {
  return apiPostMessage(
    FinalizeBrowserRecordingRequestSchema,
    BrowserRecordingSchema,
    `${base(courseId, streamId)}/${id}/finalize`,
    create(FinalizeBrowserRecordingRequestSchema, { courseId, streamId, recordingId: id, expected }),
  );
}

export function abandonBrowserRecording(courseId: number, streamId: number, id: number): Promise<void> {
  return apiDelete(`${base(courseId, streamId)}/${id}`);
}

/**
 * Sends one chunk. Not through apiFetch: the ticket in the path is the credential,
 * so the upload goes on when the session expires, and the caller needs the raw
 * response to tell an outage from a refusal.
 */
export function putChunk(
  uploadPath: string,
  segment: number,
  seq: number,
  data: Blob,
  gapMs?: number,
): Promise<Response> {
  const gap = gapMs ? `?gap_ms=${Math.round(gapMs)}` : "";
  return fetch(`${uploadPath}/segments/${segment}/chunks/${seq}${gap}`, {
    method: "PUT",
    body: data,
    credentials: "omit",
    headers: { "Content-Type": "application/octet-stream" },
  });
}
