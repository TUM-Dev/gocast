import { afterEach, describe, expect, it, vi } from "vitest";

import { ApiError, clearToken } from "./api";
import type { Course } from "./courses";
import type { Semester } from "./semesters";
import {
  courseHitUrl,
  courseKey,
  coursesOfSemesters,
  formatTimestamp,
  kindsFor,
  search,
  searchErrorMessage,
  searchPath,
  semesterKey,
  streamHitUrl,
  subtitleHitUrl,
} from "./search";

vi.mock("./courses", async (importOriginal) => ({
  ...(await importOriginal<typeof import("./courses")>()),
  fetchPublicCourses: vi.fn(),
  fetchUserCourses: vi.fn(),
}));

import { fetchPublicCourses, fetchUserCourses } from "./courses";

const s2022 = { year: 2022, term: "S" as const };
const w2021 = { year: 2021, term: "W" as const };
const course = (slug: string, semester: Semester = s2022) => ({ slug, name: slug, semester }) as unknown as Course;

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("keys and paths", () => {
  it("write semesters and courses the way the API filters read them", () => {
    expect(semesterKey(s2022)).toBe("2022S");
    expect(courseKey(course("brauereiwesen"))).toBe("brauereiwesen2022S");
    expect(searchPath("bier & malz", { semesters: [s2022, w2021], courses: [course("godev", w2021)] }, 5)).toBe(
      "/search?query=bier+%26+malz&limit=5&semesters=2022S&semesters=2021W&courses=godev2021W",
    );
  });

  it("know which kinds a filter can find, as the old page showed them", () => {
    expect(kindsFor({ semesters: [], courses: [] })).toEqual({ streams: false, subtitles: false });
    expect(kindsFor({ semesters: [s2022], courses: [] })).toEqual({ streams: true, subtitles: false });
    expect(kindsFor({ semesters: [s2022, w2021], courses: [] })).toEqual({ streams: false, subtitles: false });
    expect(kindsFor({ semesters: [s2022], courses: [course("x")] })).toEqual({ streams: true, subtitles: true });
  });

  it("link a hit to its page", () => {
    expect(courseHitUrl({ name: "", slug: "brauereiwesen", semester: s2022 })).toBe("/course/2022/S/brauereiwesen");
    expect(streamHitUrl({ id: 7, courseSlug: "brauereiwesen" } as never)).toBe("/w/brauereiwesen/7");
    expect(subtitleHitUrl({ streamId: 7, courseSlug: "brauereiwesen", timestamp: 65_900 } as never)).toBe(
      "/w/brauereiwesen/7?t=65",
    );
  });
});

describe("search", () => {
  it("reads the three kinds of hit back", async () => {
    clearToken();
    // No session: the token request answers 401 and the search goes out anonymously.
    vi.stubGlobal("fetch", async (input: RequestInfo | URL) => {
      if (String(input).includes("/auth/token")) return new Response("", { status: 401 });
      return new Response(
        JSON.stringify({
          courses: [{ name: "Brauereiwesen", slug: "brauereiwesen", year: 2022, term: "S" }],
          streams: [{ id: 7, name: "Hopfen", courseName: "Brauereiwesen", courseSlug: "brauereiwesen", year: 2022, term: "S" }],
          subtitles: [
            {
              streamId: 7,
              timestamp: "65900",
              textPrev: "a",
              text: "b",
              textNext: "c",
              streamName: "Hopfen",
              streamStart: "2022-04-11T10:00:00Z",
              courseName: "Brauereiwesen",
              courseSlug: "brauereiwesen",
              year: 2022,
              term: "S",
            },
          ],
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    });

    const hits = await search("hopfen", { semesters: [s2022], courses: [] });

    expect(hits.courses[0]).toEqual({ name: "Brauereiwesen", slug: "brauereiwesen", semester: s2022 });
    expect(hits.streams[0].id).toBe(7);
    expect(hits.subtitles[0].timestamp).toBe(65_900);
    expect(hits.subtitles[0].streamStart?.toISOString()).toBe("2022-04-11T10:00:00.000Z");
  });
});

describe("coursesOfSemesters", () => {
  it("joins public and own courses of each semester, each once, own ones only when signed in", async () => {
    vi.mocked(fetchPublicCourses).mockImplementation(async (s) => [course("public", s as never)]);
    vi.mocked(fetchUserCourses).mockImplementation(async (s) => [course("public", s as never), course("mine", s as never)]);

    const signedIn = await coursesOfSemesters([s2022, w2021], true);
    expect(signedIn.map(courseKey)).toEqual(["public2022S", "mine2022S", "public2021W", "mine2021W"]);

    const anonymous = await coursesOfSemesters([s2022], false);
    expect(anonymous.map(courseKey)).toEqual(["public2022S"]);
  });
});

describe("formatTimestamp", () => {
  it("shows minutes and seconds, and hours when there are any", () => {
    expect(formatTimestamp(65_900)).toBe("1:05");
    expect(formatTimestamp(3_723_000)).toBe("1:02:03");
    expect(formatTimestamp(0)).toBe("0:00");
  });
});

describe("searchErrorMessage", () => {
  it("names an unavailable search, a course out of reach and repeats a validation message", () => {
    expect(searchErrorMessage(new ApiError(503, "search is not available"))).toContain("not available right now");
    expect(searchErrorMessage(new ApiError(404, "no such course"))).toContain("not available to you");
    expect(searchErrorMessage(new ApiError(400, "query is required"))).toBe("query is required");
    expect(searchErrorMessage(new Error("boom"))).toBe("Something went wrong. Please try again.");
  });
});
