/**
 * A lecture's units: named, described ranges of its recording. The page reads them
 * with the recording to preview against, adds one at a time and removes them.
 */

import { create } from "@bufbuild/protobuf";
import DOMPurify from "dompurify";
import { marked } from "marked";

import {
  CreateLectureUnitRequestSchema,
  ListLectureUnitsResponseSchema,
  LectureUnitSchema,
} from "@/gen/server/apiv2_pb";
import { ApiError, apiDelete, apiGetMessage, apiPostMessage } from "./api";

export interface LectureUnit {
  id: number;
  title: string;
  /** Markdown, as entered. */
  description: string;
  fromMs: number;
  toMs: number;
}

export interface LectureUnits {
  lectureName: string;
  /** Signed playlist of the recording; "" while the lecture has none. */
  playlistUrl: string;
  /** 0 when the server does not know the length (yet). */
  durationSeconds: number;
  units: LectureUnit[];
}

const unitsPath = (courseId: number, streamId: number) => `/courses/${courseId}/streams/${streamId}/units`;

export async function fetchLectureUnits(courseId: number, streamId: number): Promise<LectureUnits> {
  const res = await apiGetMessage(ListLectureUnitsResponseSchema, unitsPath(courseId, streamId));
  return {
    lectureName: res.lectureName,
    playlistUrl: res.playlistUrl,
    durationSeconds: res.durationSeconds,
    units: res.units.map((u) => ({
      id: u.id,
      title: u.title,
      description: u.description,
      fromMs: u.fromMs,
      toMs: u.toMs,
    })),
  };
}

export interface NewUnit {
  title: string;
  description: string;
  fromMs: number;
  toMs: number;
}

/** The stored unit, with its ID. */
export async function createLectureUnit(courseId: number, streamId: number, unit: NewUnit): Promise<LectureUnit> {
  const res = await apiPostMessage(
    CreateLectureUnitRequestSchema,
    LectureUnitSchema,
    unitsPath(courseId, streamId),
    create(CreateLectureUnitRequestSchema, { ...unit, courseId, streamId }),
  );
  return {
    id: res.id,
    title: res.title,
    description: res.description,
    fromMs: res.fromMs,
    toMs: res.toMs,
  };
}

export function deleteLectureUnit(courseId: number, streamId: number, unitId: number): Promise<void> {
  return apiDelete(`${unitsPath(courseId, streamId)}/${unitId}`);
}

/** Keeps the list in recording order, as the server answers it. */
export function sortUnits(units: LectureUnit[]): LectureUnit[] {
  return [...units].sort((a, b) => a.fromMs - b.fromMs || a.id - b.id);
}

const pad = (n: number) => String(n).padStart(2, "0");

/** "HH:MM:SS", whole seconds, as the old slider's tooltips showed. */
export function formatTimecode(ms: number): string {
  const total = Math.max(0, Math.floor(ms / 1000));
  return `${pad(Math.floor(total / 3600))}:${pad(Math.floor(total / 60) % 60)}:${pad(total % 60)}`;
}

/**
 * Milliseconds for "1:02:03", "02:03" or a plain number of seconds; null for anything
 * else. Minutes and seconds past 59 are refused rather than carried, since "0:75" is
 * more likely a typo than a minute and a quarter.
 */
export function parseTimecode(text: string): number | null {
  const parts = text.trim().split(":");
  if (parts.length > 3 || parts.some((p) => !/^\d+$/.test(p))) return null;
  const nums = parts.map(Number);
  if (nums.length > 1 && nums.slice(1).some((n) => n >= 60)) return null;
  let seconds = 0;
  for (const n of nums) seconds = seconds * 60 + n;
  return seconds * 1000;
}

export interface UnitDraft {
  title: string;
  description: string;
  from: string;
  to: string;
}

export type UnitCheck = { ok: true; unit: NewUnit } | { ok: false; problem: string };

/**
 * Checks a unit as createLectureUnit does, so the form says what is wrong before the
 * server refuses it. A known length also bounds the range, which the server cannot
 * (a recording may be longer than the duration it recorded).
 */
export function checkUnit(draft: UnitDraft, durationSeconds = 0): UnitCheck {
  const title = draft.title.trim();
  if (!title) return { ok: false, problem: "Give the unit a title." };

  const fromMs = parseTimecode(draft.from);
  const toMs = parseTimecode(draft.to);
  if (fromMs === null || toMs === null) {
    return { ok: false, problem: "Enter the start and end as hours:minutes:seconds, e.g. 0:05:30." };
  }
  if (toMs <= fromMs) return { ok: false, problem: "The unit must end after it starts." };
  if (durationSeconds > 0 && toMs > durationSeconds * 1000) {
    return { ok: false, problem: `The recording is only ${formatTimecode(durationSeconds * 1000)} long.` };
  }
  return { ok: true, unit: { title, description: draft.description, fromMs, toMs } };
}

/** The description as the old page rendered it: Markdown, sanitized. */
export function renderDescription(markdown: string): string {
  if (!markdown.trim()) return "";
  return DOMPurify.sanitize(marked.parse(markdown, { async: false }) as string);
}

export function unitsErrorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.isUnauthenticated) return "Your session expired. Please sign in again.";
    // A lecture outside the course answers the same as a missing one.
    if (err.status === 404) return "This lecture does not exist or you do not administer it.";
    return err.message;
  }
  return "Something went wrong. Please try again.";
}
