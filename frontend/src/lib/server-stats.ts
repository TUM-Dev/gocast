/** Server-wide usage statistics, for the administration page. */

import { ServerStatsResponseSchema } from "@/gen/server/apiv2_pb";
import { apiGetMessage } from "./api";
import { type UsageStats, usageStats } from "./usage-stats";

export type { ChartPoint } from "./usage-stats";

/** numLectures counts across every course. */
export type ServerStats = UsageStats;

/** The quick counters and every chart's series, in one call. */
export async function fetchServerStats(): Promise<ServerStats> {
  return usageStats(await apiGetMessage(ServerStatsResponseSchema, "/admin/server-stats"));
}

/**
 * A direct link rather than an API call: the browser downloads the response, reached
 * with the session cookie the browser already has (see SettingsView's personal-data
 * export, which relies on the same cookie fallback in the v2 auth interceptor).
 */
export function serverStatsExportLink(format: "json" | "csv"): string {
  return `/api/v2/admin/server-stats/export?format=${format}`;
}
