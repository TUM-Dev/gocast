/**
 * What a lecture's card does to its content: sections, attachments, the custom
 * thumbnail, subtitles and the transcoding progress, over the lecture content RPCs
 * and the two uploads beside them.
 */

import { fromJson, type JsonValue } from "@bufbuild/protobuf";

import {
  CreateLectureSectionsResponseSchema,
  GetLectureTranscodingProgressResponseSchema,
  LectureFileAdminSchema,
} from "@/gen/server/apiv2_pb";
import { ApiError, apiDelete, apiGetMessage, apiPost, apiPut, apiUpload } from "./api";
import { FILE_TYPE_ATTACHMENT, type CourseLecture, type LectureSection } from "./course-lectures";

/* model.FileType, as far as the card tells the kinds apart. */
export const FILE_TYPE_VOD = 1;
export const FILE_TYPE_THUMB_CUSTOM = 11;

export type LectureFile = CourseLecture["files"][number];

export function attachmentsOf(files: LectureFile[]): LectureFile[] {
  return files.filter((f) => f.type === FILE_TYPE_ATTACHMENT);
}

/** The uploaded thumbnail, which wins over the generated ones; undefined for none. */
export function customThumbnailOf(files: LectureFile[]): LectureFile | undefined {
  return files.find((f) => f.type === FILE_TYPE_THUMB_CUSTOM);
}

/** The recording files a course administrator may download. */
export function downloadsOf(files: LectureFile[]): LectureFile[] {
  return files.filter((f) => f.type === FILE_TYPE_VOD);
}

/**
 * Where the browser fetches a file: `serve` shows an image inline, `download` hands
 * the file over as an attachment. Both through the session cookie, so a plain <img>
 * or link works without the bearer token.
 */
export function fileUrl(id: number, as: "serve" | "download"): string {
  return `/api/download/${id}?type=${as}`;
}

/* Sections. */

/** "4:02" or "1:04:02", as the player names a chapter's start. */
export function formatSectionStart(s: Pick<LectureSection, "startHours" | "startMinutes" | "startSeconds">): string {
  const ss = String(s.startSeconds).padStart(2, "0");
  if (s.startHours > 0) {
    return `${s.startHours}:${String(s.startMinutes).padStart(2, "0")}:${ss}`;
  }
  return `${s.startMinutes}:${ss}`;
}

/** What the section form holds: number inputs bound with v-model give "" when empty. */
export interface SectionForm {
  hours: number | string;
  minutes: number | string;
  seconds: number | string;
  description: string;
}

export interface SectionInput {
  description: string;
  startHours: number;
  startMinutes: number;
  startSeconds: number;
}

export interface SectionProblems {
  description?: string;
  time?: string;
}

export type SectionCheck = { ok: true; input: SectionInput } | { ok: false; problems: SectionProblems };

export function emptySectionForm(): SectionForm {
  return { hours: "", minutes: "", seconds: "", description: "" };
}

export function sectionForm(s: LectureSection): SectionForm {
  return {
    hours: s.startHours,
    minutes: s.startMinutes,
    seconds: s.startSeconds,
    description: s.description,
  };
}

/** A whole, non-negative number, an empty field counting as 0; NaN otherwise. */
function timePart(value: number | string): number {
  if (typeof value === "string") {
    const trimmed = value.trim();
    if (trimmed === "") return 0;
    if (!/^\d+$/.test(trimmed)) return Number.NaN;
    return Number(trimmed);
  }
  return Number.isInteger(value) && value >= 0 ? value : Number.NaN;
}

/**
 * Checks a section as createLectureSections and updateLectureSection do, so the form
 * says what is wrong before the server refuses it: a description, and minutes and
 * seconds below 60. Hours have no upper bound there either.
 */
export function checkSection(form: SectionForm): SectionCheck {
  const problems: SectionProblems = {};
  const description = form.description.trim();
  if (!description) problems.description = "Give the section a description.";

  const [h, m, s] = [timePart(form.hours), timePart(form.minutes), timePart(form.seconds)];
  if ([h, m, s].some(Number.isNaN)) {
    problems.time = "Use whole numbers for the start.";
  } else if (m >= 60 || s >= 60) {
    problems.time = "Minutes and seconds must be below 60.";
  }

  if (problems.description || problems.time) return { ok: false, problems };
  return { ok: true, input: { description, startHours: h, startMinutes: m, startSeconds: s } };
}

/** Whether the form differs from the section it edits. */
export function sectionChanged(s: LectureSection, input: SectionInput): boolean {
  return (
    s.description !== input.description ||
    s.startHours !== input.startHours ||
    s.startMinutes !== input.startMinutes ||
    s.startSeconds !== input.startSeconds
  );
}

const lecturePath = (courseId: number, streamId: number) => `/courses/${courseId}/streams/${streamId}`;

/** The IDs of all of the lecture's sections afterwards, in order of their start. */
export async function createLectureSections(
  courseId: number,
  streamId: number,
  sections: SectionInput[],
): Promise<number[]> {
  const json = await apiPost<JsonValue>(`${lecturePath(courseId, streamId)}/sections`, { sections });
  const res = fromJson(CreateLectureSectionsResponseSchema, json, { ignoreUnknownFields: true });
  return res.sections.map((s) => s.id);
}

export async function updateLectureSection(
  courseId: number,
  streamId: number,
  sectionId: number,
  input: SectionInput,
): Promise<void> {
  await apiPut(`${lecturePath(courseId, streamId)}/sections/${sectionId}`, input);
}

export async function deleteLectureSection(courseId: number, streamId: number, sectionId: number): Promise<void> {
  await apiDelete(`${lecturePath(courseId, streamId)}/sections/${sectionId}`);
}

/* Files. */

function fileForm(file: File): FormData {
  const form = new FormData();
  form.append("file", file);
  return form;
}

async function upload(path: string, file: File): Promise<LectureFile> {
  const json = await apiUpload<JsonValue>(path, fileForm(file));
  const f = fromJson(LectureFileAdminSchema, json, { ignoreUnknownFields: true });
  return { id: f.id, type: f.type, friendlyName: f.friendlyName };
}

/** The 50 MB the server takes, checked here so a large file is not sent for nothing. */
export const MAX_UPLOAD_BYTES = 50 * 1000 * 1000;

export function uploadLectureAttachment(courseId: number, streamId: number, file: File): Promise<LectureFile> {
  return upload(`${lecturePath(courseId, streamId)}/attachments`, file);
}

export async function deleteLectureAttachment(courseId: number, streamId: number, fileId: number): Promise<void> {
  await apiDelete(`${lecturePath(courseId, streamId)}/attachments/${fileId}`);
}

/** The image formats the server takes as a thumbnail. */
export const THUMBNAIL_ACCEPT = ".jpg,.jpeg,.png,.gif,.webp";

/** Replaces any custom thumbnail the lecture had. */
export function uploadLectureThumbnail(courseId: number, streamId: number, file: File): Promise<LectureFile> {
  return upload(`${lecturePath(courseId, streamId)}/thumbnail`, file);
}

export async function deleteLectureThumbnail(courseId: number, streamId: number): Promise<void> {
  await apiDelete(`${lecturePath(courseId, streamId)}/thumbnail`);
}

/** Why a file cannot be uploaded, before sending it; "" when it can. */
export function uploadProblem(file: File, kind: "attachment" | "thumbnail"): string {
  if (file.size > MAX_UPLOAD_BYTES) return "The file is too large (the limit is 50 MB).";
  if (kind === "thumbnail") {
    const ext = file.name.slice(file.name.lastIndexOf(".")).toLowerCase();
    if (!file.type.startsWith("image/") || !THUMBNAIL_ACCEPT.split(",").includes(ext)) {
      return "The thumbnail must be a JPG, PNG, GIF or WebP image.";
    }
  }
  return "";
}

/* Subtitles. */

export const SUBTITLE_LANGUAGES = [
  { code: "de", label: "German" },
  { code: "en", label: "English" },
] as const;

export async function requestLectureSubtitles(courseId: number, streamId: number, language: string): Promise<void> {
  await apiPost(`${lecturePath(courseId, streamId)}/subtitles`, { language });
}

/* Transcoding progress. */

export type TranscodingProgress = CourseLecture["transcodingProgresses"][number];

export async function fetchTranscodingProgress(courseId: number, streamId: number): Promise<TranscodingProgress[]> {
  const res = await apiGetMessage(
    GetLectureTranscodingProgressResponseSchema,
    `${lecturePath(courseId, streamId)}/transcoding-progress`,
  );
  return res.progresses.map((p) => ({ version: p.version, progress: p.progress }));
}

export const PROGRESS_POLL_MS = 4000;

/** Polling is worth it only while a version is still transcoding. */
export function shouldPollProgress(converting: boolean, progresses: TranscodingProgress[]): boolean {
  return converting && progresses.length > 0;
}

export interface ProgressPoll {
  fetch: () => Promise<TranscodingProgress[]>;
  /** Every answer that still lists a version. */
  onProgress: (progresses: TranscodingProgress[]) => void;
  /** Once nothing is transcoding any more; polling has stopped. */
  onDone: () => void;
  intervalMs?: number;
}

/**
 * Asks for the progress every few seconds until the server lists nothing still
 * transcoding, one request at a time: the next is scheduled only once the last has
 * answered, so a slow server is not stacked up. A failed request is retried at the
 * next tick, except a 401 or 404 (signed out, or the lecture is gone), which stops.
 *
 * Returns the function that stops it, for when the card goes away.
 */
export function pollTranscodingProgress(poll: ProgressPoll): () => void {
  const interval = poll.intervalMs ?? PROGRESS_POLL_MS;
  let stopped = false;
  let timer: ReturnType<typeof setTimeout> | undefined;

  const schedule = () => {
    if (!stopped) timer = setTimeout(tick, interval);
  };

  async function tick(): Promise<void> {
    try {
      const progresses = await poll.fetch();
      if (stopped) return;
      if (progresses.length === 0) {
        stopped = true;
        poll.onDone();
        return;
      }
      poll.onProgress(progresses);
    } catch (err) {
      if (err instanceof ApiError && (err.status === 401 || err.status === 404)) {
        stopped = true;
        return;
      }
    }
    schedule();
  }

  schedule();
  return () => {
    stopped = true;
    if (timer !== undefined) clearTimeout(timer);
  };
}

/* Errors. */

/** What to tell the person when one of these calls fails. */
export function contentErrorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.isUnauthenticated) return "Your session expired. Please sign in again.";
    // A section or file of another lecture, a lecture of another course and a course
    // the caller does not administer all answer the same as a missing one.
    if (err.status === 404) return "It no longer exists, or you do not administer this course. Reload the page.";
    return err.message;
  }
  return "Something went wrong. Please try again.";
}

export function subtitleErrorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    // No voice service configured, or it did not answer.
    if (err.status === 503) return "Subtitle generation is not available right now.";
    if (err.status === 400 && err.message === "the lecture has no recording") {
      return "This lecture has no recording to generate subtitles from.";
    }
  }
  return contentErrorMessage(err);
}
