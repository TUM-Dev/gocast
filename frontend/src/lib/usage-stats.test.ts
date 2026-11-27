import { create } from "@bufbuild/protobuf";
import { describe, expect, it } from "vitest";

import { ServerStatsSeriesSchema, type ServerStatsSeries } from "@/gen/server/apiv2_pb";
import {
  chartColors,
  points,
  usageCharts,
  usageCounters,
  usageStats,
  type UsageStats,
} from "./usage-stats";

function series(label: string, data: [string, number][] = []): ServerStatsSeries {
  return create(ServerStatsSeriesSchema, {
    label,
    points: data.map(([x, y]) => ({ x, y })),
  });
}

const stats: UsageStats = {
  numStudents: 42,
  vodViews: 7,
  liveViews: 3,
  numLectures: 12,
  activityLive: [{ x: "2024 01", y: 5 }],
  activityVod: [{ x: "2024 01", y: 9 }],
  hourly: [{ x: "14", y: 2 }],
  weekdays: [{ x: "Monday", y: 4 }],
  allDays: [{ x: "18.04.2022", y: 6 }],
};

describe("points", () => {
  it("maps each wire point to its x and y", () => {
    expect(points(series("Live", [["2024 01", 5], ["2024 02", 8]]))).toEqual([
      { x: "2024 01", y: 5 },
      { x: "2024 02", y: 8 },
    ]);
  });

  it("treats a missing or empty series as no points", () => {
    expect(points(undefined)).toEqual([]);
    expect(points(series("VoD"))).toEqual([]);
  });
});

describe("usageStats", () => {
  it("reads the counters and every chart's series off the wire", () => {
    const result = usageStats({
      numStudents: 42n,
      vodViews: 7,
      liveViews: 3,
      numLectures: 12,
      activityLive: series("Live", [["2024 01", 5]]),
      activityVod: series("VoD", [["2024 01", 9]]),
      hourly: series("Sum(viewers)", [["14", 2]]),
      weekdays: series("Sum(viewers)", [["Monday", 4]]),
      allDays: series("views", [["18.04.2022", 6]]),
    });

    expect(result).toEqual(stats);
    expect(result.numStudents).toBe(42);
  });

  it("survives a response with no series and no enrolments", () => {
    // protojson omits an empty repeated field, and a fresh deployment has no stats yet.
    const result = usageStats({ numStudents: 0n, vodViews: 0, liveViews: 0, numLectures: 0 });

    expect(result).toEqual({
      numStudents: 0,
      vodViews: 0,
      liveViews: 0,
      numLectures: 0,
      activityLive: [],
      activityVod: [],
      hourly: [],
      weekdays: [],
      allDays: [],
    });
  });
});

describe("usageCounters", () => {
  it("lists the counters in the order the old pages used", () => {
    expect(usageCounters(stats)).toEqual([
      { label: "Enrolled Students", value: 42 },
      { label: "Lectures", value: 12 },
      { label: "Vod Views", value: 7 },
      { label: "Live Views", value: 3 },
    ]);
  });
});

describe("usageCharts", () => {
  it("builds the five charts with their titles, types, labels, colours and points", () => {
    expect(usageCharts(stats)).toEqual([
      {
        title: "Student Live activity per week",
        type: "line",
        label: "Live",
        borderColor: chartColors.live,
        backgroundColor: "",
        points: stats.activityLive,
      },
      {
        title: "Student VoD activity per week",
        type: "line",
        label: "VoD",
        borderColor: chartColors.vod,
        backgroundColor: "",
        points: stats.activityVod,
      },
      {
        title: "VoD activity throughout the day",
        type: "bar",
        label: "Sum(viewers)",
        borderColor: chartColors.blue,
        backgroundColor: chartColors.blue,
        points: stats.hourly,
      },
      {
        title: "VoD activity per day of week",
        type: "bar",
        label: "Sum(viewers)",
        borderColor: chartColors.blue,
        backgroundColor: chartColors.blue,
        points: stats.weekdays,
      },
      {
        title: "VoD activity per day",
        type: "bar",
        label: "views",
        borderColor: chartColors.live,
        backgroundColor: chartColors.live,
        points: stats.allDays,
      },
    ]);
  });
});
