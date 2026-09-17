import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError, clearToken } from "./api";
import { importCourseImportCourses, searchCourseImportSchedule } from "./course-import";

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

describe("searchCourseImportSchedule", () => {
  it("reads a found course's fields off the wire", async () => {
    respondWith({
      courses: [
        {
          title: "Grundlagen der Datenbanken",
          slug: "GDB",
          courseId: 42,
          language: "de",
          import: true,
          events: [
            {
              start: "2025-10-13T08:00:00Z",
              end: "2025-10-13T10:00:00Z",
              roomName: "MI HS1",
              comment: "",
              eventId: "1",
              import: true,
            },
          ],
          contacts: [
            {
              firstName: "Ada",
              lastName: "Lovelace",
              email: "ada@example.com",
              role: "Veranstaltungsleiter",
              mainContact: true,
            },
          ],
        },
      ],
    });

    const [course] = await searchCourseImportSchedule(
      new Date("2025-10-01"),
      new Date("2025-10-31"),
      53598,
    );

    expect(course.title).toBe("Grundlagen der Datenbanken");
    expect(course.courseId).toBe(42);
    expect(course.language).toBe("de");
    expect(course.events).toHaveLength(1);
    expect(course.events[0].roomName).toBe("MI HS1");
    expect(course.contacts).toHaveLength(1);
    expect(course.contacts[0].mainContact).toBe(true);
  });

  it("sends the department and date range as the request body", async () => {
    respondWith({ courses: [] });

    await searchCourseImportSchedule(
      new Date("2025-10-01T00:00:00Z"),
      new Date("2025-10-31T00:00:00Z"),
      53598,
    );

    const call = fetchMock.mock.calls.find((c: unknown[]) => !String(c[0]).endsWith("/auth/token"));
    expect(call?.[0]).toBe("/api/v2/admin/course-import/search");
    const body = JSON.parse((call?.[1] as RequestInit).body as string);
    expect(body.departmentId).toBe(53598);
    expect(body.from).toBe("2025-10-01T00:00:00Z");
    expect(body.to).toBe("2025-10-31T00:00:00Z");
  });

  it("propagates an upstream failure instead of returning an empty list", async () => {
    // The bug this replaces: the v1 handler logged a failed TUMonline client and
    // returned with no response at all, leaving the caller hanging. A rejected
    // promise here is the point of the fix, not incidental.
    respondWith({ message: "fetching the schedule from TUMonline: dial tcp: timeout" }, 502);

    await expect(
      searchCourseImportSchedule(new Date(), new Date(), 53598),
    ).rejects.toBeInstanceOf(ApiError);
  });
});

describe("importCourseImportCourses", () => {
  it("reports each course's own outcome", async () => {
    respondWith({
      results: [
        { title: "Broken", success: false, error: "creating the course: duplicate slug" },
        { title: "Fine", success: true, error: "" },
      ],
    });

    const results = await importCourseImportCourses(2025, "W", false, []);

    expect(results).toEqual([
      { title: "Broken", success: false, error: "creating the course: duplicate slug" },
      { title: "Fine", success: true, error: "" },
    ]);
  });

  it("sends the year, term, opt-in flag and courses", async () => {
    respondWith({ results: [] });

    await importCourseImportCourses(2025, "W", true, [
      {
        title: "GDB",
        slug: "gdb",
        courseId: 42,
        language: "de",
        import: true,
        events: [],
        contacts: [],
      },
    ]);

    const call = fetchMock.mock.calls.find((c: unknown[]) => !String(c[0]).endsWith("/auth/token"));
    const body = JSON.parse((call?.[1] as RequestInit).body as string);
    expect(body.year).toBe(2025);
    expect(body.term).toBe("W");
    expect(body.optIn).toBe(true);
    expect(body.courses).toHaveLength(1);
    expect(body.courses[0].slug).toBe("gdb");
  });
});
