/**
 * What the statistics pages show: quick counters plus the series behind each chart.
 * The RPCs answer with plain series and StatsPanel builds the Chart.js configs, so
 * the wire format does not carry a charting library's own shapes (the old pages were
 * handed ready-made configs by api/statistics.go's chartJs struct).
 */

import type { ServerStatsSeries } from "@/gen/server/apiv2_pb";

export interface ChartPoint {
  x: string;
  y: number;
}

export interface UsageStats {
  numStudents: number;
  vodViews: number;
  liveViews: number;
  /** Streams recorded or currently live. */
  numLectures: number;
  activityLive: ChartPoint[];
  activityVod: ChartPoint[];
  hourly: ChartPoint[];
  weekdays: ChartPoint[];
  allDays: ChartPoint[];
}

export function points(series: ServerStatsSeries | undefined): ChartPoint[] {
  return (series?.points ?? []).map((point) => ({ x: point.x, y: point.y }));
}

/** The fields both responses share, under the names both use. */
interface UsageStatsMessage {
  numStudents: bigint;
  vodViews: number;
  liveViews: number;
  numLectures: number;
  activityLive?: ServerStatsSeries;
  activityVod?: ServerStatsSeries;
  hourly?: ServerStatsSeries;
  weekdays?: ServerStatsSeries;
  allDays?: ServerStatsSeries;
}

export function usageStats(res: UsageStatsMessage): UsageStats {
  return {
    // An int64 arrives as a bigint; enrolment counts are small enough for Number.
    numStudents: Number(res.numStudents),
    vodViews: res.vodViews,
    liveViews: res.liveViews,
    numLectures: res.numLectures,
    activityLive: points(res.activityLive),
    activityVod: points(res.activityVod),
    hourly: points(res.hourly),
    weekdays: points(res.weekdays),
    allDays: points(res.allDays),
  };
}

/** One row of a statistics page's quick-stats table. */
export interface StatsCounter {
  label: string;
  value: string | number;
}

/** One chart on a statistics page. */
export interface StatsChart {
  title: string;
  type: "line" | "bar";
  /** The dataset's legend entry. */
  label: string;
  borderColor: string;
  backgroundColor: string;
  points: ChartPoint[];
}

/**
 * The colors v1's chartJs datasets used. A dataset there started from
 * newChartJsDataset's blue and some charts then overrode it.
 */
export const chartColors = {
  blue: "#427dbd",
  live: "#d12a5c",
  vod: "#2a7dd1",
} as const;

/** The server and course pages' counters, in the order the old pages listed them. */
export function usageCounters(stats: UsageStats): StatsCounter[] {
  return [
    { label: "Enrolled Students", value: stats.numStudents },
    { label: "Lectures", value: stats.numLectures },
    { label: "Vod Views", value: stats.vodViews },
    { label: "Live Views", value: stats.liveViews },
  ];
}

/** The server and course pages' five charts, in the order the old pages drew them. */
export function usageCharts(stats: UsageStats): StatsChart[] {
  const { blue, live, vod } = chartColors;
  return [
    {
      title: "Student Live activity per week",
      type: "line",
      label: "Live",
      borderColor: live,
      backgroundColor: "",
      points: stats.activityLive,
    },
    {
      title: "Student VoD activity per week",
      type: "line",
      label: "VoD",
      borderColor: vod,
      backgroundColor: "",
      points: stats.activityVod,
    },
    {
      title: "VoD activity throughout the day",
      type: "bar",
      label: "Sum(viewers)",
      borderColor: blue,
      backgroundColor: blue,
      points: stats.hourly,
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
      borderColor: live,
      backgroundColor: live,
      points: stats.allDays,
    },
  ];
}
