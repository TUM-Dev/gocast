import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { clearToken } from "./api";
import { fetchLectureStats, lectureCharts, lectureCounters, lectureTime } from "./lecture-stats";

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

describe("fetchLectureStats", () => {
  it("asks for the course's lecture and reads its counters, times and series", async () => {
    respondWith({
      courseName: "Einführung Brauereiwesen",
      lectureName: "Hopfen",
      start: "2024-04-18T08:15:00Z",
      end: "2024-04-18T09:45:00Z",
      numStudents: "3",
      vodViews: 30,
      maxLiveViews: 8,
      liveViewers: { label: "View Count", points: [{ x: "10:20", y: 8 }] },
    });

    const stats = await fetchLectureStats(1, 7);

    expect(String(fetchMock.mock.calls.at(-1)?.[0])).toBe("/api/v2/courses/1/streams/7/stats");
    expect(stats.lectureName).toBe("Hopfen");
    expect(stats.start?.toISOString()).toBe("2024-04-18T08:15:00.000Z");
    expect(stats.end?.toISOString()).toBe("2024-04-18T09:45:00.000Z");
    expect(stats.numStudents).toBe(3);
    expect(stats.maxLiveViews).toBe(8);
    expect(stats.liveViewers).toEqual([{ x: "10:20", y: 8 }]);
    // protojson leaves out empty series and false bools.
    expect(stats.allDays).toEqual([]);
    expect(stats.partialHistory).toBe(false);
  });
});

describe("lectureTime", () => {
  it("writes the day once and both times, as FriendlyTime did", () => {
    expect(lectureTime(new Date(2024, 3, 8, 9, 5), new Date(2024, 3, 8, 10, 35))).toBe(
      "08.04.2024 09:05 - 10:35",
    );
  });

  it("copes with a lecture missing its times", () => {
    expect(lectureTime(null, null)).toBe("");
    expect(lectureTime(new Date(2024, 3, 8, 9, 5), null)).toBe("08.04.2024 09:05");
  });
});

describe("the page's counters and charts", () => {
  const stats = {
    courseName: "",
    lectureName: "",
    start: null,
    end: null,
    numStudents: 3,
    vodViews: 30,
    maxLiveViews: 8,
    liveViewers: [],
    weekdays: [],
    allDays: [],
    partialHistory: false,
  };

  it("lists the old page's counters in its order", () => {
    expect(lectureCounters(stats).map((c) => c.label)).toEqual([
      "Lecture Time",
      "Enrolled Students",
      "Vod Views",
      "Max Live Views",
    ]);
  });

  it("draws the old page's three charts", () => {
    expect(lectureCharts(stats).map((c) => c.title)).toEqual([
      "Student Live activity during lecture",
      "VoD activity per day of week",
      "VoD activity per day",
    ]);
  });
});
