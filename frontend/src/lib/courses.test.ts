import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { clearToken } from "./api";
import {
  calendarUrl,
  courseUrl,
  fetchPublicCourses,
  fetchUserCourses,
  isHidden,
  setCoursePinned,
  type Course,
} from "./courses";

let fetchMock: ReturnType<typeof vi.fn>;

function urls(): string[] {
  return fetchMock.mock.calls.map((call) => String(call[0]));
}

beforeEach(() => {
  clearToken();
  fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => vi.unstubAllGlobals());

/** A signed-in caller: the token request succeeds before the listing is fetched. */
function respondWith(body: unknown): void {
  fetchMock
    .mockResolvedValueOnce(new Response(JSON.stringify({ access_token: "t" }), { status: 200 }))
    .mockResolvedValueOnce(new Response(JSON.stringify(body), { status: 200 }));
}

const courseMessage = {
  id: 1,
  name: "Introduction to Informatics",
  slug: "eidi",
  semester: { year: 2026, teachingTerm: "W" },
  visibility: "public",
  pinned: true,
  isAdmin: false,
  downloadsEnabled: true,
};

describe("fetchPublicCourses", () => {
  it("reads a course and the caller's relationship to it", async () => {
    respondWith({ courses: [courseMessage] });

    const [course] = await fetchPublicCourses();

    expect(course).toMatchObject({
      id: 1,
      slug: "eidi",
      semester: { year: 2026, term: "W" },
      visibility: "public",
      pinned: true,
      isAdmin: false,
    });
  });

  /**
   * The server sends these absent when the course has none, rather than as a stream
   * with id 0 — which is what the old page had to test for on every use.
   */
  it("leaves a course with no lectures without a last recording or next lecture", async () => {
    respondWith({ courses: [courseMessage] });

    const [course] = await fetchPublicCourses();

    expect(course.lastRecording).toBeUndefined();
    expect(course.nextLecture).toBeUndefined();
    expect(course.streams).toEqual([]);
  });

  it("converts lecture timestamps to dates", async () => {
    respondWith({
      courses: [
        {
          ...courseMessage,
          lastRecording: {
            id: 9,
            name: "Lecture 1",
            start: "2026-08-20T10:00:00Z",
            end: "2026-08-20T12:00:00Z",
            duration: 7200,
            recording: true,
            isPubliclyVisible: true,
          },
        },
      ],
    });

    const [course] = await fetchPublicCourses();

    expect(course.lastRecording?.start).toEqual(new Date("2026-08-20T10:00:00Z"));
    expect(course.lastRecording?.duration).toBe(7200);
  });

  it("sorts by name, as every listing on the old page was ordered", async () => {
    respondWith({
      courses: [
        { ...courseMessage, id: 2, name: "Zoology", slug: "zoo" },
        { ...courseMessage, id: 3, name: "Algebra", slug: "alg" },
      ],
    });

    expect((await fetchPublicCourses()).map((c) => c.name)).toEqual(["Algebra", "Zoology"]);
  });

  it("asks for no semester in particular when none is given", async () => {
    respondWith({ courses: [] });

    await fetchPublicCourses();

    // No `?year=&term=`: the server answers with the current semester, which only it
    // knows. Sending empty parameters would ask for year 0.
    expect(urls()[1]).toBe("/api/v2/courses");
  });

  it("passes on the semester it was given", async () => {
    respondWith({ courses: [] });

    await fetchPublicCourses({ year: 2025, term: "S" });

    expect(urls()[1]).toBe("/api/v2/courses?year=2025&term=S");
  });

  /**
   * protobuf omits a field at its zero value, so a course the server did not classify
   * arrives with no visibility at all. Defaulting to "public" there would list a
   * course to everyone; the fallback is the most restricted of the four instead.
   */
  it("treats an unclassified course as the most restricted kind", async () => {
    respondWith({ courses: [{ ...courseMessage, visibility: undefined }] });

    const [course] = await fetchPublicCourses();

    expect(course.visibility).toBe("enrolled");
    expect(isHidden(course)).toBe(false);
  });
});

describe("fetchUserCourses", () => {
  it("goes to the enrolled listing, which needs a signed-in caller", async () => {
    respondWith({ courses: [] });

    await fetchUserCourses({ year: 2026, term: "W" });

    expect(urls()[1]).toBe("/api/v2/courses/enrolled?year=2026&term=W");
  });
});

describe("setCoursePinned", () => {
  it("posts the pin as the server expects it", async () => {
    respondWith({ message: "ok" });

    await setCoursePinned(42, true);

    expect(urls()[1]).toBe("/api/v2/courses/42/pin");
    const init = fetchMock.mock.calls[1][1] as RequestInit;
    expect(init.method).toBe("POST");
    expect(JSON.parse(String(init.body))).toEqual({ courseId: 42, pin: true });
  });
});

describe("urls", () => {
  const course = { slug: "eidi", semester: { year: 2026, term: "W" } } as Course;

  it("builds the client route for a course", () => {
    expect(courseUrl(course)).toBe("/course/2026/W/eidi");
  });

  it("points the calendar feed at the v1 endpoint that still serves it", () => {
    expect(calendarUrl(course)).toBe("/api/download_ics/2026/W/eidi/events.ics");
  });
});
