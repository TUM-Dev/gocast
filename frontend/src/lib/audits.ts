/**
 * The server-wide audit log: course, stream, user, token, integration, hall and runner
 * changes, camera moves, and warnings or errors the backend surfaced itself.
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

/**
 * The kinds the filter offers, spelled as model.AuditType.String() renders them. A
 * copy of that list, because the server answers the label rather than a code; the
 * server refuses a name it does not know, so a stale entry fails loudly.
 */
export const AUDIT_TYPES = [
  "Info",
  "Warning",
  "Error",
  "Course Created",
  "Course Edited",
  "Course Deleted",
  "Stream Created",
  "Stream Edited",
  "Stream Deleted",
  "Camera Moved",
  "Impersonation",
  "User Created",
  "User Edited",
  "User Deleted",
  "Token Created",
  "Token Deleted",
  "Integration",
  "Lecture Hall",
  "Runner",
  "Maintenance",
  "Content",
] as const;

export interface AuditPage {
  entries: AuditEntry[];
  /** Entries matching the filter across all pages. */
  total: number;
}

/**
 * One page of the audit log, newest first, optionally of a single kind. With a course,
 * only the entries about that course and its lectures -- the course administrators'
 * own view, where the server-wide log is the server administrators'.
 */
export async function fetchAuditPage(
  offset: number,
  limit: number = AUDITS_PAGE_SIZE,
  type = "",
  courseId?: number,
): Promise<AuditPage> {
  const query = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  if (type) query.set("type", type);
  const path = courseId === undefined ? "/admin/audits" : `/courses/${courseId}/audits`;
  const res = await apiGetMessage(ListAuditsResponseSchema, `${path}?${query}`);

  return {
    total: res.total,
    entries: res.audits.map((audit) => ({
      id: audit.id,
      createdAt: audit.createdAt ? timestampDate(audit.createdAt) : null,
      type: audit.type,
      message: audit.message,
      userId: audit.userId,
      userName: audit.userName,
    })),
  };
}

/** One page of the audit log, newest first. */
export async function fetchAudits(
  offset: number,
  limit: number = AUDITS_PAGE_SIZE,
): Promise<AuditEntry[]> {
  return (await fetchAuditPage(offset, limit)).entries;
}
