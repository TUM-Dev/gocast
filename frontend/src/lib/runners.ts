/**
 * The transcoding runners, for the administration page.
 *
 * The server-rendered page kept this list fresh over a websocket, but not as a
 * subscription: web/ts/api/runner.ts asked the channel for the alive statuses every
 * five seconds and the server answered with all of them. That is polling with a
 * socket in the middle, and the answer was a subset of what this endpoint returns —
 * so the SPA polls the endpoint directly and no realtime plumbing is involved.
 */

import { timestampDate } from "@bufbuild/protobuf/wkt";

import { ListRunnersResponseSchema } from "@/gen/server/apiv2_pb";
import { apiDelete, apiGetMessage } from "./api";

export interface Runner {
  /** Runners are keyed by hostname; there is no numeric id. */
  hostname: string;
  port: number;
  version: string;
  /**
   * Derived by the server from the last heartbeat, not computed here: the five-second
   * rule belongs to model.Runner, and a second copy of it would eventually disagree
   * with the scheduler about which runners can take work.
   */
  alive: boolean;
  jobCount: number;
  /** Shutting down, so it is being given no further jobs. */
  draining: boolean;
  registeredAt: Date | null;
}

/** How often the page refreshes, matching the old page's five-second poll. */
export const REFRESH_INTERVAL_MS = 5000;

/**
 * Every registered runner.
 *
 * Sorted by hostname so the rows do not reorder under the pointer on each poll. The
 * endpoint returns database order, which nothing guarantees is stable.
 */
export async function fetchRunners(): Promise<Runner[]> {
  const res = await apiGetMessage(ListRunnersResponseSchema, "/runners");

  return res.runners
    .map((runner) => ({
      hostname: runner.hostname,
      port: runner.port,
      version: runner.version,
      alive: runner.alive,
      // protobuf uint64 is a bigint over the wire; job counts are small enough that
      // Number is exact, and the template needs one to render.
      jobCount: Number(runner.jobCount),
      draining: runner.draining,
      registeredAt: runner.timeOfRegister ? timestampDate(runner.timeOfRegister) : null,
    }))
    .sort((a, b) => a.hostname.localeCompare(b.hostname));
}

/**
 * Removes a runner's registration.
 *
 * Not a way to stop a runner: one that is still running registers again on its next
 * heartbeat, which is why the page says "remove" rather than "delete".
 */
export async function deleteRunner(hostname: string): Promise<void> {
  await apiDelete(`/runners/${encodeURIComponent(hostname)}`);
}

/**
 * How long ago a runner registered, in the words the old page used.
 *
 * Its own implementation of this lived in an x-data block in the template and parsed
 * a formatted timestamp back into a Date; the API sends a real one.
 */
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
