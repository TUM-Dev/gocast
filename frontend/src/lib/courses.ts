/**
 * Courses, and the caller's relationship to them.
 *
 * Which listing to ask for depends on who is asking: the public and live listings
 * answer anonymous callers but include more for a signed-in one, while the enrolled
 * and pinned listings need a user and 401 without one.
 */

import { create } from "@bufbuild/protobuf";

import {
  GetLiveCoursesResponseSchema,
  GetPinnedCoursesResponseSchema,
  GetPublicCoursesResponseSchema,
  GetUserCoursesResponseSchema,
  GetCourseBySlugResponseSchema,
  PinCourseRequestSchema,
  PinCourseResponseSchema,
  type Course as CourseMessage,
  type CourseStream as CourseStreamMessage,
  type LectureHall as LectureHallMessage,
} from "@/gen/server/apiv2_pb";
import { apiGetMessage, apiGetMessageOptionalAuth, apiPostMessage } from "./api";
import type { Semester, TeachingTerm } from "./semesters";
import { parseStream, type Stream } from "./streams";

/** How much of the site a course is offered to. Mirrors model.Course.Visibility. */
export type Visibility = "public" | "loggedin" | "enrolled" | "hidden";

export interface Course {
  id: number;
  name: string;
  slug: string;
  semester: Semester;
  visibility: Visibility;
  /** Whether the calling user has pinned it. Always false when anonymous. */
  pinned: boolean;
  /** Whether the calling user administers this course specifically. */
  isAdmin: boolean;
  downloadsEnabled: boolean;
  vodEnabled: boolean;
  /** The most recent recorded lecture, absent when the course has none. */
  lastRecording?: Stream;
  /** The next lecture that has not ended, absent when the course has none. */
  nextLecture?: Stream;
  /** Only the course endpoint fills this; the listings leave it empty. */
  streams: Stream[];
}

export interface LectureHall {
  name: string;
  externalUrl: string;
}

/** A lecture that is live right now, with the course it belongs to. */
export interface LiveStream {
  course: Course;
  stream: Stream;
  /** Absent for a lecture streamed from somewhere other than a lecture hall. */
  lectureHall?: LectureHall;
}

/** Hidden courses are unlisted rather than private, and are marked as such. */
export function isHidden(course: Course): boolean {
  return course.visibility === "hidden";
}

/** Sorts by name, as the server-rendered start page ordered every listing. */
export function byName(a: Course, b: Course): number {
  return a.name.localeCompare(b.name);
}

/** The client route for a course. */
export function courseUrl(course: Course): string {
  return `/course/${course.semester.year}/${course.semester.term}/${course.slug}`;
}

/**
 * The calendar feed, which only the v1 API serves. A plain link: the browser
 * authenticates it with the session cookie, as it did before the migration.
 */
export function calendarUrl(course: Course): string {
  const { year, term } = course.semester;
  return `/api/download_ics/${year}/${term}/${encodeURIComponent(course.slug)}/events.ics`;
}

function parseLectureHall(message: LectureHallMessage): LectureHall {
  return { name: message.name, externalUrl: message.externalUrl };
}

function parseCourse(message: CourseMessage): Course {
  return {
    id: message.id,
    name: message.name,
    slug: message.slug,
    semester: {
      year: message.semester?.year ?? 0,
      term: message.semester?.teachingTerm === "S" ? "S" : "W",
    },
    // Anything the server has not classified is treated as the most restricted of
    // the four rather than the most open.
    visibility: (message.visibility || "enrolled") as Visibility,
    pinned: message.pinned,
    isAdmin: message.isAdmin,
    downloadsEnabled: message.downloadsEnabled,
    vodEnabled: message.vodEnabled,
    lastRecording: message.lastRecording ? parseStream(message.lastRecording) : undefined,
    nextLecture: message.nextLecture ? parseStream(message.nextLecture) : undefined,
    streams: message.streams.map(parseStream),
  };
}

function parseLiveStream(message: CourseStreamMessage): LiveStream | null {
  // Both are always sent together; a half-filled entry is not something to render.
  if (!message.course || !message.stream) return null;

  return {
    course: parseCourse(message.course),
    stream: parseStream(message.stream),
    lectureHall: message.lectureHall ? parseLectureHall(message.lectureHall) : undefined,
  };
}

/** `?year=&term=`, omitted entirely for the current semester. */
function semesterQuery(semester?: Partial<Semester>): string {
  const params = new URLSearchParams();
  if (semester?.year !== undefined) params.set("year", String(semester.year));
  if (semester?.term !== undefined) params.set("term", semester.term);
  const query = params.toString();
  return query === "" ? "" : `?${query}`;
}

/** Courses anyone may watch, plus the logged-in-only ones when a user is signed in. */
export async function fetchPublicCourses(semester?: Partial<Semester>): Promise<Course[]> {
  const res = await apiGetMessageOptionalAuth(
    GetPublicCoursesResponseSchema,
    `/courses${semesterQuery(semester)}`,
  );
  return res.courses.map(parseCourse).sort(byName);
}

/** The caller's own courses: enrolled, administered, or all of them for an admin. */
export async function fetchUserCourses(semester?: Partial<Semester>): Promise<Course[]> {
  const res = await apiGetMessage(
    GetUserCoursesResponseSchema,
    `/courses/enrolled${semesterQuery(semester)}`,
  );
  return res.courses.map(parseCourse).sort(byName);
}

/**
 * The caller's pinned courses, across all semesters — a pin is not scoped to one, and
 * the endpoint takes no semester.
 */
export async function fetchPinnedCourses(): Promise<Course[]> {
  const res = await apiGetMessage(GetPinnedCoursesResponseSchema, "/courses/pinned");
  return res.courses.map(parseCourse).sort(byName);
}

/** Every lecture live right now that the caller may see. */
export async function fetchLiveStreams(): Promise<LiveStream[]> {
  const res = await apiGetMessageOptionalAuth(GetLiveCoursesResponseSchema, "/courses/live");
  return res.liveCourses.map(parseLiveStream).filter((s): s is LiveStream => s !== null);
}

/**
 * One course with its lectures. Unlike the listings this reaches hidden courses, which
 * are unlisted rather than private, so a direct link still opens.
 */
export async function fetchCourse(slug: string, semester?: Partial<Semester>): Promise<Course> {
  const res = await apiGetMessageOptionalAuth(
    GetCourseBySlugResponseSchema,
    `/courses/${encodeURIComponent(slug)}${semesterQuery(semester)}`,
  );

  if (!res.course) {
    throw new Error(`no course in the response for ${slug}`);
  }
  return parseCourse(res.course);
}

/** Pins or unpins a course for the calling user. */
export async function setCoursePinned(courseId: number, pinned: boolean): Promise<void> {
  await apiPostMessage(
    PinCourseRequestSchema,
    PinCourseResponseSchema,
    `/courses/${courseId}/pin`,
    create(PinCourseRequestSchema, { courseId, pin: pinned }),
  );
}

export type { Semester, TeachingTerm };
