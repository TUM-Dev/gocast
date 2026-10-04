import { expect, test } from "@playwright/test";

import { apiAs, bearerToken, login } from "./helpers";
import { users } from "./seed";

/**
 * Creating a course.
 *
 * This file writes: it creates a course and deletes it again afterwards through v1
 * (there is no v2 delete yet). The deletion is soft, and GetAvailableSemesters counts
 * deleted courses too, so the course goes in year 1234 -- the test semester that
 * dao/courses.go already leaves out of every public semester list. The dump's course 1 is brauereiwesen in the
 * summer semester of 2022, which a new course can therefore not be called.
 *
 * Neither CI nor a local stack runs Meilisearch with TUMOnline's course list, so the
 * search is only ever seen failing; the page has to stay usable when it does.
 */

const slug = "e2e-new";
const createdIds: number[] = [];

test.afterAll(async ({ playwright }) => {
  const context = await apiAs(playwright, users.admin);
  for (const id of createdIds) {
    const response = await context.delete(`/api/course/${id}/`);
    expect(response.ok(), `could not delete course ${id} after the test`).toBe(true);
  }
});

test.describe("the create course page", () => {
  test("is served the SPA shell", async ({ page }) => {
    await login(page, users.prof1);

    const response = await page.goto("/admin/create-course");
    expect((await response?.text()) ?? "").toContain("/spa-assets/");
  });

  test("is reached from the administration menu", async ({ page }) => {
    await login(page, users.prof1, "/admin/create-course");

    await expect(
      page.getByRole("navigation", { name: "Administration" }).getByRole("link", { name: "Create Course" }),
    ).toBeVisible();
  });

  test("is refused to a student by the server", async ({ page }) => {
    await login(page, users.studi1);

    const response = await page.goto("/admin/create-course");
    expect(response?.status()).toBe(403);
    expect((await response?.text()) ?? "").not.toContain("/spa-assets/");
  });

  test("stays usable when the TUMOnline search is not", async ({ page }) => {
    await login(page, users.prof1, "/admin/create-course");

    await page.getByRole("searchbox", { name: "Find your course in TUMOnline" }).fill("Bier");

    await expect(page.getByText("Enter the course manually below")).toBeVisible();
    await expect(page.getByRole("textbox", { name: "Course title", exact: true })).toBeEditable();
  });

  test("drops what a slug cannot hold as it is typed", async ({ page }) => {
    await login(page, users.prof1, "/admin/create-course");

    const slugField = page.getByRole("textbox", { name: "Slug", exact: true });
    await slugField.pressSequentially("Ei Di/2!");
    await expect(slugField).toHaveValue("EiDi2");
  });

  test("refuses a slug the semester already has", async ({ page }) => {
    await login(page, users.prof1, "/admin/create-course");

    await page.getByRole("textbox", { name: "Course title", exact: true }).fill("Noch einmal Bier");
    await page.getByRole("textbox", { name: "Slug", exact: true }).fill("brauereiwesen");
    await page.getByLabel("Semester").selectOption("S");
    await page.getByRole("spinbutton", { name: "Year" }).fill("2022");
    await page.getByRole("button", { name: "Create Course" }).click();

    await expect(page.getByRole("alert")).toContainText('already has a course with the slug "brauereiwesen"');
  });

  test("creates the course and opens its page for its creator", async ({ page }) => {
    await login(page, users.prof1, "/admin/create-course");

    await page.getByRole("textbox", { name: "Course title", exact: true }).fill("E2E Neuer Kurs");
    await page.getByRole("textbox", { name: "Slug", exact: true }).fill(slug);
    await page.getByLabel("Semester").selectOption("S");
    await page.getByRole("spinbutton", { name: "Year" }).fill("1234");
    await page.getByLabel("Language").selectOption("en");
    await page.getByRole("button", { name: "Create Course" }).click();

    await expect(page).toHaveURL(/\/admin\/course\/\d+\?created$/);
    createdIds.push(Number(/\/admin\/course\/(\d+)/.exec(page.url())?.[1]));

    // The course page is behind AdminOfCourse, so getting it at all shows prof1 was
    // made an administrator of the new course.
    await expect(page.getByText("Course was created successfully.")).toBeVisible();
    await expect(page.getByRole("link", { name: "E2E Neuer Kurs" })).toBeVisible();
  });
});

test.describe("the create course API", () => {
  // Never created: every request below is refused. Still in the test semester.
  const body = { name: "E2E API Kurs", slug: "e2e-api", year: 1234, term: "S", language: "" };

  test("refuses a student", async ({ playwright }) => {
    const context = await apiAs(playwright, users.studi1);

    expect((await context.post("/api/v2/courses", { data: body })).status()).toBe(403);
    expect((await context.get("/api/v2/tumonline-courses?q=bier")).status()).toBe(403);
  });

  test("refuses an anonymous caller", async ({ playwright }) => {
    const context = await apiAs(playwright);

    expect((await context.post("/api/v2/courses", { data: body })).status()).toBe(401);
  });

  test("rejects a slug that could not be a URL segment", async ({ playwright }) => {
    const context = await apiAs(playwright, users.prof1);
    const token = await bearerToken(context);

    const response = await context.post("/api/v2/courses", {
      data: { ...body, slug: "e2e/api" },
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(response.status()).toBe(400);
  });

  test("answers the search as unavailable rather than failing", async ({ playwright }) => {
    const context = await apiAs(playwright, users.prof1);

    expect((await context.get("/api/v2/tumonline-courses?q=bier")).status()).toBe(503);
  });
});
