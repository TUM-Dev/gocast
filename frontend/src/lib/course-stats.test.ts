import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { clearToken } from "./api";
import { courseStatsExportLink, fetchCourseStats } from "./course-stats";

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

describe("fetchCourseStats", () => {
  it("asks for the one course and reads its counters, series and flags", async () => {
    respondWith({
      courseName: "Einführung Brauereiwesen",
      numStudents: "3",
      vodViews: 30,
      liveViews: 8,
      numLectures: 5,
      allDays: { label: "views", points: [{ x: "18.04.2022", y: 6 }] },
      partialHistory: true,
    });

    const stats = await fetchCourseStats(1);

    expect(String(fetchMock.mock.calls.at(-1)?.[0])).toBe("/api/v2/courses/1/stats");
    expect(stats.courseName).toBe("Einführung Brauereiwesen");
    expect(stats.numStudents).toBe(3);
    expect(stats.numLectures).toBe(5);
    expect(stats.allDays).toEqual([{ x: "18.04.2022", y: 6 }]);
    expect(stats.hourly).toEqual([]);
    expect(stats.partialHistory).toBe(true);
  });

  it("reads an omitted flag as unset", async () => {
    // protojson leaves out a false bool.
    respondWith({ courseName: "Spieleentwicklung", numStudents: "0", vodViews: 0, liveViews: 0 });

    expect((await fetchCourseStats(2)).partialHistory).toBe(false);
  });
});

describe("courseStatsExportLink", () => {
  it("builds a link per course and format", () => {
    expect(courseStatsExportLink(3, "json")).toBe("/api/v2/courses/3/stats/export?format=json");
    expect(courseStatsExportLink(3, "csv")).toBe("/api/v2/courses/3/stats/export?format=csv");
  });
});
