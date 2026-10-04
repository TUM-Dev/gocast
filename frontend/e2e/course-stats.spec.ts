import { expect, test } from "@playwright/test";

import { apiAs, bearerToken, login } from "./helpers";
import { users } from "./seed";

/**
 * A course's statistics page, for its lecturers.
 *
 * Course 1 (Einführung Brauereiwesen, 2022) is administered by prof1 and prof2 and has
 * three enrolled students and three lectures that are recorded or live. Course 2 is
 * prof2's but not prof1's. The page reads the fixture only, so the file needs no
 * reload.
 */

const page1 = "/admin/courses/1/stats";

test.describe("the course statistics page", () => {
  test("is served the SPA shell", async ({ page }) => {
    await login(page, users.prof1);

    const response = await page.goto(page1);
    expect((await response?.text()) ?? "").toContain("/spa-assets/");
  });

  test("moved from the old singular path", async ({ page }) => {
    await login(page, users.prof1);

    await page.goto("/admin/course/1/stats");
    await expect(page).toHaveURL(page1);
  });

  test("shows the course's own quick stats, not the server's", async ({ page }) => {
    await login(page, users.prof1, page1);

    await expect(page.getByRole("link", { name: "Einführung Brauereiwesen" })).toBeVisible();
    // course_users has three rows for course 1 and eight in all.
    await expect(page.getByRole("row", { name: /Enrolled Students/ })).toContainText("3");
    await expect(page.getByRole("row", { name: /Lectures/ })).toContainText("3");
  });

  test("draws a chart per activity breakdown", async ({ page }) => {
    await login(page, users.prof1, page1);

    await expect(page.locator("canvas")).toHaveCount(5);
  });

  test("warns that a 2022 course predates part of the viewing data", async ({ page }) => {
    await login(page, users.prof1, page1);

    await expect(page.getByText("only captured from June 28th 2021")).toBeVisible();
  });

  test("offers both export formats as direct, authenticated downloads", async ({ page }) => {
    await login(page, users.prof1, page1);

    await expect(page.getByRole("link", { name: "Export as JSON" })).toHaveAttribute(
      "href",
      "/api/v2/courses/1/stats/export?format=json",
    );
    await expect(page.getByRole("link", { name: "Export as CSV" })).toHaveAttribute(
      "href",
      "/api/v2/courses/1/stats/export?format=csv",
    );
  });

  test("is refused by the server to a lecturer of other courses", async ({ page }) => {
    await login(page, users.prof1);

    const response = await page.goto("/admin/courses/2/stats");
    expect(response?.ok()).toBe(false);
    expect((await response?.text()) ?? "").not.toContain("/spa-assets/");
  });

  test("is refused by the server to a student of the course", async ({ page }) => {
    await login(page, users.studi1);

    const response = await page.goto(page1);
    expect(response?.ok()).toBe(false);
    expect((await response?.text()) ?? "").not.toContain("/spa-assets/");
  });
});

test.describe("the course statistics API", () => {
  test("answers an administrator of the course", async ({ playwright }) => {
    const context = await apiAs(playwright, users.prof2);
    const token = await bearerToken(context);

    const response = await context.get("/api/v2/courses/1/stats", {
      headers: { Authorization: `Bearer ${token}` },
    });

    expect(response.status()).toBe(200);
    const body = await response.json();
    expect(body.courseName).toBe("Einführung Brauereiwesen");
    expect(body.numStudents).toBe("3");
    expect(body.partialHistory).toBe(true);
  });

  test("answers a server administrator for any course", async ({ playwright }) => {
    const context = await apiAs(playwright, users.admin);
    const token = await bearerToken(context);

    const response = await context.get("/api/v2/courses/3/stats", {
      headers: { Authorization: `Bearer ${token}` },
    });

    expect(response.status()).toBe(200);
  });

  test("exports json and csv", async ({ playwright }) => {
    // Through the session cookie, as the page's download links are followed.
    const context = await apiAs(playwright, users.prof1);

    for (const format of ["json", "csv"]) {
      const response = await context.get(`/api/v2/courses/1/stats/export?format=${format}`);
      expect(response.status()).toBe(200);
      expect((await response.body()).length).toBeGreaterThan(0);
    }
  });

  test("refuses an unknown export format", async ({ playwright }) => {
    const context = await apiAs(playwright, users.prof1);

    expect((await context.get("/api/v2/courses/1/stats/export?format=xml")).status()).toBe(400);
  });

  test.describe("refuses everyone else", () => {
    test("a lecturer of other courses", async ({ playwright }) => {
      const context = await apiAs(playwright, users.prof1);

      expect((await context.get("/api/v2/courses/2/stats")).status()).toBe(404);
      expect((await context.get("/api/v2/courses/2/stats/export?format=csv")).status()).toBe(404);
    });

    test("a student of the course", async ({ playwright }) => {
      const context = await apiAs(playwright, users.studi1);

      expect((await context.get("/api/v2/courses/1/stats")).status()).toBe(404);
    });

    test("an anonymous caller", async ({ playwright }) => {
      const context = await apiAs(playwright);

      expect((await context.get("/api/v2/courses/1/stats")).status()).toBe(401);
    });

    test("anyone asking for course 0, which the queries read as every course", async ({
      playwright,
    }) => {
      // Even a server administrator: the server-wide numbers have their own endpoint.
      const context = await apiAs(playwright, users.admin);

      expect((await context.get("/api/v2/courses/0/stats")).status()).toBe(404);
      expect((await context.get("/api/v2/courses/0/stats/export?format=json")).status()).toBe(404);
    });
  });
});
