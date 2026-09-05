import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError, clearToken } from "./api";
import {
  addCourseAdmin,
  copyCourse,
  CourseSourceMode,
  deleteCourse,
  fetchAdministeredCourses,
  fetchCourseAdmin,
  fetchCourseIntegrationGrants,
  fetchLectureHallSettings,
  groupBySemester,
  inviteCourseParticipants,
  parseInviteBatch,
  publicCoursePath,
  removeCourseAdmin,
  revokeCourseIntegrationGrant,
  searchUsersForCourse,
  updateCourseSettings,
  updateLectureHallSettings,
} from "./course-admin";

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

/** The last API call: its URL, method and parsed body. */
function lastCall(): { url: string; method: string; body: unknown } {
  const [url, init] = fetchMock.mock.calls.at(-1) as [string, RequestInit];
  return {
    url: String(url),
    method: init.method ?? "GET",
    body: init.body ? JSON.parse(String(init.body)) : undefined,
  };
}

beforeEach(() => {
  clearToken();
  fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("fetchCourseAdmin", () => {
  it("reads the course, with false flags left out by protojson", async () => {
    respondWith({
      id: 1,
      name: "Einführung Brauereiwesen",
      slug: "brauereiwesen",
      year: 2022,
      term: "S",
      visibility: "public",
      vodEnabled: true,
      chatEnabled: true,
      userId: 1,
    });

    const course = await fetchCourseAdmin(1);

    expect(lastCall().url).toBe("/api/v2/courses/1/admin");
    expect(course).toMatchObject({
      id: 1,
      slug: "brauereiwesen",
      term: "S",
      visibility: "public",
      vodEnabled: true,
      downloadsEnabled: false,
      chatEnabled: true,
      language: "",
      tumOnlineIdentifier: "",
    });
    expect(publicCoursePath(course)).toBe("/course/2022/S/brauereiwesen");
  });

  it("surfaces a 404 as an ApiError", async () => {
    respondWith({ code: 5, message: "no such course" }, 404);

    await expect(fetchCourseAdmin(2)).rejects.toMatchObject({ status: 404 });
    await expect(fetchCourseAdmin(2)).rejects.toBeInstanceOf(ApiError);
  });
});

describe("updateCourseSettings", () => {
  it("sends only the fields given, false included", async () => {
    respondWith({ id: 1, name: "X", slug: "x", year: 2022, term: "S", visibility: "hidden" });

    const updated = await updateCourseSettings(1, { downloadsEnabled: false, visibility: "hidden" });

    const call = lastCall();
    expect(call.url).toBe("/api/v2/courses/1/settings");
    expect(call.method).toBe("PATCH");
    expect(call.body).toEqual({ courseId: 1, downloadsEnabled: false, visibility: "hidden" });
    expect(updated.visibility).toBe("hidden");
  });
});

describe("copy and delete", () => {
  it("copies into the semester given and answers the new course", async () => {
    respondWith({ courseId: 9, numErrors: 1 });

    expect(await copyCourse(1, { year: 2026, term: "W" })).toEqual({ courseId: 9, numErrors: 1 });
    expect(lastCall()).toMatchObject({
      url: "/api/v2/courses/1/copy",
      method: "POST",
      body: { courseId: 1, year: 2026, term: "W" },
    });
  });

  it("deletes by id", async () => {
    respondWith({});

    await deleteCourse(4);
    expect(lastCall()).toMatchObject({ url: "/api/v2/courses/4", method: "DELETE" });
  });
});

describe("authorized applications", () => {
  it("lists applications and revokes the grant in its course", async () => {
    respondWith({ grants: [{ id: 7, name: "Course portal" }] });
    expect(await fetchCourseIntegrationGrants(1)).toEqual([{ id: 7, name: "Course portal" }]);
    expect(lastCall().url).toBe("/api/v2/courses/1/integrations");
    respondWith({});
    await revokeCourseIntegrationGrant(1, 7);
    expect(lastCall()).toMatchObject({ url: "/api/v2/courses/1/integrations/7", method: "DELETE" });
  });
});

describe("administrators", () => {
  it("adds by user id", async () => {
    respondWith({ id: 2, name: "Peter Prof", login: "prof1", role: 2 });

    expect(await addCourseAdmin(1, 2)).toEqual({ id: 2, name: "Peter Prof", login: "prof1", role: 2 });
    expect(lastCall()).toMatchObject({ url: "/api/v2/courses/1/admins", body: { courseId: 1, userId: 2 } });
  });

  it("passes the server's reason for refusing to remove the last admin", async () => {
    respondWith({ code: 3, message: "can not remove the course's last administrator" }, 400);

    await expect(removeCourseAdmin(3, 2)).rejects.toThrow("last administrator");
    expect(lastCall()).toMatchObject({ url: "/api/v2/courses/3/admins/2", method: "DELETE" });
  });

  it("escapes the search query", async () => {
    respondWith({ users: [{ id: 5, name: "Stephanie Studi", login: "studi1", role: 4 }] });

    const found = await searchUsersForCourse(1, "a&b c");
    expect(lastCall().url).toBe("/api/v2/courses/1/admins/search?q=a%26b%20c");
    expect(found).toHaveLength(1);
  });
});

describe("lecture hall settings", () => {
  it("reads the source mode enum by name", async () => {
    respondWith({
      lectureHalls: [
        {
          lectureHallId: 1,
          lectureHallName: "HS1",
          presets: [{ presetId: 1, name: "Tafel", isDefault: true }],
          sourceMode: "COURSE_SOURCE_MODE_CAMERA_ONLY",
          selectedPresetId: 1,
        },
      ],
    });

    const [hall] = await fetchLectureHallSettings(1);
    expect(hall.sourceMode).toBe(CourseSourceMode.CAMERA_ONLY);
    expect(hall.presets[0]).toEqual({ presetId: 1, name: "Tafel", image: "", isDefault: true });
  });

  it("replaces every hall's settings with a PUT", async () => {
    respondWith({ lectureHalls: [] });

    await updateLectureHallSettings(1, [
      { lectureHallId: 1, sourceMode: CourseSourceMode.PRESENTATION_ONLY, selectedPresetId: 3 },
    ]);
    const call = lastCall();
    expect(call.method).toBe("PUT");
    expect(call.url).toBe("/api/v2/courses/1/lecture-halls");
    expect(call.body).toEqual({
      courseId: 1,
      lectureHalls: [
        { lectureHallId: 1, sourceMode: "COURSE_SOURCE_MODE_PRESENTATION_ONLY", selectedPresetId: 3 },
      ],
    });
  });
});

describe("participants", () => {
  it("invites and reads each invitee's result", async () => {
    respondWith({ results: [{ email: "tim@lmu.de", accountCreated: true }, { email: "a@b.de", error: "nope" }] });

    const results = await inviteCourseParticipants(1, [
      { name: "Tim", email: "tim@lmu.de" },
      { name: "A", email: "a@b.de" },
    ]);
    expect(lastCall().body).toEqual({
      courseId: 1,
      invitees: [
        { name: "Tim", email: "tim@lmu.de" },
        { name: "A", email: "a@b.de" },
      ],
    });
    expect(results).toEqual([
      { email: "tim@lmu.de", accountCreated: true, error: "" },
      { email: "a@b.de", accountCreated: false, error: "nope" },
    ]);
  });
});

describe("parseInviteBatch", () => {
  it("reads name,email lines in the old form's order, skipping blank ones", () => {
    expect(parseInviteBatch("Tim, tim69@hotmail.com\n\n  Anja,anja@lmu.de \n")).toEqual({
      invitees: [
        { name: "Tim", email: "tim69@hotmail.com" },
        { name: "Anja", email: "anja@lmu.de" },
      ],
      badLines: [],
    });
  });

  it("reports the lines that are not name,email instead of dropping them", () => {
    const parsed = parseInviteBatch("Tim,tim@lmu.de\nanja@lmu.de\nA,b,c@d.de\n,x@y.de\nBob,bob");
    expect(parsed.invitees).toEqual([{ name: "Tim", email: "tim@lmu.de" }]);
    expect(parsed.badLines).toEqual([2, 3, 4, 5]);
  });
});

describe("the sidebar's course tree", () => {
  it("fetches the administered courses", async () => {
    respondWith({ courses: [{ id: 3, name: "Praktikum: Golang", slug: "godev", year: 2021, term: "W" }] });

    expect(await fetchAdministeredCourses()).toEqual([
      { id: 3, name: "Praktikum: Golang", slug: "godev", year: 2021, term: "W" },
    ]);
    expect(lastCall().url).toBe("/api/v2/courses/administered");
  });

  it("groups by semester, newest first, a winter after its year's summer", () => {
    const groups = groupBySemester([
      { id: 3, name: "Praktikum: Golang", slug: "godev", year: 2021, term: "W" },
      { id: 4, name: "Fortgeschrittene Bierkunde", slug: "bierkunde", year: 2022, term: "S" },
      { id: 6, name: "Neu", slug: "neu", year: 2022, term: "W" },
      { id: 1, name: "Einführung Brauereiwesen", slug: "brauereiwesen", year: 2022, term: "S" },
    ]);

    expect(groups.map((g) => g.label)).toEqual(["Winter 2022/23", "Summer 2022", "Winter 2021/22"]);
    expect(groups[1].courses.map((c) => c.id)).toEqual([1, 4]);
  });
});
