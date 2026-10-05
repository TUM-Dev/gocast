import { expect, test, type APIRequestContext } from "@playwright/test";

import { apiAs, bearerToken } from "./helpers";
import { users, type SeedUser } from "./seed";

/**
 * The course administration API, ahead of the edit-course page's move to the SPA.
 *
 * Course 1 (brauereiwesen) is administered by prof1 and prof2, course 2 (games101)
 * by prof2 alone and course 3 (godev) by prof1 alone. Other files run alongside this
 * one and read all three, so a write here has to land where none of them looks, not
 * merely be put back: the settings test flips godev, which no other file reads the
 * settings of, and granting an admin happens on a course created here. Copying and
 * deleting likewise only ever touch a course created here, in the test semester 1234
 * that the public listings hide.
 */

interface Caller {
  context: APIRequestContext;
  headers: Record<string, string>;
}

async function callerAs(playwright: Parameters<typeof apiAs>[0], user?: SeedUser): Promise<Caller> {
  const context = await apiAs(playwright, user);
  const token = user ? await bearerToken(context) : null;
  return { context, headers: token ? { Authorization: `Bearer ${token}` } : {} };
}

test.describe("getCourseAdmin", () => {
  test("answers an administrator of the course", async ({ playwright }) => {
    const { context, headers } = await callerAs(playwright, users.prof1);

    const response = await context.get("/api/v2/courses/1/admin", { headers });
    expect(response.status()).toBe(200);
    const body = await response.json();
    expect(body.id).toBe(1);
    expect(body.slug).toBe("brauereiwesen");
    expect(body.year).toBe(2022);
    expect(body.term).toBe("S");
    expect(body.visibility).toBe("public");
  });

  test("answers a lecturer of other courses as if the course did not exist", async ({ playwright }) => {
    const { context, headers } = await callerAs(playwright, users.prof1);

    expect((await context.get("/api/v2/courses/2/admin", { headers })).status()).toBe(404);
  });

  test("answers a student of the course the same way", async ({ playwright }) => {
    const { context, headers } = await callerAs(playwright, users.studi1);

    expect((await context.get("/api/v2/courses/1/admin", { headers })).status()).toBe(404);
  });

  test("refuses an anonymous caller", async ({ playwright }) => {
    const { context } = await callerAs(playwright);

    expect((await context.get("/api/v2/courses/1/admin")).status()).toBe(401);
  });
});

test.describe("updateCourseSettings", () => {
  test("changes only what is sent, and changes it back", async ({ playwright }) => {
    // Course 3 rather than course 1: course-settings.spec.ts flips course 1's downloads
    // too, and compares the whole settings object before and after.
    const { context, headers } = await callerAs(playwright, users.prof1);
    const before = await (await context.get("/api/v2/courses/3/admin", { headers })).json();
    // The JSON leaves false out.
    const downloads = before.downloadsEnabled ?? false;

    try {
      const flipped = await context.patch("/api/v2/courses/3/settings", {
        headers,
        data: { downloadsEnabled: !downloads },
      });
      expect(flipped.status()).toBe(200);
      const body = await flipped.json();
      expect(body.downloadsEnabled ?? false).toBe(!downloads);
      // Left alone because it was not sent.
      expect(body.visibility).toBe(before.visibility);
      expect(body.vodEnabled ?? false).toBe(before.vodEnabled ?? false);

      const reread = await (await context.get("/api/v2/courses/3/admin", { headers })).json();
      expect(reread.downloadsEnabled ?? false).toBe(!downloads);
    } finally {
      const restored = await context.patch("/api/v2/courses/3/settings", {
        headers,
        data: { downloadsEnabled: downloads },
      });
      expect(restored.status()).toBe(200);
    }

    const after = await (await context.get("/api/v2/courses/3/admin", { headers })).json();
    expect(after).toEqual(before);
  });

  test("rejects a visibility that is not one of the four", async ({ playwright }) => {
    const { context, headers } = await callerAs(playwright, users.prof1);

    const response = await context.patch("/api/v2/courses/1/settings", {
      headers,
      data: { visibility: "publicly" },
    });
    expect(response.status()).toBe(400);
  });
});

test.describe("course admins", () => {
  test("lists both lecturers of course 1", async ({ playwright }) => {
    const { context, headers } = await callerAs(playwright, users.prof1);

    const response = await context.get("/api/v2/courses/1/admins", { headers });
    expect(response.status()).toBe(200);
    const names = (await response.json()).admins.map((a: { name: string }) => a.name).sort();
    expect(names).toEqual([users.prof1.name, users.prof2.name].sort());
  });

  test("adds an admin who can then reach the course, and removes them again", async ({ playwright }) => {
    // On a course of its own, not games101: making prof1 an admin of a seeded course,
    // however briefly, is visible to every file that asserts what prof1 may not reach,
    // and those run alongside this one.
    const prof2 = await callerAs(playwright, users.prof2);
    const prof1 = await callerAs(playwright, users.prof1);
    const create = await prof2.context.post("/api/v2/courses", {
      headers: prof2.headers,
      data: { name: "E2E Admins Kurs", slug: "e2e-admins", year: 1234, term: "S", language: "" },
    });
    expect(create.status()).toBe(200);
    const id = (await create.json()).courseId;

    try {
      // Its creator is its only admin, and prof1 cannot reach it yet.
      const initial = await (await prof2.context.get(`/api/v2/courses/${id}/admins`, { headers: prof2.headers })).json();
      expect(initial.admins.map((a: { id: number }) => a.id)).toEqual([3]);
      expect((await prof1.context.get(`/api/v2/courses/${id}/admin`, { headers: prof1.headers })).status()).toBe(404);

      try {
        const added = await prof2.context.post(`/api/v2/courses/${id}/admins`, {
          headers: prof2.headers,
          data: { userId: 2 },
        });
        expect(added.status()).toBe(200);
        expect((await added.json()).name).toBe(users.prof1.name);

        const listed = await (await prof2.context.get(`/api/v2/courses/${id}/admins`, { headers: prof2.headers })).json();
        expect(listed.admins.map((a: { id: number }) => a.id).sort()).toEqual([2, 3]);

        // A fresh token: the policy reads the caller's administered courses.
        const fresh = await callerAs(playwright, users.prof1);
        expect((await fresh.context.get(`/api/v2/courses/${id}/admin`, { headers: fresh.headers })).status()).toBe(200);
      } finally {
        const removed = await prof2.context.delete(`/api/v2/courses/${id}/admins/2`, { headers: prof2.headers });
        expect(removed.status()).toBe(200);
      }

      const listed = await (await prof2.context.get(`/api/v2/courses/${id}/admins`, { headers: prof2.headers })).json();
      expect(listed.admins.map((a: { id: number }) => a.id)).toEqual([3]);
      expect((await prof1.context.get(`/api/v2/courses/${id}/admin`, { headers: prof1.headers })).status()).toBe(404);
    } finally {
      expect((await prof2.context.delete(`/api/v2/courses/${id}`, { headers: prof2.headers })).status()).toBe(200);
    }
  });

  test("refuses to remove a course's last admin", async ({ playwright }) => {
    const { context, headers } = await callerAs(playwright, users.prof1);

    // prof1 is course 3's only admin.
    expect((await context.delete("/api/v2/courses/3/admins/2", { headers })).status()).toBe(400);

    const listed = await (await context.get("/api/v2/courses/3/admins", { headers })).json();
    expect(listed.admins.map((a: { id: number }) => a.id)).toEqual([2]);
  });

  test("answers 404 for removing someone who is not an admin", async ({ playwright }) => {
    const { context, headers } = await callerAs(playwright, users.prof1);

    expect((await context.delete("/api/v2/courses/1/admins/4", { headers })).status()).toBe(404);
  });

  test("refuses a lecturer of other courses", async ({ playwright }) => {
    const { context, headers } = await callerAs(playwright, users.prof1);

    expect((await context.post("/api/v2/courses/2/admins", { headers, data: { userId: 2 } })).status()).toBe(404);
  });

  test("searches users for a course's lecturers, who lack users.manage", async ({ playwright }) => {
    const { context, headers } = await callerAs(playwright, users.prof1);

    const response = await context.get("/api/v2/courses/1/admins/search?q=studi", { headers });
    expect(response.status()).toBe(200);
    const found = (await response.json()).users;
    expect(found.map((u: { login: string }) => u.login).sort()).toEqual(["studi1", "studi2", "studi3"]);
    expect(found.every((u: { role: number }) => u.role === 4)).toBe(true);

    expect((await context.get("/api/v2/courses/1/admins/search?q=st", { headers })).status()).toBe(400);
    expect((await context.get("/api/v2/courses/2/admins/search?q=studi", { headers })).status()).toBe(404);
  });
});

test.describe("listAdministeredCourses", () => {
  test("lists the courses prof1 administers, in every semester", async ({ playwright }) => {
    const { context, headers } = await callerAs(playwright, users.prof1);

    const response = await context.get("/api/v2/courses/administered", { headers });
    expect(response.status()).toBe(200);
    // Leaving out the test semester: create-course.spec.ts and course-settings.spec.ts
    // create throwaway courses there as prof1 while this runs, and they are listed for
    // as long as they exist.
    const slugs = (await response.json()).courses
      .filter((c: { year: number }) => c.year !== 1234)
      .map((c: { slug: string }) => c.slug);
    // bierkunde is administered through course_admins too, added by the e2e seed.
    expect(slugs.sort()).toEqual(["bierkunde", "brauereiwesen", "godev"]);
  });

  test("refuses a student", async ({ playwright }) => {
    const { context, headers } = await callerAs(playwright, users.studi1);

    expect((await context.get("/api/v2/courses/administered", { headers })).status()).toBe(403);
  });
});

test.describe("lecture halls and participants", () => {
  test("lists course 1's lecture-hall settings and invited participants", async ({ playwright }) => {
    const { context, headers } = await callerAs(playwright, users.prof1);

    const halls = await context.get("/api/v2/courses/1/lecture-halls", { headers });
    expect(halls.status()).toBe(200);
    expect(Array.isArray((await halls.json()).lectureHalls ?? [])).toBe(true);

    const participants = await context.get("/api/v2/courses/1/participants", { headers });
    expect(participants.status()).toBe(200);
  });
});

test.describe("a throwaway course", () => {
  test("is copied into another semester and both are deleted", async ({ playwright }) => {
    const { context, headers } = await callerAs(playwright, users.prof1);
    const created: number[] = [];

    try {
      const create = await context.post("/api/v2/courses", {
        headers,
        data: { name: "E2E Admin Kurs", slug: "e2e-admin", year: 1234, term: "S", language: "" },
      });
      expect(create.status()).toBe(200);
      const id = (await create.json()).courseId;
      created.push(id);

      // No lectures, so no halls: any hall named is refused, and nothing is a valid
      // replacement.
      const badHall = await context.put(`/api/v2/courses/${id}/lecture-halls`, {
        headers,
        data: { lectureHalls: [{ lectureHallId: 1 }] },
      });
      expect(badHall.status()).toBe(400);
      expect(
        (await context.put(`/api/v2/courses/${id}/lecture-halls`, { headers, data: { lectureHalls: [] } })).status(),
      ).toBe(200);

      // Validated before anyone is invited, so this creates no account.
      const noInvitees = await context.post(`/api/v2/courses/${id}/participants`, { headers, data: { invitees: [] } });
      expect(noInvitees.status()).toBe(400);

      const copy = await context.post(`/api/v2/courses/${id}/copy`, { headers, data: { year: 1234, term: "W" } });
      expect(copy.status()).toBe(200);
      const copyId = (await copy.json()).courseId;
      created.push(copyId);

      // Same semester and slug as the copy now has.
      const again = await context.post(`/api/v2/courses/${id}/copy`, { headers, data: { year: 1234, term: "W" } });
      expect(again.status()).toBe(409);

      // The copy took its administrators along.
      const copied = await context.get(`/api/v2/courses/${copyId}/admin`, { headers });
      expect(copied.status()).toBe(200);
      expect((await copied.json()).term).toBe("W");
    } finally {
      for (const id of created) {
        expect((await context.delete(`/api/v2/courses/${id}`, { headers })).status()).toBe(200);
      }
    }

    for (const id of created) {
      expect((await context.get(`/api/v2/courses/${id}/admin`, { headers })).status()).toBe(404);
    }
  });

  test("refuses deleting a course to a lecturer of other courses", async ({ playwright }) => {
    const { context, headers } = await callerAs(playwright, users.prof1);

    expect((await context.delete("/api/v2/courses/2", { headers })).status()).toBe(404);
  });
});
