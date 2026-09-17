/**
 * The encoding workers, for the administration page.
 *
 * Workers are the older, pre-runner equivalent of the runner pool: same idea, keyed
 * by a self-generated UUID instead of hostname, and they additionally self-report VM
 * stats. The old page rendered once per load; this polls instead, the way the
 * already-migrated runners page does, so the list does not go stale between visits.
 */

import { timestampDate } from "@bufbuild/protobuf/wkt";

import { ListWorkersResponseSchema } from "@/gen/server/apiv2_pb";
import { apiDelete, apiGetMessage } from "./api";

export interface Worker {
  /** Workers are keyed by a UUID they generate for themselves; there is no hostname key. */
  workerId: string;
  host: string;
  version: string;
  /** Derived by the server, so this cannot disagree with the scheduler. */
  alive: boolean;
  workload: number;
  /** The jobs currently running, as a comma-separated summary the worker sends itself. */
  status: string;
  /** VM stats, self-reported by the worker; empty until it has sent a heartbeat. */
  cpu: string;
  memory: string;
  disk: string;
  uptime: string;
  lastSeen: Date | null;
}

export interface WorkersPage {
  workers: Worker[];
  /** The token a new worker authenticates its registration with. */
  token: string;
}

/** How often the page refreshes, matching the pattern the runners page settled on. */
export const REFRESH_INTERVAL_MS = 5000;

/** Every registered worker, sorted so rows do not reorder under the pointer. */
export async function fetchWorkers(): Promise<WorkersPage> {
  const res = await apiGetMessage(ListWorkersResponseSchema, "/admin/workers");

  const workers = res.workers
    .map((worker) => ({
      workerId: worker.workerId,
      host: worker.host,
      version: worker.version,
      alive: worker.alive,
      workload: worker.workload,
      status: worker.status,
      cpu: worker.cpu,
      memory: worker.memory,
      disk: worker.disk,
      uptime: worker.uptime,
      lastSeen: worker.lastSeen ? timestampDate(worker.lastSeen) : null,
    }))
    .sort((a, b) => a.host.localeCompare(b.host));

  return { workers, token: res.workerToken };
}

/** Removes a registration. A worker still running re-registers on its next heartbeat. */
export async function deleteWorker(workerId: string): Promise<void> {
  await apiDelete(`/admin/workers/${encodeURIComponent(workerId)}`);
}
