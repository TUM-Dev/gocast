/** Creating a course, optionally filled in from TUMOnline's course list. */

import {
  CreateCourseRequestSchema,
  CreateCourseResponseSchema,
  SearchTumOnlineCoursesResponseSchema,
} from "@/gen/server/apiv2_pb";
import { apiGetMessage, apiPostMessage } from "./api";
import type { TeachingTerm } from "./semesters";

export interface TumOnlineCourse {
  tumOnlineId: string;
  name: string;
  year: number;
  term: TeachingTerm;
}

export interface NewCourse {
  name: string;
  slug: string;
  year: number;
  term: TeachingTerm;
  /** Set when the course was picked from TUMOnline; empty otherwise. */
  tumOnlineId: string;
  /** "de", "en", or "" for unspecified. */
  language: string;
}

export async function searchTumOnlineCourses(q: string): Promise<TumOnlineCourse[]> {
  const res = await apiGetMessage(
    SearchTumOnlineCoursesResponseSchema,
    `/tumonline-courses?q=${encodeURIComponent(q)}`,
  );
  return res.courses.map((c) => ({
    tumOnlineId: c.tumOnlineId,
    name: c.name,
    year: c.year,
    term: c.term === "S" ? "S" : "W",
  }));
}

/** Answers the new course's ID. */
export async function createCourse(course: NewCourse): Promise<number> {
  const res = await apiPostMessage(CreateCourseRequestSchema, CreateCourseResponseSchema, "/courses", {
    $typeName: "protobuf.CreateCourseRequest",
    ...course,
  });
  return res.courseId;
}

/** What the old form let into a slug; anything else is dropped as it is typed. */
export function sanitizeSlug(slug: string): string {
  return slug.replace(/[^A-Za-z0-9\-_]/g, "");
}

