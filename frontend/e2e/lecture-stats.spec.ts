import { expect, test } from "@playwright/test";

import { apiAs, bearerToken, login } from "./helpers";
import { users } from "./seed";

/**
 * A lecture's statistics page, for its course's lecturers.
 *
 * Stream 7 ("VL 1: Livestream") is course 2's (Spieleentwicklung für Dummies), which
 * prof2 administers and prof1 does not; it is the only lecture the dump records
 * viewer samples for -- nine, taken while it was live. Course 1 is prof1's. The page
 * reads the fixture only, so the file needs no reload.
 */

const page7 = "/admin/courses/2/lectures/7/stats";

test.describe("the lecture statistics page", () => {
  test("is served the SPA shell", async ({ page }) => {
    await login(page, users.prof2);

    const response = await page.goto(page7);
    expect((await response?.text()) ?? "").toContain("/spa-assets/");
  });

  test("moved from the old path", async ({ page }) => {
    await login(page, users.prof2);

    await page.goto("/admin/stats/2/7");
    await expect(page).toHaveURL(page7);
  });

  test("names the lecture and links back to its course's statistics", async ({ page }) => {
    await login(page, users.prof2, page7);

    await expect(page.getByText("VL 1: Livestream")).toBeVisible();
    await page.getByRole("link", { name: "Spieleentwicklung für Dummies" }).click();
    await expect(page).toHaveURL("/admin/courses/2/stats");
  });

  test("shows the old page's counters", async ({ page }) => {
    await login(page, users.prof2, page7);

    // course_users has two rows for course 2.
    await expect(page.getByRole("row", { name: /Enrolled Students/ })).toContainText("2");
    await expect(page.getByRole("row", { name: /Lecture Time/ })).toContainText(/\d\d\.\d\d\.\d{4} \d\d:\d\d - \d\d:\d\d/);
    await expect(page.getByRole("row", { name: /Max Live Views/ })).toBeVisible();
  });

  test("draws its three charts and offers no export", async ({ page }) => {
    await login(page, users.prof2, page7);

    await expect(page.locator("canvas")).toHaveCount(3);
    // The old page had its export links commented out; there is no lecture export.
    await expect(page.getByRole("link", { name: /Export/ })).toHaveCount(0);
  });

  test("is refused by the server to a lecturer of other courses", async ({ page }) => {
    await login(page, users.prof1);

    const response = await page.goto(page7);
    expect(response?.ok()).toBe(false);
    expect((await response?.text()) ?? "").not.toContain("/spa-assets/");
  });

  test("shows nothing of a lecture reached through the wrong course", async ({ page }) => {
    // prof1 administers course 1, so the server serves the page; stream 7 is course
    // 2's, so the API refuses it.
    await login(page, users.prof1, "/admin/courses/1/lectures/7/stats");

    await expect(page.getByRole("alert")).toContainText("does not exist or you do not administer it");
    await expect(page.locator("canvas")).toHaveCount(0);
  });
});

test.describe("the lecture statistics API", () => {
  test("answers an administrator of the course", async ({ playwright }) => {
    const context = await apiAs(playwright, users.prof2);
    const token = await bearerToken(context);

    const response = await context.get("/api/v2/courses/2/streams/7/stats", {
      headers: { Authorization: `Bearer ${token}` },
    });

    expect(response.status()).toBe(200);
    const body = await response.json();
    expect(body.lectureName).toBe("VL 1: Livestream");
    expect(body.numStudents).toBe("2");
    expect(body.liveViewers.points).toHaveLength(9);
  });

  test.describe("refuses everyone else", () => {
    test("a lecturer of other courses", async ({ playwright }) => {
      const context = await apiAs(playwright, users.prof1);

      expect((await context.get("/api/v2/courses/2/streams/7/stats")).status()).toBe(404);
    });

    test("a lecturer naming another course's lecture through their own course", async ({
      playwright,
    }) => {
      const context = await apiAs(playwright, users.prof1);

      const response = await context.get("/api/v2/courses/1/streams/7/stats");
      expect(response.status()).toBe(404);
      expect((await response.json()).message).toBe("no such stream");
    });

    test("a student of the course", async ({ playwright }) => {
      const context = await apiAs(playwright, users.studi1);

      expect((await context.get("/api/v2/courses/2/streams/7/stats")).status()).toBe(404);
    });

    test("an anonymous caller", async ({ playwright }) => {
      const context = await apiAs(playwright);

      expect((await context.get("/api/v2/courses/2/streams/7/stats")).status()).toBe(401);
    });
  });
});
