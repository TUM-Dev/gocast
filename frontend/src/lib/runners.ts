/**
 * The transcoding runners, for the administration page.
 *
 * The old page's websocket was not a subscription — it asked for the alive statuses
 * every five seconds — so this polls the endpoint and needs no realtime plumbing.
 */

import { timestampDate } from "@bufbuild/protobuf/wkt";

import { ListRunnersResponseSchema } from "@/gen/server/apiv2_pb";
import { apiDelete, apiGetMessage } from "./api";

export interface Runner {
  /** Runners are keyed by hostname; there is no numeric id. */
  hostname: string;
  port: number;
  version: string;
  /** Derived by the server, so this cannot disagree with the scheduler. */
  alive: boolean;
  jobCount: number;
  /** Shutting down, so it is being given no further jobs. */
  draining: boolean;
  registeredAt: Date | null;
}

/** How often the page refreshes, matching the old page's five-second poll. */
export const REFRESH_INTERVAL_MS = 5000;

/** Every registered runner, sorted so rows do not reorder under the pointer. */
export async function fetchRunners(): Promise<Runner[]> {
  const res = await apiGetMessage(ListRunnersResponseSchema, "/admin/runners");

  return res.runners
    .map((runner) => ({
      hostname: runner.hostname,
      port: runner.port,
      version: runner.version,
      alive: runner.alive,
      // A uint64 arrives as a bigint; job counts are small enough for Number.
      jobCount: Number(runner.jobCount),
      draining: runner.draining,
      registeredAt: runner.timeOfRegister ? timestampDate(runner.timeOfRegister) : null,
    }))
    .sort((a, b) => a.hostname.localeCompare(b.hostname));
}

/** Removes a registration. A runner still running re-registers on its next heartbeat. */
export async function deleteRunner(hostname: string): Promise<void> {
  await apiDelete(`/admin/runners/${encodeURIComponent(hostname)}`);
}

/** How long ago a runner registered, in the words the old page used. */
export function timeAgo(from: Date | null, now: Date = new Date()): string {
  if (!from) return "unknown";

  const seconds = Math.floor((now.getTime() - from.getTime()) / 1000);
  const intervals: [string, number][] = [
    ["year", 31536000],
    ["month", 2592000],
    ["day", 86400],
    ["hour", 3600],
    ["minute", 60],
    ["second", 1],
  ];

  for (const [label, size] of intervals) {
    const count = Math.floor(seconds / size);
    if (count >= 1) {
      return `${count} ${label}${count === 1 ? "" : "s"}`;
    }
  }

  return "just now";
}
