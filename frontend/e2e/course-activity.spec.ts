import { expect, test, type APIRequestContext } from "@playwright/test";

import { apiAs, bearerToken, login } from "./helpers";
import { users } from "./seed";

/**
 * A course's own audit log: the entries about it and its lectures, on the settings
 * page and behind GET /courses/{id}/audits, readable by its administrators alone.
 *
 * The log is built by writing to it, so everything here happens on a course created
 * for the purpose in the test semester 1234 and deleted afterwards: the seeded
 * courses' logs are read by nothing else, but their settings are (see
 * course-admin-api.spec.ts). The server-wide log is audits.spec.ts's, which runs
 * before anything here has written to it -- which is why this file must not be named
 * so that the `audits` project's testMatch (/audits\.spec\.ts/) picks it up.
 */

interface Api {
  context: APIRequestContext;
  headers: Record<string, string>;
}

async function apiFor(playwright: Parameters<typeof apiAs>[0], user?: (typeof users)[keyof typeof users]): Promise<Api> {
  const context = await apiAs(playwright, user);
  const token = user ? await bearerToken(context) : null;
  return { context, headers: token ? { Authorization: `Bearer ${token}` } : {} };
}

test.describe("a course's audit log", () => {
  test("lists what was done to the course, to its administrators only", async ({ playwright, page }) => {
    const prof2 = await apiFor(playwright, users.prof2);
    const prof1 = await apiFor(playwright, users.prof1);
    const studi1 = await apiFor(playwright, users.studi1);
    const admin = await apiFor(playwright, users.admin);
    const anonymous = await apiFor(playwright);

    const create = await prof2.context.post("/api/v2/courses", {
      headers: prof2.headers,
      data: { name: "E2E Audit Kurs", slug: "e2e-audit", year: 1234, term: "S", language: "" },
    });
    expect(create.status()).toBe(200);
    const id = (await create.json()).courseId;

    try {
      const edit = await prof2.context.patch(`/api/v2/courses/${id}/settings`, {
        headers: prof2.headers,
        data: { downloadsEnabled: true },
      });
      expect(edit.status()).toBe(200);

      // Newest first: the edit, then the creation, each by the lecturer who did it.
      const listed = await prof2.context.get(`/api/v2/courses/${id}/audits`, { headers: prof2.headers });
      expect(listed.status()).toBe(200);
      const body = await listed.json();
      expect(body.total).toBe(2);
      expect(body.audits.map((a: { type: string }) => a.type)).toEqual(["Course Edited", "Course Created"]);
      expect(body.audits[0].userId).toBe(3);

      // The kind filter narrows both the page and the total; an unknown kind is refused.
      const created = await (
        await prof2.context.get(`/api/v2/courses/${id}/audits?type=Course%20Created`, { headers: prof2.headers })
      ).json();
      expect(created.total).toBe(1);
      expect(created.audits[0].type).toBe("Course Created");
      expect(
        (await prof2.context.get(`/api/v2/courses/${id}/audits?type=Nope`, { headers: prof2.headers })).status(),
      ).toBe(400);

      // Other courses' administrators, students and strangers get what any other
      // course route gives them; a server administrator may read it.
      expect((await prof1.context.get(`/api/v2/courses/${id}/audits`, { headers: prof1.headers })).status()).toBe(404);
      expect((await studi1.context.get(`/api/v2/courses/${id}/audits`, { headers: studi1.headers })).status()).toBe(404);
      expect((await anonymous.context.get(`/api/v2/courses/${id}/audits`)).status()).toBe(401);
      expect((await admin.context.get(`/api/v2/courses/${id}/audits`, { headers: admin.headers })).status()).toBe(200);

      // Nothing of it leaks into another course's log.
      const other = await (
        await prof1.context.get("/api/v2/courses/1/audits?limit=100", { headers: prof1.headers })
      ).json();
      expect(other.audits.map((a: { message: string }) => a.message).join("\n")).not.toContain("e2e-audit");

      // And the settings page shows it.
      await login(page, users.prof2, `/admin/courses/${id}/settings`);
      const activity = page.getByRole("region", { name: "Activity" });
      // The entries, not the dropdown's options of the same names.
      const entries = activity.getByRole("list");
      await expect(entries.getByText("Course Edited")).toBeVisible();
      await expect(entries.getByText("Course Created")).toBeVisible();
      await expect(activity.getByText("2 entries")).toBeVisible();

      await activity.getByLabel("Type").selectOption("Course Created");
      await expect(activity.getByText("1 entries")).toBeVisible();
      await expect(entries.getByText("Course Edited")).toHaveCount(0);
    } finally {
      expect((await prof2.context.delete(`/api/v2/courses/${id}`, { headers: prof2.headers })).status()).toBe(200);
    }
  });
});
