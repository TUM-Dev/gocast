/**
 * Semesters, as the sidebar's semester picker needs them.
 *
 * A semester is a year and a teaching term: `W` runs across the turn of the year,
 * `S` within it.
 */

import { GetSemestersResponseSchema, type Semester as SemesterMessage } from "@/gen/server/apiv2_pb";
import { apiGetMessagePublic } from "./api";

export type TeachingTerm = "W" | "S";

export interface Semester {
  year: number;
  term: TeachingTerm;
}

export interface Semesters {
  /** The semester the deployment considers current, by date. */
  current: Semester;
  /** Every semester with courses, newest first. */
  all: Semester[];
}

/**
 * `Winter 2026/27` or `Summer 2026`.
 *
 * Kept identical to Semester.FriendlyString in web/ts/api/semesters.ts: a winter
 * semester spans two years and is labelled with both, which is what people call it.
 */
export function semesterLabel(semester: Semester): string {
  if (semester.term === "W") {
    return `Winter ${semester.year}/${`${semester.year + 1}`.slice(-2)}`;
  }
  return `Summer ${semester.year}`;
}

export function sameSemester(a: Semester | undefined, b: Semester | undefined): boolean {
  return a !== undefined && b !== undefined && a.year === b.year && a.term === b.term;
}

/**
 * The server's teaching term is a free string; anything but `S` is a winter semester,
 * matching how model.Course treats it.
 */
function parseSemester(message: SemesterMessage): Semester {
  return { year: message.year, term: message.teachingTerm === "S" ? "S" : "W" };
}

export async function fetchSemesters(): Promise<Semesters> {
  // Public, and identical for everyone: GetSemesters never looks at the caller. Asking
  // for a token here would fail for a visitor who is not signed in, and the start page
  // waits on this before it loads anything else.
  const res = await apiGetMessagePublic(GetSemestersResponseSchema, "/semesters");

  // `current` is always sent; the fallback keeps a malformed response from leaving
  // the picker with no selection at all.
  const all = res.semesters.map(parseSemester);
  const current = res.current ? parseSemester(res.current) : all[0];

  return { current, all };
}
