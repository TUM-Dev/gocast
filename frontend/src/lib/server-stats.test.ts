import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { clearToken } from "./api";
import { fetchServerStats, serverStatsExportLink } from "./server-stats";

let fetchMock: ReturnType<typeof vi.fn>;

/** Answers the token request, then the given body for everything else. */
function respondWith(body: unknown, status = 200): void {
  fetchMock.mockImplementation((url: string) => {
    if (String(url).endsWith("/auth/token")) {
      return Promise.resolve(
        new Response(JSON.stringify({ access_token: "t", expires_in: 900 }), { status: 200 }),
      );
    }
    return Promise.resolve(new Response(JSON.stringify(body), { status }));
  });
}

beforeEach(() => {
  clearToken();
  fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("fetchServerStats", () => {
  it("reads the quick counters and every chart's series off the wire", async () => {
    respondWith({
      numStudents: "42",
      vodViews: 7,
      liveViews: 3,
      numLectures: 12,
      activityLive: { label: "Live", points: [{ x: "2024 01", y: 5 }] },
      activityVod: { label: "VoD", points: [{ x: "2024 01", y: 9 }] },
      hourly: { label: "Sum(viewers)", points: [{ x: "14", y: 2 }] },
      weekdays: { label: "Sum(viewers)", points: [{ x: "Monday", y: 4 }] },
      allDays: { label: "views", points: [{ x: "18.04.2022", y: 6 }] },
    });

    const stats = await fetchServerStats();

    expect(stats.numStudents).toBe(42);
    expect(stats.vodViews).toBe(7);
    expect(stats.liveViews).toBe(3);
    expect(stats.numLectures).toBe(12);
    expect(stats.activityLive).toEqual([{ x: "2024 01", y: 5 }]);
    expect(stats.activityVod).toEqual([{ x: "2024 01", y: 9 }]);
    expect(stats.hourly).toEqual([{ x: "14", y: 2 }]);
    expect(stats.weekdays).toEqual([{ x: "Monday", y: 4 }]);
    expect(stats.allDays).toEqual([{ x: "18.04.2022", y: 6 }]);
  });

  it("survives a series with no data points", async () => {
    // protojson omits an empty repeated field, and a fresh deployment has no stats yet.
    respondWith({ numStudents: "0", vodViews: 0, liveViews: 0 });

    const stats = await fetchServerStats();

    expect(stats.activityLive).toEqual([]);
    expect(stats.hourly).toEqual([]);
  });
});

describe("serverStatsExportLink", () => {
  it("builds a link per format", () => {
    expect(serverStatsExportLink("json")).toBe("/api/v2/admin/server-stats/export?format=json");
    expect(serverStatsExportLink("csv")).toBe("/api/v2/admin/server-stats/export?format=csv");
  });
});
