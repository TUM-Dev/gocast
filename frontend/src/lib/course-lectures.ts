/**
 * A course's lectures as its administrators manage them: the edit-course page's
 * lecture list, over the v2 lecture administration RPCs.
 */

import { timestampDate } from "@bufbuild/protobuf/wkt";

import {
  CourseAdminSchema,
  ListAdministeredCoursesResponseSchema,
  ListCourseLecturesAdminResponseSchema,
  type CourseLectureAdmin,
} from "@/gen/server/apiv2_pb";
import { ApiError, apiDelete, apiFetch, apiGetMessage, apiPatch, apiPost } from "./api";

/** model.FileType's attachment; the kinds above it are thumbnails. */
export const FILE_TYPE_ATTACHMENT = 2;

export interface CourseLecture {
  id: number;
  courseId: number;
  name: string;
  description: string;
  start: Date;
  end: Date;
  /** 0 for a lecture in no hall, i.e. self-streamed. */
  lectureHallId: number;
  lectureHallName: string;
  /** Empty for a lecture in no series. */
  seriesIdentifier: string;
  /** The secret a self-streaming lecturer puts in the ingest URL. */
  streamKey: string;
  chatEnabled: boolean;
  private: boolean;
  liveNow: boolean;
  /** Has a recording (a VoD). */
  recording: boolean;
  /** Ended and not recorded. */
  past: boolean;
  converting: boolean;
  premiere: boolean;
  /** Any of COMB, PRES and CAM. */
  vodVersions: string[];
  /** 0 when not known. */
  durationSeconds: number;
  files: { id: number; type: number; friendlyName: string }[];
  transcodingProgresses: { version: string; progress: number }[];
  videoSectionCount: number;
}

function toLecture(l: CourseLectureAdmin): CourseLecture {
  return {
    id: l.id,
    courseId: l.courseId,
    name: l.name,
    description: l.description,
    // Always sent; the epoch fallback only keeps a malformed answer from throwing.
    start: l.start ? timestampDate(l.start) : new Date(0),
    end: l.end ? timestampDate(l.end) : new Date(0),
    lectureHallId: l.lectureHallId,
    lectureHallName: l.lectureHallName,
    seriesIdentifier: l.seriesIdentifier,
    streamKey: l.streamKey,
    chatEnabled: l.chatEnabled,
    private: l.private,
    liveNow: l.liveNow,
    recording: l.recording,
    past: l.past,
    converting: l.converting,
    premiere: l.premiere,
    vodVersions: [...l.vodVersions],
    durationSeconds: l.durationSeconds,
    files: l.files.map((f) => ({ id: f.id, type: f.type, friendlyName: f.friendlyName })),
    transcodingProgresses: l.transcodingProgresses.map((p) => ({
      version: p.version,
      progress: p.progress,
    })),
    videoSectionCount: l.videoSections.length,
  };
}

/** Every lecture of the course, newest first. */
export async function fetchCourseLectures(courseId: number): Promise<CourseLecture[]> {
  const res = await apiGetMessage(
    ListCourseLecturesAdminResponseSchema,
    `/courses/${courseId}/lectures/admin`,
  );
  return res.lectures.map(toLecture);
}

export interface CourseHeader {
  name: string;
  /** Part of a self-streamer's stream key, `<slug>-<lecture id>`. */
  slug: string;
}

export async function fetchCourseHeader(courseId: number): Promise<CourseHeader> {
  const res = await apiGetMessage(CourseAdminSchema, `/courses/${courseId}/admin`);
  return { name: res.name, slug: res.slug };
}

export interface AdministeredCourse {
  id: number;
  name: string;
  year: number;
  term: string;
}

/** Where a lecture can be copied or moved to. */
export async function fetchAdministeredCourses(): Promise<AdministeredCourse[]> {
  const res = await apiGetMessage(ListAdministeredCoursesResponseSchema, "/courses/administered");
  return res.courses.map((c) => ({ id: c.id, name: c.name, year: c.year, term: c.term }));
}

export interface LectureChanges {
  name?: string;
  description?: string;
  /** Both or neither. */
  start?: Date;
  end?: Date;
  /** 0 for none (self-streamed). */
  lectureHallId?: number;
  chatEnabled?: boolean;
  private?: boolean;
}

/** What updateLectureSeries can set on every lecture of a series. */
export type SeriesChanges = Pick<LectureChanges, "name" | "description" | "lectureHallId" | "chatEnabled">;

/** Leaves out what is not being changed: the RPCs set exactly the fields present. */
function body(changes: LectureChanges): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(changes)) {
    if (value === undefined) continue;
    out[key] = value instanceof Date ? value.toISOString() : value;
  }
  return out;
}

export async function updateLecture(
  courseId: number,
  streamId: number,
  changes: LectureChanges,
): Promise<void> {
  await apiPatch(`/courses/${courseId}/streams/${streamId}`, body(changes));
}

/** Sets the changes on every lecture of the course in `streamId`'s series. */
export async function updateLectureSeries(
  courseId: number,
  streamId: number,
  changes: SeriesChanges,
): Promise<void> {
  await apiPatch(`/courses/${courseId}/streams/${streamId}/series`, body(changes));
}

/**
 * Gives the lecture the new times and the rest of its series their time of day and
 * duration, each lecture keeping its own date.
 */
export async function updateLectureSeriesTime(
  courseId: number,
  streamId: number,
  start: Date,
  end: Date,
): Promise<void> {
  await apiFetch(`/courses/${courseId}/streams/${streamId}/series/time`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body({ start, end })),
  });
}

/** Deletes nothing unless every lecture is the course's. */
export async function deleteLectures(courseId: number, streamIds: number[]): Promise<void> {
  await apiPost(`/courses/${courseId}/streams/delete`, { streamIds });
}

export async function deleteLectureSeries(courseId: number, streamId: number): Promise<void> {
  await apiDelete(`/courses/${courseId}/streams/${streamId}/series`);
}

/** The copy's ID, a new one even on a move. */
export async function copyLecture(
  courseId: number,
  streamId: number,
  targetCourseId: number,
  move: boolean,
): Promise<number> {
  const res = await apiPost<{ streamId?: number }>(
    `/courses/${courseId}/streams/${streamId}/copy`,
    { targetCourseId, move },
  );
  return res?.streamId ?? 0;
}

/*
 * State, as the server-rendered card showed it. Exactly one primary state, which is
 * the coloured tab on the card, plus independent flags beside it.
 */

export type LectureState = "live" | "converting" | "recording" | "past" | "planned";

export function lectureState(l: CourseLecture): LectureState {
  if (l.liveNow) return "live";
  // Before recording: a version still transcoding is not watchable yet.
  if (l.converting) return "converting";
  if (l.recording) return "recording";
  if (l.past) return "past";
  return "planned";
}

export type BadgeTone = "danger" | "success" | "warn" | "info" | "muted";

export interface LectureBadge {
  key: string;
  label: string;
  tone: BadgeTone;
}

const STATE_BADGES: Record<LectureState, Omit<LectureBadge, "key">> = {
  live: { label: "Live", tone: "danger" },
  converting: { label: "Converting", tone: "info" },
  recording: { label: "VoD", tone: "success" },
  past: { label: "Past", tone: "warn" },
  planned: { label: "Scheduled", tone: "info" },
};

export function lectureBadges(l: CourseLecture): LectureBadge[] {
  const state = lectureState(l);
  const badges: LectureBadge[] = [{ key: state, ...STATE_BADGES[state] }];
  if (l.private) badges.push({ key: "private", label: "Private", tone: "muted" });
  if (l.premiere) badges.push({ key: "premiere", label: "Premiere", tone: "muted" });
  badges.push(
    l.chatEnabled
      ? { key: "chat", label: "Chat on", tone: "muted" }
      : { key: "chat", label: "Chat off", tone: "muted" },
  );
  // Only where it still matters: a recorded or past lecture streams from nowhere.
  if (l.lectureHallId === 0 && state !== "recording" && state !== "converting" && state !== "past") {
    badges.push({ key: "self-stream", label: "Self-stream", tone: "muted" });
  }
  return badges;
}

/** "1:05:09" or "45:00", or "" when not known. */
export function formatDuration(seconds: number): string {
  if (seconds <= 0) return "";
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = seconds % 60;
  const mm = String(m).padStart(h > 0 ? 2 : 1, "0");
  const ss = String(s).padStart(2, "0");
  return h > 0 ? `${h}:${mm}:${ss}` : `${mm}:${ss}`;
}

/** How many lectures of the list share `l`'s series, `l` included; 0 for none. */
export function seriesSize(lectures: CourseLecture[], l: CourseLecture): number {
  if (!l.seriesIdentifier) return 0;
  return lectures.filter((o) => o.seriesIdentifier === l.seriesIdentifier).length;
}

/** Oldest first when `ascending`, newest first otherwise; ties by ID. */
export function sortLectures(lectures: CourseLecture[], ascending: boolean): CourseLecture[] {
  const sign = ascending ? 1 : -1;
  return [...lectures].sort(
    (a, b) => sign * (a.start.getTime() - b.start.getTime() || a.id - b.id),
  );
}

/*
 * The sort order, kept where the server-rendered page's Alpine $persist kept it:
 * `_x_` is $persist's prefix and the value its JSON, so a preference set there
 * carries over.
 */
const SORT_KEY = "_x_courseStreamsSortOrder";

export function loadSortAscending(): boolean {
  try {
    return JSON.parse(localStorage.getItem(SORT_KEY) ?? "false") === true;
  } catch {
    return false;
  }
}

export function saveSortAscending(ascending: boolean): void {
  try {
    localStorage.setItem(SORT_KEY, JSON.stringify(ascending));
  } catch {
    // Private mode or blocked storage: the choice lasts for this page only.
  }
}

/** The lecture `#lecture-<id>` names, or null. */
export function lectureFromHash(hash: string): number | null {
  const m = /^#lecture-(\d+)$/.exec(hash);
  return m ? Number(m[1]) : null;
}

/** What to tell the person when one of these calls fails. */
export function lectureErrorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.isUnauthenticated) return "Your session expired. Please sign in again.";
    // Another course's lecture, and a course the caller does not administer, answer
    // the same as a missing one.
    if (err.status === 404) return "This lecture no longer exists or you do not administer it.";
    return err.message;
  }
  return "Something went wrong. Please try again.";
}
