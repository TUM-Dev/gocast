/**
 * Server-wide usage statistics, for the administration page.
 *
 * The old page asked the server to hand back a ready-to-use Chart.js config per
 * chart (api/statistics.go's chartJs struct); getServerStats instead answers with the
 * plain series and this module builds the Chart.js configs, so the wire format does
 * not carry a charting library's own shapes.
 */

import { ServerStatsResponseSchema, type ServerStatsSeries } from "@/gen/server/apiv2_pb";
import { apiGetMessage } from "./api";

export interface ChartPoint {
  x: string;
  y: number;
}

export interface ServerStats {
  numStudents: number;
  vodViews: number;
  liveViews: number;
  /** Streams recorded or currently live, across every course. */
  numLectures: number;
  activityLive: ChartPoint[];
  activityVod: ChartPoint[];
  hourly: ChartPoint[];
  weekdays: ChartPoint[];
  allDays: ChartPoint[];
}

function points(series: ServerStatsSeries | undefined): ChartPoint[] {
  return (series?.points ?? []).map((point) => ({ x: point.x, y: point.y }));
}

/** The quick counters and every chart's series, in one call. */
export async function fetchServerStats(): Promise<ServerStats> {
  const res = await apiGetMessage(ServerStatsResponseSchema, "/admin/server-stats");

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

/**
 * A direct link rather than an API call: the browser downloads the response, reached
 * with the session cookie the browser already has (see SettingsView's personal-data
 * export, which relies on the same cookie fallback in the v2 auth interceptor).
 */
export function serverStatsExportLink(format: "json" | "csv"): string {
  return `/api/v2/admin/server-stats/export?format=${format}`;
}
