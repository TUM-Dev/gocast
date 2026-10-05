/**
 * The search page: courses, lectures and subtitle lines by text, narrowed by
 * semesters or by courses. Public, and a signed-in user finds more, so every call
 * goes out with the session when there is one.
 */

import { SearchResponseSchema } from "@/gen/server/apiv2_pb";
import { ApiError, apiGetMessageOptionalAuth } from "./api";
import { fetchPublicCourses, fetchUserCourses, type Course } from "./courses";
import type { Semester } from "./semesters";

export interface SearchCourseHit {
  name: string;
  slug: string;
  semester: Semester;
}

export interface SearchStreamHit {
  id: number;
  name: string;
  description: string;
  courseName: string;
  courseSlug: string;
  semester: Semester;
}

export interface SearchSubtitleHit {
  streamId: number;
  /** Milliseconds into the recording. */
  timestamp: number;
  textPrev: string;
  text: string;
  textNext: string;
  streamName: string;
  streamStart?: Date;
  courseName: string;
  courseSlug: string;
  semester: Semester;
}

export interface SearchHits {
  courses: SearchCourseHit[];
  streams: SearchStreamHit[];
  subtitles: SearchSubtitleHit[];
}

/** What the page lets the user narrow a search by. */
export interface SearchFilters {
  semesters: Semester[];
  /** Up to three; searching inside them looks at lectures and subtitles instead. */
  courses: Course[];
}

/** The fewest characters a query needs before it is sent, as the old page had it. */
export const MIN_QUERY_LENGTH = 3;
export const MAX_FILTER_COURSES = 3;
export const SEARCH_LIMIT = 20;

/** A semester as the API's filter wants it: "2022S". */
export const semesterKey = (s: Semester): string => `${s.year}${s.term}`;
/** A course as the API's filter wants it: "brauereiwesen2022S". */
export const courseKey = (c: Pick<Course, "slug" | "semester">): string => `${c.slug}${semesterKey(c.semester)}`;

/** Which kinds of hit a search with these filters can return, as the old page showed them. */
export function kindsFor(filters: SearchFilters): { streams: boolean; subtitles: boolean } {
  const inCourses = filters.courses.length > 0;
  return { streams: inCourses || filters.semesters.length === 1, subtitles: inCourses };
}

export function searchPath(query: string, filters: SearchFilters, limit = SEARCH_LIMIT): string {
  const params = new URLSearchParams({ query, limit: String(limit) });
  for (const s of filters.semesters) params.append("semesters", semesterKey(s));
  for (const c of filters.courses) params.append("courses", courseKey(c));
  return `/search?${params.toString()}`;
}

export async function search(query: string, filters: SearchFilters, limit = SEARCH_LIMIT): Promise<SearchHits> {
  const res = await apiGetMessageOptionalAuth(SearchResponseSchema, searchPath(query, filters, limit));
  const semester = (year: number, term: string): Semester => ({ year, term: term as Semester["term"] });
  return {
    courses: res.courses.map((c) => ({ name: c.name, slug: c.slug, semester: semester(c.year, c.term) })),
    streams: res.streams.map((s) => ({
      id: s.id,
      name: s.name,
      description: s.description,
      courseName: s.courseName,
      courseSlug: s.courseSlug,
      semester: semester(s.year, s.term),
    })),
    subtitles: res.subtitles.map((s) => ({
      streamId: s.streamId,
      timestamp: Number(s.timestamp),
      textPrev: s.textPrev,
      text: s.text,
      textNext: s.textNext,
      streamName: s.streamName,
      streamStart: s.streamStart ? new Date(Number(s.streamStart.seconds) * 1000) : undefined,
      courseName: s.courseName,
      courseSlug: s.courseSlug,
      semester: semester(s.year, s.term),
    })),
  };
}

/**
 * The courses the user may pick to search inside: everyone's public ones and, when
 * signed in, their own, for each chosen semester, each course once.
 */
export async function coursesOfSemesters(semesters: Semester[], signedIn: boolean): Promise<Course[]> {
  const seen = new Set<string>();
  const courses: Course[] = [];
  for (const semester of semesters) {
    const lists = [fetchPublicCourses(semester)];
    if (signedIn) lists.push(fetchUserCourses(semester));
    for (const list of await Promise.all(lists)) {
      for (const course of list) {
        const key = courseKey(course);
        if (seen.has(key)) continue;
        seen.add(key);
        courses.push(course);
      }
    }
  }
  return courses;
}

export const courseHitUrl = (hit: SearchCourseHit): string =>
  `/course/${hit.semester.year}/${hit.semester.term}/${hit.slug}`;
export const streamHitUrl = (hit: SearchStreamHit): string => `/w/${hit.courseSlug}/${hit.id}`;
/** The watch page takes `t` in seconds. */
export const subtitleHitUrl = (hit: SearchSubtitleHit): string =>
  `/w/${hit.courseSlug}/${hit.streamId}?t=${Math.floor(hit.timestamp / 1000)}`;

/** A subtitle's place in the recording, "1:02:03" or "2:03". */
export function formatTimestamp(ms: number): string {
  const total = Math.max(0, Math.floor(ms / 1000));
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  const mm = h > 0 ? String(m).padStart(2, "0") : String(m);
  return `${h > 0 ? `${h}:` : ""}${mm}:${String(s).padStart(2, "0")}`;
}

export function searchErrorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.status === 503) return "Search is not available right now. Please try again later.";
    if (err.status === 404) return "One of the chosen courses is not available to you.";
    if (err.status === 400) return err.message;
  }
  return "Something went wrong. Please try again.";
}
