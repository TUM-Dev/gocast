/**
 * What the server and course statistics pages both show: quick counters plus the
 * series behind each chart. getServerStats and getCourseStats answer with the plain
 * series, and UsageStatsPanel builds the Chart.js configs, so the wire format does
 * not carry a charting library's own shapes (the old pages were handed ready-made
 * configs by api/statistics.go's chartJs struct).
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
