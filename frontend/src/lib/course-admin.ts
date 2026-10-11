/**
 * A course's administration page: its settings, administrators, lecture-hall sources
 * and external participants, plus the sidebar's tree of administered courses.
 *
 * Every course-scoped call answers 404 for a course the caller does not administer,
 * the same as for one that does not exist.
 */

import { create } from "@bufbuild/protobuf";

import {
  AddCourseAdminRequestSchema,
  CopyCourseRequestSchema,
  CopyCourseResponseSchema,
  CourseAdminSchema,
  CourseAdminUserSchema,
  CourseSourceMode,
  InviteCourseParticipantsRequestSchema,
  InviteCourseParticipantsResponseSchema,
  ListAdministeredCoursesResponseSchema,
  ListCourseAdminsResponseSchema,
  ListCourseIntegrationGrantsResponseSchema,
  ListCourseLectureHallSettingsResponseSchema,
  ListCourseParticipantsResponseSchema,
  SearchUsersForCourseResponseSchema,
  UpdateCourseLectureHallSettingsRequestSchema,
  UpdateCourseSettingsRequestSchema,
  type CourseAdmin as CourseAdminMessage,
  type CourseLectureHallSetting,
} from "@/gen/server/apiv2_pb";
import { apiDelete, apiGetMessage, apiPatchMessage, apiPostMessage, apiPutMessage } from "./api";
import { semesterLabel, type Semester, type TeachingTerm } from "./semesters";

export { CourseSourceMode };

export type CourseVisibility = "public" | "loggedin" | "enrolled" | "hidden";

export interface CourseAdmin {
  id: number;
  name: string;
  slug: string;
  year: number;
  term: TeachingTerm;
  visibility: CourseVisibility;
  vodEnabled: boolean;
  downloadsEnabled: boolean;
  chatEnabled: boolean;
  anonymousChatEnabled: boolean;
  moderatedChatEnabled: boolean;
  /** Read-only: nothing has ever set it. */
  vodChatEnabled: boolean;
  livePrivate: boolean;
  vodPrivate: boolean;
  /** "de", "en" or "". */
  language: string;
  tumOnlineIdentifier: string;
}

/** The settings updateCourseSettings changes; anything left out stays as it is. */
export type CourseSettings = Pick<
  CourseAdmin,
  | "visibility"
  | "vodEnabled"
  | "downloadsEnabled"
  | "chatEnabled"
  | "anonymousChatEnabled"
  | "moderatedChatEnabled"
  | "livePrivate"
  | "vodPrivate"
>;

function toVisibility(value: string): CourseVisibility {
  return value === "public" || value === "loggedin" || value === "enrolled" ? value : "hidden";
}

function toCourseAdmin(c: CourseAdminMessage): CourseAdmin {
  return {
    id: c.id,
    name: c.name,
    slug: c.slug,
    year: c.year,
    term: c.term === "S" ? "S" : "W",
    visibility: toVisibility(c.visibility),
    vodEnabled: c.vodEnabled,
    downloadsEnabled: c.downloadsEnabled,
    chatEnabled: c.chatEnabled,
    anonymousChatEnabled: c.anonymousChatEnabled,
    moderatedChatEnabled: c.moderatedChatEnabled,
    vodChatEnabled: c.vodChatEnabled,
    livePrivate: c.livePrivate,
    vodPrivate: c.vodPrivate,
    language: c.language,
    tumOnlineIdentifier: c.tumOnlineIdentifier,
  };
}

/** The public page of a course, which its name links to. */
export function publicCoursePath(course: Pick<CourseAdmin, "year" | "term" | "slug">): string {
  return `/course/${course.year}/${course.term}/${encodeURIComponent(course.slug)}`;
}

export async function fetchCourseAdmin(courseId: number): Promise<CourseAdmin> {
  return toCourseAdmin(await apiGetMessage(CourseAdminSchema, `/courses/${courseId}/admin`));
}

export async function updateCourseSettings(
  courseId: number,
  settings: Partial<CourseSettings>,
): Promise<CourseAdmin> {
  const updated = await apiPatchMessage(
    UpdateCourseSettingsRequestSchema,
    CourseAdminSchema,
    `/courses/${courseId}/settings`,
    create(UpdateCourseSettingsRequestSchema, { courseId, ...settings }),
  );
  return toCourseAdmin(updated);
}

export interface CopiedCourse {
  courseId: number;
  /** Lectures and administrators that could not be copied; the course itself was. */
  numErrors: number;
}

export async function copyCourse(courseId: number, target: Semester): Promise<CopiedCourse> {
  const res = await apiPostMessage(
    CopyCourseRequestSchema,
    CopyCourseResponseSchema,
    `/courses/${courseId}/copy`,
    create(CopyCourseRequestSchema, { courseId, year: target.year, term: target.term }),
  );
  return { courseId: res.courseId, numErrors: res.numErrors };
}

export async function deleteCourse(courseId: number): Promise<void> {
  await apiDelete(`/courses/${courseId}`);
}

// ----- Authorized applications -----

export interface CourseIntegrationGrant {
  id: number;
  name: string;
}

export async function fetchCourseIntegrationGrants(courseId: number): Promise<CourseIntegrationGrant[]> {
  const res = await apiGetMessage(ListCourseIntegrationGrantsResponseSchema, `/courses/${courseId}/integrations`);
  return res.grants.map((g) => ({ id: g.id, name: g.name }));
}

export async function revokeCourseIntegrationGrant(courseId: number, grantId: number): Promise<void> {
  await apiDelete(`/courses/${courseId}/integrations/${grantId}`);
}

// ----- Administrators -----

export interface CourseAdminUser {
  id: number;
  name: string;
  /** The email address, or the TUM login without one. */
  login: string;
  role: number;
}

function toUser(u: { id: number; name: string; login: string; role: number }): CourseAdminUser {
  return { id: u.id, name: u.name, login: u.login, role: u.role };
}

export async function fetchCourseAdmins(courseId: number): Promise<CourseAdminUser[]> {
  const res = await apiGetMessage(ListCourseAdminsResponseSchema, `/courses/${courseId}/admins`);
  return res.admins.map(toUser);
}

export async function addCourseAdmin(courseId: number, userId: number): Promise<CourseAdminUser> {
  const added = await apiPostMessage(
    AddCourseAdminRequestSchema,
    CourseAdminUserSchema,
    `/courses/${courseId}/admins`,
    create(AddCourseAdminRequestSchema, { courseId, userId }),
  );
  return toUser(added);
}

/** Answers 400 for the course's last administrator, with a message worth showing. */
export async function removeCourseAdmin(courseId: number, userId: number): Promise<void> {
  await apiDelete(`/courses/${courseId}/admins/${userId}`);
}

/** The server wants three letters or digits; this many characters is the client's cue. */
export const MIN_USER_QUERY = 3;

export async function searchUsersForCourse(courseId: number, q: string): Promise<CourseAdminUser[]> {
  const res = await apiGetMessage(
    SearchUsersForCourseResponseSchema,
    `/courses/${courseId}/admins/search?q=${encodeURIComponent(q)}`,
  );
  return res.users.map(toUser);
}

// ----- Lecture halls -----

export interface LectureHallPreset {
  presetId: number;
  name: string;
  image: string;
  isDefault: boolean;
}

export interface LectureHallSetting {
  lectureHallId: number;
  lectureHallName: string;
  presets: LectureHallPreset[];
  sourceMode: CourseSourceMode;
  /** 0 when the course chose none, and the hall's default preset applies. */
  selectedPresetId: number;
}

function toHallSetting(h: CourseLectureHallSetting): LectureHallSetting {
  return {
    lectureHallId: h.lectureHallId,
    lectureHallName: h.lectureHallName,
    presets: h.presets.map((p) => ({
      presetId: p.presetId,
      name: p.name,
      image: p.image,
      isDefault: p.isDefault,
    })),
    sourceMode: h.sourceMode,
    selectedPresetId: h.selectedPresetId,
  };
}

export async function fetchLectureHallSettings(courseId: number): Promise<LectureHallSetting[]> {
  const res = await apiGetMessage(
    ListCourseLectureHallSettingsResponseSchema,
    `/courses/${courseId}/lecture-halls`,
  );
  return res.lectureHalls.map(toHallSetting);
}

/** Replaces every hall's settings; a hall left out goes back to the defaults. */
export async function updateLectureHallSettings(
  courseId: number,
  halls: Pick<LectureHallSetting, "lectureHallId" | "sourceMode" | "selectedPresetId">[],
): Promise<LectureHallSetting[]> {
  const res = await apiPutMessage(
    UpdateCourseLectureHallSettingsRequestSchema,
    ListCourseLectureHallSettingsResponseSchema,
    `/courses/${courseId}/lecture-halls`,
    create(UpdateCourseLectureHallSettingsRequestSchema, {
      courseId,
      lectureHalls: halls.map((h) => ({
        lectureHallId: h.lectureHallId,
        sourceMode: h.sourceMode,
        selectedPresetId: h.selectedPresetId,
      })),
    }),
  );
  return res.lectureHalls.map(toHallSetting);
}

// ----- External participants -----

export interface CourseParticipant {
  id: number;
  name: string;
  email: string;
  /** Whether they have set a password yet. */
  accountSetUp: boolean;
}

export interface Invitee {
  name: string;
  email: string;
}

export interface InvitationResult {
  email: string;
  /** False when the address already had an account, which was enrolled instead. */
  accountCreated: boolean;
  /** Empty on success. */
  error: string;
}

export async function fetchCourseParticipants(courseId: number): Promise<CourseParticipant[]> {
  const res = await apiGetMessage(ListCourseParticipantsResponseSchema, `/courses/${courseId}/participants`);
  return res.participants.map((p) => ({
    id: p.id,
    name: p.name,
    email: p.email,
    accountSetUp: p.accountSetUp,
  }));
}

/**
 * Invites everyone or nobody: the server checks every invitee before inviting any,
 * and answers 400 naming the first bad one.
 */
export async function inviteCourseParticipants(
  courseId: number,
  invitees: Invitee[],
): Promise<InvitationResult[]> {
  const res = await apiPostMessage(
    InviteCourseParticipantsRequestSchema,
    InviteCourseParticipantsResponseSchema,
    `/courses/${courseId}/participants`,
    create(InviteCourseParticipantsRequestSchema, { courseId, invitees }),
  );
  return res.results.map((r) => ({ email: r.email, accountCreated: r.accountCreated, error: r.error }));
}

export interface ParsedBatch {
  invitees: Invitee[];
  /** 1-based line numbers that are not `name,email`. */
  badLines: number[];
}

/**
 * Reads the batch box: one `name,email` per line, the order the old form used
 * (`Tim,tim69@hotmail.com`). Blank lines are skipped. The old handler dropped any
 * other line without a word; this reports them so nobody is silently left out.
 */
export function parseInviteBatch(text: string): ParsedBatch {
  const invitees: Invitee[] = [];
  const badLines: number[] = [];
  text.split("\n").forEach((line, i) => {
    if (line.trim() === "") return;
    const parts = line.split(",");
    const name = parts[0]?.trim() ?? "";
    const email = parts[1]?.trim() ?? "";
    if (parts.length !== 2 || name === "" || !email.includes("@")) {
      badLines.push(i + 1);
      return;
    }
    invitees.push({ name, email });
  });
  return { invitees, badLines };
}

// ----- The sidebar's course tree -----

export interface AdministeredCourse {
  id: number;
  name: string;
  slug: string;
  year: number;
  term: TeachingTerm;
}

export interface SemesterGroup {
  semester: Semester;
  label: string;
  courses: AdministeredCourse[];
}

/** Newest semester first, as the server sorts them. */
export async function fetchAdministeredCourses(): Promise<AdministeredCourse[]> {
  const res = await apiGetMessage(ListAdministeredCoursesResponseSchema, "/courses/administered");
  return res.courses.map((c) => ({
    id: c.id,
    name: c.name,
    slug: c.slug,
    year: c.year,
    term: c.term === "S" ? "S" : "W",
  }));
}

/**
 * Groups courses by semester, newest first. Sorted here rather than trusting the
 * order they arrive in, so a change on the server cannot scramble the tree.
 */
export function groupBySemester(courses: AdministeredCourse[]): SemesterGroup[] {
  const groups = new Map<string, SemesterGroup>();
  for (const course of courses) {
    const key = `${course.year}-${course.term}`;
    let group = groups.get(key);
    if (!group) {
      const semester = { year: course.year, term: course.term };
      group = { semester, label: semesterLabel(semester), courses: [] };
      groups.set(key, group);
    }
    group.courses.push(course);
  }
  // A year's winter term starts after its summer term.
  const rank = (s: Semester) => s.year * 2 + (s.term === "W" ? 1 : 0);
  const sorted = [...groups.values()].sort((a, b) => rank(b.semester) - rank(a.semester));
  for (const group of sorted) {
    group.courses.sort((a, b) => a.name.localeCompare(b.name));
  }
  return sorted;
}
