/**
 * Server maintenance: thumbnail regeneration, manually triggering a cron job, and
 * dismissing failed transcodings and emails.
 *
 * Thumbnail regeneration runs in a background goroutine the request that started it
 * does not wait for, so the old page polled a status endpoint every five seconds
 * instead of blocking; REFRESH_INTERVAL_MS keeps that behaviour here.
 */

import { timestampDate } from "@bufbuild/protobuf/wkt";

import {
  ListMaintenanceCronJobsResponseSchema,
  ListMaintenanceEmailFailuresResponseSchema,
  ListMaintenanceTranscodingFailuresResponseSchema,
  MaintenanceThumbnailStatusSchema,
  type MaintenanceEmailFailure,
  type MaintenanceTranscodingFailure,
} from "@/gen/server/apiv2_pb";
import { apiDelete, apiGetMessage, apiPost, apiPostMessage } from "./api";

/** How often the page polls while a thumbnail run is in progress. */
export const REFRESH_INTERVAL_MS = 5000;

export interface ThumbnailStatus {
  running: boolean;
  /** Fraction between 0 and 1; meaningless while running is false. */
  progress: number;
}

export interface TranscodingFailure {
  id: number;
  streamId: number;
  version: string;
  friendlyTime: string;
  hostname: string;
  filePath: string;
  logs: string;
}

export interface EmailFailure {
  id: number;
  to: string;
  subject: string;
  body: string;
  lastTry: Date | null;
  retries: number;
  errors: string;
}

function toThumbnailStatus(status: { running: boolean; progress: number }): ThumbnailStatus {
  return { running: status.running, progress: status.progress };
}

function toTranscodingFailure(failure: MaintenanceTranscodingFailure): TranscodingFailure {
  return {
    id: failure.id,
    streamId: failure.streamId,
    version: failure.version,
    friendlyTime: failure.friendlyTime,
    hostname: failure.hostname,
    filePath: failure.filePath,
    logs: failure.logs,
  };
}

function toEmailFailure(failure: MaintenanceEmailFailure): EmailFailure {
  return {
    id: failure.id,
    to: failure.to,
    subject: failure.subject,
    body: failure.body,
    lastTry: failure.lastTry ? timestampDate(failure.lastTry) : null,
    retries: failure.retries,
    errors: failure.errors,
  };
}

/** The current state of the background regeneration job, for the page to poll. */
export async function fetchThumbnailStatus(): Promise<ThumbnailStatus> {
  const res = await apiGetMessage(
    MaintenanceThumbnailStatusSchema,
    "/admin/maintenance/thumbnails/status",
  );
  return toThumbnailStatus(res);
}

/**
 * Starts regenerating every VoD thumbnail. A run already in progress is left alone by
 * the server, so calling this twice is harmless; the returned status reflects
 * whichever run is now active.
 */
export async function generateThumbnails(): Promise<ThumbnailStatus> {
  const res = await apiPostMessage(
    MaintenanceThumbnailStatusSchema,
    MaintenanceThumbnailStatusSchema,
    "/admin/maintenance/thumbnails/generate",
    { $typeName: "protobuf.MaintenanceThumbnailStatus", running: false, progress: 0 },
  );
  return toThumbnailStatus(res);
}

/** The jobs the scheduler knows by name, to offer as choices before running one. */
export async function fetchCronJobs(): Promise<string[]> {
  const res = await apiGetMessage(ListMaintenanceCronJobsResponseSchema, "/admin/maintenance/cron");
  return res.jobs;
}

/** Runs a registered job immediately, without waiting for its schedule. */
export async function runCronJob(job: string): Promise<void> {
  await apiPost("/admin/maintenance/cron/run", { job });
}

/** Every recorded transcoding failure. */
export async function fetchTranscodingFailures(): Promise<TranscodingFailure[]> {
  const res = await apiGetMessage(
    ListMaintenanceTranscodingFailuresResponseSchema,
    "/admin/maintenance/transcoding-failures",
  );
  return res.failures.map(toTranscodingFailure);
}

/** Dismisses a recorded failure. It does not retry the transcode. */
export async function deleteTranscodingFailure(id: number): Promise<void> {
  await apiDelete(`/admin/maintenance/transcoding-failures/${id}`);
}

/** Every email that has exhausted its retries or is still failing. */
export async function fetchEmailFailures(): Promise<EmailFailure[]> {
  const res = await apiGetMessage(
    ListMaintenanceEmailFailuresResponseSchema,
    "/admin/maintenance/email-failures",
  );
  return res.failures.map(toEmailFailure);
}

/** Dismisses a failed email. It is not retried afterwards. */
export async function deleteEmailFailure(id: number): Promise<void> {
  await apiDelete(`/admin/maintenance/email-failures/${id}`);
}
