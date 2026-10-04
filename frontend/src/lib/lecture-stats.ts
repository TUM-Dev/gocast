/** One lecture's usage statistics, for its course's lecturers. */

import { timestampDate } from "@bufbuild/protobuf/wkt";

import { LectureStatsResponseSchema } from "@/gen/server/apiv2_pb";
import { apiGetMessage } from "./api";
import { chartColors, points, type ChartPoint, type StatsChart, type StatsCounter } from "./usage-stats";

export interface LectureStats {
  courseName: string;
  lectureName: string;
  start: Date | null;
  end: Date | null;
  /** Enrolled in the course; a lecture has no enrolment of its own. */
  numStudents: number;
  vodViews: number;
  /** The most viewers the lecture had at once while it was live. */
  maxLiveViews: number;
  liveViewers: ChartPoint[];
  weekdays: ChartPoint[];
  allDays: ChartPoint[];
  /** The lecture can predate the viewing data, so it undercounts. */
  partialHistory: boolean;
}

export async function fetchLectureStats(courseId: number, streamId: number): Promise<LectureStats> {
  const res = await apiGetMessage(
    LectureStatsResponseSchema,
    `/courses/${courseId}/streams/${streamId}/stats`,
  );

  return {
    courseName: res.courseName,
    lectureName: res.lectureName,
    start: res.start ? timestampDate(res.start) : null,
    end: res.end ? timestampDate(res.end) : null,
    // An int64 arrives as a bigint; enrolment counts are small enough for Number.
    numStudents: Number(res.numStudents),
    vodViews: res.vodViews,
    maxLiveViews: res.maxLiveViews,
    liveViewers: points(res.liveViewers),
    weekdays: points(res.weekdays),
    allDays: points(res.allDays),
    partialHistory: res.partialHistory,
  };
}

const pad = (n: number) => String(n).padStart(2, "0");

/**
 * "18.04.2024 10:15 - 11:45", as model.Stream.FriendlyTime wrote it -- but in the
 * viewer's time zone rather than the server's.
 */
export function lectureTime(start: Date | null, end: Date | null): string {
  if (!start) return "";
  const date = `${pad(start.getDate())}.${pad(start.getMonth() + 1)}.${start.getFullYear()}`;
  const from = `${pad(start.getHours())}:${pad(start.getMinutes())}`;
  if (!end) return `${date} ${from}`;
  return `${date} ${from} - ${pad(end.getHours())}:${pad(end.getMinutes())}`;
}

/** The old page's counters, in its order. */
export function lectureCounters(stats: LectureStats): StatsCounter[] {
  return [
    { label: "Lecture Time", value: lectureTime(stats.start, stats.end) },
    { label: "Enrolled Students", value: stats.numStudents },
    { label: "Vod Views", value: stats.vodViews },
    { label: "Max Live Views", value: stats.maxLiveViews },
  ];
}

/** The old page's three charts. All three were v1's default blue bars but the last. */
export function lectureCharts(stats: LectureStats): StatsChart[] {
  const { blue, live } = chartColors;
  return [
    {
      title: "Student Live activity during lecture",
      type: "bar",
      label: "View Count",
      borderColor: blue,
      backgroundColor: blue,
      points: stats.liveViewers,
    },
    {
      title: "VoD activity per day of week",
      type: "bar",
      label: "Sum(viewers)",
      borderColor: blue,
      backgroundColor: blue,
      points: stats.weekdays,
    },
    {
      title: "VoD activity per day",
      type: "bar",
      label: "views",
      borderColor: blue,
      backgroundColor: live,
      points: stats.allDays,
    },
  ];
}
