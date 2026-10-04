/** One course's usage statistics, for its lecturers. */

import { CourseStatsResponseSchema } from "@/gen/server/apiv2_pb";
import { apiGetMessage } from "./api";
import { type UsageStats, usageStats } from "./usage-stats";

export interface CourseStats extends UsageStats {
  courseName: string;
  /** The course ran partly before viewing data was collected, so it undercounts. */
  partialHistory: boolean;
}

export async function fetchCourseStats(courseId: number): Promise<CourseStats> {
  const res = await apiGetMessage(CourseStatsResponseSchema, `/courses/${courseId}/stats`);

  return { ...usageStats(res), courseName: res.courseName, partialHistory: res.partialHistory };
}

/** A direct link for the browser to download, as serverStatsExportLink explains. */
export function courseStatsExportLink(courseId: number, format: "json" | "csv"): string {
  return `/api/v2/courses/${courseId}/stats/export?format=${format}`;
}
