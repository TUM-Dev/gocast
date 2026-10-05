/**
 * The server-wide audit log: course and stream changes, camera moves, and warnings
 * or errors the backend surfaced itself.
 *
 * Read-only and paginated exactly like the old page -- one page fetched at a time on
 * "Previous"/"Next", never the whole log at once, since it only ever grows.
 */

import { timestampDate } from "@bufbuild/protobuf/wkt";

import { ListAuditsResponseSchema } from "@/gen/server/apiv2_pb";
import { apiGetMessage } from "./api";

export interface AuditEntry {
  id: number;
  createdAt: Date | null;
  /** Already rendered to text server-side, e.g. "Course Created", "Camera Moved". */
  type: string;
  message: string;
  /** 0 when the audit was made by the system rather than a signed-in user. */
  userId: number;
  userName: string;
}

/** Matches the page size the old admin page used. */
export const AUDITS_PAGE_SIZE = 10;

/** One page of the audit log, newest first. */
export async function fetchAudits(
  offset: number,
  limit: number = AUDITS_PAGE_SIZE,
): Promise<AuditEntry[]> {
  const res = await apiGetMessage(
    ListAuditsResponseSchema,
    `/admin/audits?limit=${limit}&offset=${offset}`,
  );

  return res.audits.map((audit) => ({
    id: audit.id,
    createdAt: audit.createdAt ? timestampDate(audit.createdAt) : null,
    type: audit.type,
    message: audit.message,
    userId: audit.userId,
    userName: audit.userName,
  }));
}
