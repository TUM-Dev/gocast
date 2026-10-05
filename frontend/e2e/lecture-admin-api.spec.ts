import { type APIRequestContext, expect, test } from "@playwright/test";

import { apiAs } from "./helpers";
import { users } from "./seed";

/**
 * The lecture administration RPCs, API only: no page uses them yet.
 *
 * Einführung Brauereiwesen (course 1) is administered by prof1 and prof2,
 * Spieleentwicklung (course 2) by prof2 alone, and its "VL 1: Livestream" is stream 7.
 *
 * The one test that changes a lecture puts back exactly what it read, in a finally,
 * and changes "VL 3: Rückblick" (stream 4): a past lecture that was never recorded, so
 * no page lists it and no other spec looks for it by name while this runs alongside
 * them. Deletes and copies are only ever exercised as refusals, since there is no v2
 * way yet to recreate what they would remove.
 */

const renamed = "VL 3: Rückblick";

interface AdminLecture {
  id: number;
  courseId: number;
  name?: string;
  description?: string;
  start: string;
  end: string;
}

const course2Stream = 7;
const course2StreamName = "VL 1: Livestream";

async function lectures(context: APIRequestContext, courseId: number): Promise<AdminLecture[]> {
  const response = await context.get(`/api/v2/courses/${courseId}/lectures/admin`);
  expect(response.status()).toBe(200);
  return (await response.json()).lectures ?? [];
}

test.describe("listing a course's lectures for administration", () => {
  test("answers the course's lectures to its administrator", async ({ playwright }) => {
    const prof1 = await apiAs(playwright, users.prof1);
    const listed = await lectures(prof1, 1);

    expect(listed.map((l) => l.name)).toContain("VL 1: Was ist Bier?");
    expect(listed.every((l) => l.courseId === 1)).toBe(true);
    expect(listed.map((l) => l.id)).not.toContain(course2Stream);
  });

  test("answers 404 for a course the caller does not administer", async ({ playwright }) => {
    const prof1 = await apiAs(playwright, users.prof1);
    expect((await prof1.get("/api/v2/courses/2/lectures/admin")).status()).toBe(404);
  });

  test("answers 404 to a student", async ({ playwright }) => {
    const studi1 = await apiAs(playwright, users.studi1);
    expect((await studi1.get("/api/v2/courses/1/lectures/admin")).status()).toBe(404);
  });

  test("answers 401 to an anonymous caller", async ({ playwright }) => {
    const anonymous = await apiAs(playwright);
    expect((await anonymous.get("/api/v2/courses/1/lectures/admin")).status()).toBe(401);
  });
});

test.describe("updating a lecture", () => {
  test("renames and reschedules one of the course's lectures", async ({ playwright }) => {
    const prof1 = await apiAs(playwright, users.prof1);
    const original = (await lectures(prof1, 1)).find((l) => l.name === renamed);
    expect(original, `${renamed} is not listed`).toBeTruthy();
    const url = `/api/v2/courses/1/streams/${original!.id}`;

    const start = new Date(new Date(original!.start).getTime() + 60 * 60 * 1000).toISOString();
    const end = new Date(new Date(original!.end).getTime() + 60 * 60 * 1000).toISOString();

    try {
      const response = await prof1.patch(url, { data: { name: "VL 3: Umbenannt", start, end } });
      expect(response.status()).toBe(200);

      const updated = (await lectures(prof1, 1)).find((l) => l.id === original!.id);
      expect(updated?.name).toBe("VL 3: Umbenannt");
      expect(new Date(updated!.start).getTime()).toBe(new Date(start).getTime());
      expect(new Date(updated!.end).getTime()).toBe(new Date(end).getTime());
    } finally {
      const restored = await prof1.patch(url, {
        data: { name: original!.name, start: original!.start, end: original!.end },
      });
      expect(restored.status()).toBe(200);
    }

    const after = (await lectures(prof1, 1)).find((l) => l.id === original!.id);
    expect(after?.name).toBe(original!.name);
    expect(after?.start).toBe(original!.start);
    expect(after?.end).toBe(original!.end);
  });

  test("refuses an end before the start, and a start alone", async ({ playwright }) => {
    const prof1 = await apiAs(playwright, users.prof1);
    const lecture = (await lectures(prof1, 1)).find((l) => l.name === renamed)!;
    const url = `/api/v2/courses/1/streams/${lecture.id}`;

    expect((await prof1.patch(url, { data: { start: lecture.end, end: lecture.start } })).status()).toBe(400);
    expect((await prof1.patch(url, { data: { start: lecture.start } })).status()).toBe(400);
  });

  test("will not touch another course's lecture through one's own", async ({ playwright }) => {
    const prof1 = await apiAs(playwright, users.prof1);
    const response = await prof1.patch(`/api/v2/courses/1/streams/${course2Stream}`, {
      data: { name: "Hijacked", private: true },
    });
    expect(response.status()).toBe(404);

    const prof2 = await apiAs(playwright, users.prof2);
    const stream = (await lectures(prof2, 2)).find((l) => l.id === course2Stream);
    expect(stream?.name).toBe(course2StreamName);
  });
});

test.describe("the series, delete and copy RPCs", () => {
  test("refuse another course's lecture", async ({ playwright }) => {
    const prof1 = await apiAs(playwright, users.prof1);
    const base = `/api/v2/courses/1/streams/${course2Stream}`;

    expect((await prof1.patch(`${base}/series`, { data: { name: "Hijacked" } })).status()).toBe(404);
    const now = Date.now();
    expect(
      (
        await prof1.put(`${base}/series/time`, {
          data: { start: new Date(now).toISOString(), end: new Date(now + 36e5).toISOString() },
        })
      ).status(),
    ).toBe(404);
    expect((await prof1.delete(`${base}/series`)).status()).toBe(404);
    expect((await prof1.post(`${base}/copy`, { data: { targetCourseId: 1 } })).status()).toBe(404);

    const prof2 = await apiAs(playwright, users.prof2);
    expect((await lectures(prof2, 2)).map((l) => l.id)).toContain(course2Stream);
  });

  test("delete nothing when one of the lectures is another course's", async ({ playwright }) => {
    const prof1 = await apiAs(playwright, users.prof1);
    const own = (await lectures(prof1, 1)).find((l) => l.name === "VL 1: Was ist Bier?")!;

    const response = await prof1.post("/api/v2/courses/1/streams/delete", {
      data: { streamIds: [own.id, course2Stream] },
    });
    expect(response.status()).toBe(404);

    expect((await lectures(prof1, 1)).map((l) => l.id)).toContain(own.id);
  });

  test("refuse a series operation on a lecture in no series", async ({ playwright }) => {
    const prof1 = await apiAs(playwright, users.prof1);
    const own = (await lectures(prof1, 1)).find((l) => l.name === "VL 1: Was ist Bier?")!;
    const base = `/api/v2/courses/1/streams/${own.id}`;

    expect((await prof1.patch(`${base}/series`, { data: { name: "Series" } })).status()).toBe(400);
    expect((await prof1.delete(`${base}/series`)).status()).toBe(400);
    expect((await lectures(prof1, 1)).find((l) => l.id === own.id)?.name).toBe("VL 1: Was ist Bier?");
  });

  test("copy refuses a target course the caller does not administer", async ({ playwright }) => {
    // prof1 administers course 1 but not course 2.
    const prof1 = await apiAs(playwright, users.prof1);
    const own = (await lectures(prof1, 1)).find((l) => l.name === "VL 1: Was ist Bier?")!;

    const response = await prof1.post(`/api/v2/courses/1/streams/${own.id}/copy`, {
      data: { targetCourseId: 2, move: true },
    });
    expect(response.status()).toBe(404);

    expect((await lectures(prof1, 1)).map((l) => l.id)).toContain(own.id);
    const prof2 = await apiAs(playwright, users.prof2);
    expect((await lectures(prof2, 2)).map((l) => l.name)).not.toContain("VL 1: Was ist Bier?");
  });
});
