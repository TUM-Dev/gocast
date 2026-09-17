import { expect, test } from "@playwright/test";

import { apiAs, bearerToken, login } from "./helpers";
import { users } from "./seed";

/**
 * The server statistics page renders the fixture's quick counters and charts, and the
 * endpoints behind it refuse everyone without server.administer — a hidden control
 * being no substitute for that.
 *
 * api/statistics.go's getStats/exportStats handlers still serve the per-course and
 * per-lecture statistics pages (courseID != 0) and are unrelated to this page beyond
 * sharing the "courseID 0 means every course" convention; those handlers, their
 * template and web/ts/stats.ts are left untouched by this migration.
 */

test.describe("the server statistics page", () => {
  test("is served the SPA shell", async ({ page }) => {
    await login(page, users.admin);

    const response = await page.goto("/admin/server-stats");
    expect((await response?.text()) ?? "").toContain("/spa-assets/");
  });

  test("shows the quick stats computed across every course", async ({ page }) => {
    await login(page, users.admin, "/admin/server-stats");

    // course_users has 8 rows across the whole fixture, and GetCourseNumStudents
    // treats courseID 0 as "count them all" rather than filtering by course.
    const row = page.getByRole("row", { name: /Enrolled Students/ });
    await expect(row).toBeVisible();
    await expect(row).toContainText("8");
  });

  test("draws a chart per activity breakdown", async ({ page }) => {
    await login(page, users.admin, "/admin/server-stats");

    // One canvas each for live activity, VoD activity, hourly, weekday and daily —
    // matching the five charts the old page drew with the same library.
    await expect(page.locator("canvas")).toHaveCount(5);
  });

  test("offers both export formats as direct, authenticated downloads", async ({ page }) => {
    await login(page, users.admin, "/admin/server-stats");

    await expect(page.getByRole("link", { name: "Export as JSON" })).toHaveAttribute(
      "href",
      "/api/v2/admin/server-stats/export?format=json",
    );
    await expect(page.getByRole("link", { name: "Export as CSV" })).toHaveAttribute(
      "href",
      "/api/v2/admin/server-stats/export?format=csv",
    );
  });

  test("offers the administration menu entry", async ({ page }) => {
    await login(page, users.admin, "/admin/server-stats");

    await expect(
      page
        .getByRole("navigation", { name: "Administration" })
        .getByRole("link", { name: "Server Statistics" }),
    ).toBeVisible();
  });

  test("is refused to a lecturer, who administers courses but not the server", async ({
    page,
  }) => {
    await login(page, users.prof1);

    const response = await page.goto("/admin/server-stats");
    expect(response?.status()).toBe(403);
    expect((await response?.text()) ?? "").not.toContain("/spa-assets/");
  });

  test("is refused to a student by the server, not by the page", async ({ page }) => {
    await login(page, users.studi1);

    const response = await page.goto("/admin/server-stats");
    expect(response?.status()).toBe(403);
  });
});

test.describe("the server statistics API", () => {
  test("answers a server administrator", async ({ playwright }) => {
    const context = await apiAs(playwright, users.admin);
    const token = await bearerToken(context);

    const response = await context.get("/api/v2/admin/server-stats", {
      headers: { Authorization: `Bearer ${token}` },
    });

    expect(response.status()).toBe(200);
    const body = await response.json();
    expect(body.numStudents).toBe("8");
  });

  test("exports json and csv", async ({ playwright }) => {
    const context = await apiAs(playwright, users.admin);
    const token = await bearerToken(context);

    for (const format of ["json", "csv"]) {
      const response = await context.get(`/api/v2/admin/server-stats/export?format=${format}`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      expect(response.status()).toBe(200);
      expect((await response.body()).length).toBeGreaterThan(0);
    }
  });

  test("refuses an unknown export format", async ({ playwright }) => {
    const context = await apiAs(playwright, users.admin);
    const token = await bearerToken(context);

    const response = await context.get("/api/v2/admin/server-stats/export?format=xml", {
      headers: { Authorization: `Bearer ${token}` },
    });

    expect(response.status()).toBe(400);
  });

  test.describe("refuses everyone else", () => {
    for (const account of ["studi1", "prof1"] as const) {
      test(`${account} may not read server statistics`, async ({ playwright }) => {
        const context = await apiAs(playwright, users[account]);
        const token = await bearerToken(context);

        const response = await context.get("/api/v2/admin/server-stats", {
          headers: { Authorization: `Bearer ${token}` },
        });

        expect(response.status()).toBe(403);
      });
    }

    test("an anonymous caller may not read server statistics", async ({ playwright }) => {
      const context = await apiAs(playwright);

      expect((await context.get("/api/v2/admin/server-stats")).status()).toBe(401);
    });
  });
});
