/**
 * Server maintenance: manually triggering a cron job, dismissing failed transcodings
 * and emails, and reaping the media of soft-deleted streams.
 */

import { create } from "@bufbuild/protobuf";
import { timestampDate } from "@bufbuild/protobuf/wkt";

import {
  ListMaintenanceCronJobsResponseSchema,
  ListMaintenanceEmailFailuresResponseSchema,
  ListMaintenanceTranscodingFailuresResponseSchema,
  ReapDeletedMediaRequestSchema,
  ReapDeletedMediaResponseSchema,
  ReapEntryKind,
  type MaintenanceEmailFailure,
  type MaintenanceTranscodingFailure,
} from "@/gen/server/apiv2_pb";
import { apiDelete, apiGetMessage, apiPost, apiPostMessage } from "./api";

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

export type ReapEntryType = "dir" | "segment" | "file";

/** One directory or file that executing the reap would delete. */
export interface ReapEntry {
  type: ReapEntryType;
  path: string;
  bytes: number;
  streamIds: number[];
  courseIds: number[];
  deletedAt: Date | null;
}

/** Something left alone on purpose, e.g. because a copy still uses it. */
export interface ReapSkipped {
  path: string;
  streamIds: number[];
  reason: string;
}

export interface ReapPlanPage {
  entries: ReapEntry[];
  /** Only sent with the first page. */
  skipped: ReapSkipped[];
  totalEntries: number;
  totalDirs: number;
  totalFiles: number;
  totalBytes: number;
  planToken: string;
}

export interface ReapResult {
  deletedFiles: number;
  deletedBytes: number;
  errors: string[];
}

const REAP_PAGE_SIZE = 5000;

const REAP_TYPES: Record<number, ReapEntryType> = {
  [ReapEntryKind.VOD_DIR]: "dir",
  [ReapEntryKind.VOD_FILE]: "segment",
  [ReapEntryKind.FILE]: "file",
};

/**
 * A dry run: lists what executing would delete and deletes nothing. The first page
 * (offset 0) computes the plan; later pages pass its token to page through it.
 */
export async function dryRunReap(offset = 0, planToken = ""): Promise<ReapPlanPage> {
  const res = await apiPostMessage(
    ReapDeletedMediaRequestSchema,
    ReapDeletedMediaResponseSchema,
    "/admin/maintenance/reap-deleted-media",
    create(ReapDeletedMediaRequestSchema, { execute: false, planToken, offset, pageSize: REAP_PAGE_SIZE }),
  );
  return {
    entries: res.entries.map((e) => ({
      type: REAP_TYPES[e.kind] ?? "file",
      path: e.path,
      bytes: Number(e.bytes),
      streamIds: e.streamIds,
      courseIds: e.courseIds,
      deletedAt: e.deletedAt ? timestampDate(e.deletedAt) : null,
    })),
    skipped: res.skipped.map((s) => ({ path: s.path, streamIds: s.streamIds, reason: s.reason })),
    totalEntries: res.totalEntries,
    totalDirs: res.totalDirs,
    totalFiles: res.totalFiles,
    totalBytes: Number(res.totalBytes),
    planToken: res.planToken,
  };
}

/** Every entry of a dry run's plan, for downloading the complete list. */
export async function fetchAllReapEntries(first: ReapPlanPage): Promise<ReapEntry[]> {
  const entries = [...first.entries];
  while (entries.length < first.totalEntries) {
    const page = await dryRunReap(entries.length, first.planToken);
    if (!page.entries.length) break;
    entries.push(...page.entries);
  }
  return entries;
}

/**
 * Deletes what the dry run with this token listed. The server refuses if anything
 * changed since, so only what was reviewed is ever removed.
 */
export async function executeReap(planToken: string): Promise<ReapResult> {
  const res = await apiPostMessage(
    ReapDeletedMediaRequestSchema,
    ReapDeletedMediaResponseSchema,
    "/admin/maintenance/reap-deleted-media",
    create(ReapDeletedMediaRequestSchema, { execute: true, planToken }),
  );
  return {
    deletedFiles: res.deletedFiles,
    deletedBytes: Number(res.deletedBytes),
    errors: res.errors,
  };
}
