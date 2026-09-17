import { expect, test, type Page } from "@playwright/test";

import { apiAs, bearerToken, login } from "./helpers";
import { users } from "./seed";

/**
 * The course import page reaches TUMonline directly rather than this deployment's own
 * database, so there is no fixture to seed a course search against. The wizard's
 * search and import steps are exercised here against a mocked API response instead;
 * what a real search returns is covered by apiv2/server/course_import_test.go against
 * a fake campus client, and by a manual run against the review environment.
 *
 * NOTE: this spec has not been run. Per the migration brief, sibling agents are
 * running the dev server and Playwright concurrently in this branch's environment, so
 * this suite was written but not executed here.
 */

const searchResponse = {
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
};

async function fillSearchForm(page: Page): Promise<void> {
  await page.getByLabel("From").fill("2025-10-01");
  await page.getByLabel("To").fill("2025-10-31");
}

test.describe("the course import page", () => {
  test("is served the SPA shell", async ({ page }) => {
    await login(page, users.admin);

    const response = await page.goto("/admin/course-import");
    expect((await response?.text()) ?? "").toContain("/spa-assets/");
  });

  test("offers the administration menu entry", async ({ page }) => {
    await login(page, users.admin, "/admin/course-import");

    await expect(
      page.getByRole("navigation", { name: "Administration" }).getByRole("link", {
        name: "Course Import",
      }),
    ).toBeVisible();
  });

  test("is refused to a lecturer, who administers courses but not the server", async ({
    page,
  }) => {
    await login(page, users.prof1);

    const response = await page.goto("/admin/course-import");
    expect(response?.status()).toBe(403);
    expect((await response?.text()) ?? "").not.toContain("/spa-assets/");
  });

  test("searches, reviews and imports the found courses", async ({ page }) => {
    await page.route("**/api/v2/admin/course-import/search", (route) =>
      route.fulfill({ json: searchResponse }),
    );
    await page.route("**/api/v2/admin/course-import", (route) =>
      route.fulfill({
        json: { results: [{ title: "Grundlagen der Datenbanken", success: true, error: "" }] },
      }),
    );

    await login(page, users.admin, "/admin/course-import");
    await fillSearchForm(page);
    await page.getByRole("button", { name: "Start Import" }).click();

    await expect(page.getByRole("heading", { name: "Grundlagen der Datenbanken" })).toBeVisible();
    await expect(page.getByText("MI HS1")).toBeVisible();

    await page.getByRole("button", { name: "Import" }).click();

    await expect(page.getByText("Grundlagen der Datenbanken")).toBeVisible();
    await expect(page.getByRole("link", { name: "Return to homepage" })).toBeVisible();
  });

  test("surfaces a search failure instead of leaving the page hanging", async ({ page }) => {
    // The bug the v2 handler fixes: a failed TUMonline client used to log and return
    // with no response at all.
    await page.route("**/api/v2/admin/course-import/search", (route) =>
      route.fulfill({ status: 502, json: { message: "fetching the schedule from TUMonline: timeout" } }),
    );

    await login(page, users.admin, "/admin/course-import");
    await fillSearchForm(page);
    await page.getByRole("button", { name: "Start Import" }).click();

    await expect(page.getByRole("alert")).toContainText("timeout");
  });

  test("reports one course's import failure without hiding another's success", async ({
    page,
  }) => {
    await page.route("**/api/v2/admin/course-import/search", (route) =>
      route.fulfill({
        json: {
          courses: [
            searchResponse.courses[0],
            { ...searchResponse.courses[0], title: "Fine Course", courseId: 43, slug: "FIN" },
          ],
        },
      }),
    );
    await page.route("**/api/v2/admin/course-import", (route) =>
      route.fulfill({
        json: {
          results: [
            { title: "Grundlagen der Datenbanken", success: false, error: "duplicate slug" },
            { title: "Fine Course", success: true, error: "" },
          ],
        },
      }),
    );

    await login(page, users.admin, "/admin/course-import");
    await fillSearchForm(page);
    await page.getByRole("button", { name: "Start Import" }).click();
    await page.getByRole("button", { name: "Import" }).click();

    await expect(page.getByText("duplicate slug")).toBeVisible();
    await expect(page.getByText("Fine Course")).toBeVisible();
  });
});

test.describe("the course import API", () => {
  test.describe("refuses everyone but a server administrator", () => {
    for (const account of ["studi1", "prof1"] as const) {
      test(`${account} may not search the TUMonline schedule`, async ({ playwright }) => {
        const context = await apiAs(playwright, users[account]);
        const token = await bearerToken(context);

        const response = await context.post("/api/v2/admin/course-import/search", {
          headers: { Authorization: `Bearer ${token}` },
          data: { departmentId: 53598, from: "2025-10-01T00:00:00Z", to: "2025-10-31T00:00:00Z" },
        });

        expect(response.status()).toBe(403);
      });

      test(`${account} may not import courses`, async ({ playwright }) => {
        const context = await apiAs(playwright, users[account]);
        const token = await bearerToken(context);

        const response = await context.post("/api/v2/admin/course-import", {
          headers: { Authorization: `Bearer ${token}` },
          data: { year: 2025, term: "W", optIn: false, courses: [] },
        });

        expect(response.status()).toBe(403);
      });
    }

    test("an anonymous caller may not search the TUMonline schedule", async ({ playwright }) => {
      const context = await apiAs(playwright);

      expect((await context.post("/api/v2/admin/course-import/search")).status()).toBe(401);
    });
  });
});
