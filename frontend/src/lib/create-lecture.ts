/**
 * Creating lectures from the course page: the old edit-course page's "create lecture"
 * wizard, over createLectures and the VOD media upload beside the gateway.
 *
 * Everything here is free of Vue so the date stepping and the request it leads to can
 * be tested on their own; components/admin/lectures/create holds the wizard itself.
 */

import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";

import {
  CreateLecturesRequestSchema,
  CreateLecturesResponseSchema,
  LectureCreationKind,
  type CreateLecturesRequest,
} from "@/gen/server/apiv2_pb";
import { ApiError, apiPostMessage, apiUpload } from "./api";
import { toLecture, type CourseLecture } from "./course-lectures";
import { fromDateTimeLocal } from "./datetime-local";

/**
 * What is being created. v1's form also had in-browser recording, dropped on purpose
 * (it was experimental), and a premiere flag nothing on the page ever set.
 */
export type CreateMode = "scheduled" | "adhoc" | "vod";

export type RecurringInterval = "daily" | "weekly" | "monthly";

/** The server's bound on one request (maxLecturesPerRequest), the first lecture included. */
export const MAX_LECTURES = 200;

/**
 * An ad-hoc livestream starts this long after the request, as the server has it
 * (adHocDelay): the stream cron only picks up lectures that are due.
 */
export const AD_HOC_DELAY_MINUTES = 2;

export type MediaType = "COMB" | "PRES" | "CAM";

/** In the order they are uploaded, as v1 did. */
export const MEDIA_TYPES: { type: MediaType; label: string }[] = [
  { type: "COMB", label: "combined video" },
  { type: "PRES", label: "presentation video" },
  { type: "CAM", label: "camera video" },
];

/** One lecture of a series, as the review list shows it. */
export interface SeriesRow {
  start: Date;
  /** The first lecture is always created; the others can be left out, as in v1. */
  enabled: boolean;
  /** Its own title, "" for the series' title. */
  title: string;
}

export interface CreateLectureForm {
  mode: CreateMode;
  title: string;
  /** datetime-local values; `end` only for a scheduled livestream. */
  start: string;
  end: string;
  /** "HH:MM", when an ad-hoc livestream ends: today, or tomorrow if that has passed. */
  endTime: string;
  /** 0 for none (self-streamed). Only server administrators are offered a hall. */
  lectureHallId: number;
  chatEnabled: boolean;
  recurring: boolean;
  interval: RecurringInterval;
  /** How many lectures the series has, the first included (v1's eventsCount). */
  count: number;
  /** The series' lectures, the first included; ignored unless recurring. */
  rows: SeriesRow[];
  files: Partial<Record<MediaType, File>>;
}

export function emptyForm(chatEnabled: boolean): CreateLectureForm {
  return {
    mode: "scheduled",
    title: "",
    start: "",
    end: "",
    endTime: "",
    lectureHallId: 0,
    chatEnabled,
    recurring: false,
    interval: "weekly",
    count: 10,
    rows: [],
    files: {},
  };
}

/* Series dates. */

function daysInMonth(year: number, month: number): number {
  // Day 0 of the next month is the last of this one; the constructor rolls the month.
  return new Date(year, month + 1, 0).getDate();
}

/**
 * The `n`th lecture of a series starting at `first` (0 is `first` itself), at the same
 * wall-clock time. Stepped on the calendar fields rather than by adding milliseconds,
 * so a series crossing a daylight-saving change keeps its time of day, and each date
 * is counted from the first rather than from the one before: a monthly series on the
 * 31st falls on the last day of the shorter months and comes back to the 31st after,
 * where v1's chained setMonth rolled 31 January into 3 March and stayed on the 3rd.
 */
export function seriesDate(first: Date, interval: RecurringInterval, n: number): Date {
  const y = first.getFullYear();
  const m = first.getMonth();
  const d = first.getDate();
  const h = first.getHours();
  const min = first.getMinutes();
  switch (interval) {
    case "daily":
      return new Date(y, m, d + n, h, min);
    case "weekly":
      return new Date(y, m, d + 7 * n, h, min);
    case "monthly": {
      // Built from the first of the month so the constructor cannot roll it over.
      const month = new Date(y, m + n, 1);
      const day = Math.min(d, daysInMonth(month.getFullYear(), month.getMonth()));
      return new Date(month.getFullYear(), month.getMonth(), day, h, min);
    }
  }
}

/** The number of lectures a series can have, kept within 1 and MAX_LECTURES. */
export function clampCount(count: number): number {
  if (!Number.isFinite(count)) return 1;
  return Math.min(MAX_LECTURES, Math.max(1, Math.floor(count)));
}

/**
 * The series' rows for `count` lectures from `first`. A row that was there before
 * keeps its title and whether it is enabled, so changing the count or the start does
 * not throw away what was typed into the list.
 */
export function seriesRows(
  first: Date,
  interval: RecurringInterval,
  count: number,
  previous: SeriesRow[] = [],
): SeriesRow[] {
  return Array.from({ length: clampCount(count) }, (_, i) => ({
    start: seriesDate(first, interval, i),
    enabled: i === 0 || (previous[i]?.enabled ?? true),
    title: previous[i]?.title ?? "",
  }));
}

/* Durations. */

/** Whole minutes from `start` to `end`. */
function minutesBetween(start: Date, end: Date): number {
  return Math.round((end.getTime() - start.getTime()) / 60_000);
}

/**
 * When an ad-hoc livestream ending at `endTime` ("HH:MM") ends: today, or tomorrow
 * when that time has passed, as v1 rolled it. Null for a malformed time.
 */
export function adHocEnd(endTime: string, now: Date): Date | null {
  const m = /^(\d{2}):(\d{2})$/.exec(endTime);
  if (!m) return null;
  const [hours, minutes] = [Number(m[1]), Number(m[2])];
  if (hours > 23 || minutes > 59) return null;
  const end = new Date(now.getFullYear(), now.getMonth(), now.getDate(), hours, minutes);
  if (end <= now) {
    return new Date(now.getFullYear(), now.getMonth(), now.getDate() + 1, hours, minutes);
  }
  return end;
}

/**
 * How long each lecture runs, in minutes, or null when the form does not say yet. A
 * VOD upload has no end of its own: 0 asks the server for its placeholder hour.
 */
export function durationMinutes(form: CreateLectureForm, now: Date): number | null {
  switch (form.mode) {
    case "vod":
      return 0;
    case "scheduled": {
      const start = fromDateTimeLocal(form.start);
      const end = fromDateTimeLocal(form.end);
      return start && end ? minutesBetween(start, end) : null;
    }
    case "adhoc": {
      const end = adHocEnd(form.endTime, now);
      if (!end) return null;
      // The server starts it a little after now; it should still end when asked to.
      const start = new Date(now.getTime() + AD_HOC_DELAY_MINUTES * 60_000);
      return minutesBetween(start, end);
    }
  }
}

/** "1h 30min", "45min", or "" for none. */
export function formatMinutes(minutes: number): string {
  if (minutes <= 0) return "";
  const h = Math.floor(minutes / 60);
  const m = minutes % 60;
  return [h > 0 ? `${h}h` : "", m > 0 ? `${m}min` : ""].filter(Boolean).join(" ");
}

/* Validation. */

/** The starts the form creates lectures at, in the order the server answers them. */
export function lectureStarts(form: CreateLectureForm): Date[] {
  if (form.mode === "adhoc") return [];
  const start = fromDateTimeLocal(form.start);
  if (!start) return [];
  if (form.mode !== "scheduled" || !form.recurring) return [start];
  // The first row is the start itself, whatever the list was built from.
  return [start, ...form.rows.slice(1).filter((r) => r.enabled).map((r) => r.start)];
}

/** How many lectures the form creates. */
export function lectureCount(form: CreateLectureForm): number {
  return form.mode === "adhoc" ? 1 : Math.max(1, lectureStarts(form).length);
}

/** True for what the media upload takes: an mp4. */
export function isMp4(file: File): boolean {
  return file.type === "video/mp4" || file.name.toLowerCase().endsWith(".mp4");
}

/**
 * What keeps the details step from going on, in the order the form shows the fields;
 * empty when nothing does. The server checks all of it again.
 */
export function detailsProblems(form: CreateLectureForm, now: Date): string[] {
  const problems: string[] = [];
  if (!form.title.trim()) problems.push("Enter a title.");

  if (form.mode === "adhoc") {
    const minutes = durationMinutes(form, now);
    if (minutes === null) problems.push("Enter when the livestream ends.");
    else if (minutes < 1) problems.push("The end must be a few minutes from now.");
    return problems;
  }

  if (!fromDateTimeLocal(form.start)) problems.push("Enter a start.");
  if (form.mode === "scheduled") {
    if (!fromDateTimeLocal(form.end)) problems.push("Enter an end.");
    else if ((durationMinutes(form, now) ?? 0) < 1) problems.push("The end must be after the start.");

    if (form.recurring && fromDateTimeLocal(form.start)) {
      const starts = lectureStarts(form);
      if (starts.length > MAX_LECTURES) {
        problems.push(`At most ${MAX_LECTURES} lectures can be created at once.`);
      }
      // The server refuses a date given twice; the stepping never makes one, but a
      // list built from an older start could.
      if (new Set(starts.map((d) => d.getTime())).size !== starts.length) {
        problems.push("Two lectures of the series start at the same time.");
      }
    }
  }
  return problems;
}

/** What keeps a VOD upload's files from being sent; empty when nothing does. */
export function filesProblems(form: CreateLectureForm): string[] {
  const chosen = MEDIA_TYPES.filter(({ type }) => form.files[type]);
  // As v1: a VOD upload is created for its recording, so it needs at least one.
  if (!chosen.length) return ["Choose at least one video."];
  return chosen
    .filter(({ type }) => !isMp4(form.files[type]!))
    .map(({ label }) => `The ${label} must be an mp4 file.`);
}

/* The request. */

export interface CreatePlan {
  request: CreateLecturesRequest;
  /**
   * The title each lecture created gets besides the series' title, in the order the
   * server answers them; "" for none. createLectures gives every lecture the same
   * title, so these are set afterwards, one updateLecture each.
   */
  titles: string[];
}

/**
 * The createLectures request for a form detailsProblems has nothing against.
 * `start` is the first lecture and `date_series` only the further ones: the server
 * refuses a date given twice.
 */
export function buildCreatePlan(courseId: number, form: CreateLectureForm, now: Date): CreatePlan {
  const title = form.title.trim();
  const livestream = form.mode !== "vod";
  const request = create(CreateLecturesRequestSchema, {
    courseId,
    title,
    kind: livestream ? LectureCreationKind.LIVESTREAM : LectureCreationKind.VOD_UPLOAD,
    durationMinutes: Math.max(0, durationMinutes(form, now) ?? 0),
    // The server refuses a hall for anything but a livestream.
    lectureHallId: livestream ? form.lectureHallId : 0,
    chatEnabled: form.chatEnabled,
    adHoc: form.mode === "adhoc",
  });

  if (form.mode === "adhoc") return { request, titles: [""] };

  const [first, ...further] = lectureStarts(form);
  if (first) request.start = timestampFromDate(first);
  request.dateSeries = further.map((d) => timestampFromDate(d));

  const rows =
    form.mode === "scheduled" && form.recurring ? [form.rows[0], ...form.rows.slice(1).filter((r) => r.enabled)] : [];
  const titles = lectureStarts(form).map((_, i) => {
    const own = rows[i]?.title.trim() ?? "";
    return own && own !== title ? own : "";
  });
  return { request, titles };
}

/** The lectures created, in the order of the request's dates. */
export async function createLectures(courseId: number, request: CreateLecturesRequest): Promise<CourseLecture[]> {
  const res = await apiPostMessage(
    CreateLecturesRequestSchema,
    CreateLecturesResponseSchema,
    `/courses/${courseId}/streams`,
    request,
  );
  return res.lectures.map(toLecture);
}

/** Hands a VOD upload's recording to a worker, which transcodes it into that version. */
export async function uploadLectureMedia(
  courseId: number,
  streamId: number,
  type: MediaType,
  file: File,
): Promise<void> {
  const form = new FormData();
  form.append("file", file);
  await apiUpload(`/courses/${courseId}/streams/${streamId}/media?type=${type}`, form);
}

/* Messages. */

export function createErrorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.isUnauthenticated) return "Your session expired. Please sign in again.";
    // A course the caller does not administer answers the same as a missing one.
    if (err.status === 404) return "This course does not exist or you do not administer it.";
    if (err.status === 400) return `The lecture could not be created: ${err.message}.`;
    return err.message;
  }
  return "Something went wrong. Please try again.";
}

export function mediaUploadErrorMessage(err: unknown, label: string): string {
  if (err instanceof ApiError) {
    if (err.status === 503) {
      return "No worker is available to receive the upload right now. The lecture was created; upload the video later from its card.";
    }
    if (err.isUnauthenticated) return "Your session expired. Please sign in again.";
    return `The ${label} could not be uploaded: ${err.message}. The lecture was created.`;
  }
  return `The ${label} could not be uploaded. The lecture was created.`;
}
