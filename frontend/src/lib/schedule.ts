/** The administration schedule: lectures of the caller's courses, on a calendar. */

import { timestampDate } from "@bufbuild/protobuf/wkt";

import {
  GetScheduleResponseSchema,
  ListScheduleLectureHallsResponseSchema,
} from "@/gen/server/apiv2_pb";
import { apiGetMessage, apiPatch } from "./api";

export interface ScheduledLecture {
  streamId: number;
  courseId: number;
  courseName: string;
  organization: string;
  name: string;
  description: string;
  start: Date;
  end: Date;
  /** 0 for a lecture in no hall, i.e. self-streamed. */
  lectureHallId: number;
  lectureHallName: string;
}

export interface ScheduleLectureHall {
  id: number;
  name: string;
}

/** Stands for lectures in no hall in a hall filter, as on the server. */
export const SELF_STREAMED = 0;

/** Applies the hall and organization filters without requiring a preset list of organizations. */
export function filterScheduleLectures(
  lectures: readonly ScheduledLecture[],
  halls: readonly number[] | "all",
  organization: string,
): ScheduledLecture[] {
  return lectures.filter(
    (lecture) =>
      (halls === "all" || halls.includes(lecture.lectureHallId)) &&
      (!organization || (lecture.organization.trim() || "Unassigned") === organization),
  );
}

/**
 * The lectures overlapping [from, to). `halls` limits them to those halls, with
 * SELF_STREAMED for lectures in none; "all" does not filter.
 */
export async function fetchSchedule(
  from: Date,
  to: Date,
  halls: number[] | "all",
): Promise<ScheduledLecture[]> {
  const params = new URLSearchParams({ from: from.toISOString(), to: to.toISOString() });
  if (halls === "all") {
    params.set("allLectureHalls", "true");
  } else {
    halls.forEach((id) => params.append("lectureHallIds", String(id)));
  }

  const res = await apiGetMessage(GetScheduleResponseSchema, `/schedule?${params}`);
  return res.lectures.map((l) => ({
    streamId: l.streamId,
    courseId: l.courseId,
    courseName: l.courseName,
    organization: l.organization,
    name: l.name,
    description: l.description,
    // Always sent; the epoch fallback only keeps a malformed answer from throwing.
    start: l.start ? timestampDate(l.start) : new Date(0),
    end: l.end ? timestampDate(l.end) : new Date(0),
    lectureHallId: l.lectureHallId,
    lectureHallName: l.lectureHallName,
  }));
}

export async function fetchScheduleLectureHalls(): Promise<ScheduleLectureHall[]> {
  const res = await apiGetMessage(ListScheduleLectureHallsResponseSchema, "/schedule/lecture-halls");
  return res.lectureHalls.map((h) => ({ id: h.id, name: h.name }));
}

/** Stable color assignment so an organization's lectures keep their color across views. */
export function scheduleOrganizationColor(organization: string): string {
  const value = organization.trim();
  if (!value) return "#475569";

  let hash = 2166136261;
  for (let i = 0; i < value.length; i++) {
    hash ^= value.charCodeAt(i);
    hash = Math.imul(hash, 16777619);
  }
  return `hsl(${(hash >>> 0) % 360} 72% 36%)`;
}

/** Sets whichever of name and description is given and leaves the other. */
export async function updateLecture(
  courseId: number,
  streamId: number,
  changes: { name?: string; description?: string },
): Promise<void> {
  await apiPatch(`/courses/${courseId}/streams/${streamId}`, changes);
}
