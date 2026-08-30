import { expect, test } from "@playwright/test";

import { login } from "./helpers";
import { courses, semesters, users } from "./seed";

/**
 * The start page, which replaced web/template/home.gohtml.
 *
 * These cover what the unit tests structurally cannot: that the page fills against a
 * real server for a caller who is signed in and for one who is not, and that the URLs
 * the old page handed out still land somewhere.
 *
 * They assert shape rather than the names of particular courses, so a change to the
 * development seed does not break them.
 */

/** The seeded database has courses in this semester; the current one is empty. */
const SEEDED = semesters.summer2022.query;

test.describe("an anonymous visitor", () => {
  test("is shown the public courses", async ({ page }) => {
    // The regression this guards: the page loads its semester list before anything
    // else, and asking for that with a bearer token failed for a visitor with no
    // session — leaving the whole start page blank rather than logged out.
    await page.goto(`/${SEEDED}`);

    const sidebar = page.locator("#side-navigation");
    await expect(sidebar.getByText("Public Courses")).toBeVisible();
    await expect(sidebar.locator('a[href^="/course/"]').first()).toBeVisible();
  });

  test("is offered the login button rather than an account menu", async ({ page }) => {
    await page.goto(`/${SEEDED}`);

    await expect(page.getByRole("link", { name: "Login" })).toBeVisible();
  });

  test("can open a course from the sidebar without a full page load", async ({ page }) => {
    await page.goto(`/${SEEDED}`);

    const course = page.locator('#side-navigation a[href^="/course/"]').first();
    const name = (await course.textContent())?.trim() ?? "";
    await course.click();

    await expect(page).toHaveURL(/\/course\/\d+\/[WS]\//);
    await expect(page.locator(".tum-live-course-view .name")).toHaveText(name);
  });
});

test.describe("a signed-in user", () => {
  test("is greeted by name", async ({ page }) => {
    await login(page, users.studi1, `/${SEEDED}`);

    await expect(page.locator("#greeting")).toBeVisible();
  });

  test("is offered the pin control on a course", async ({ page }) => {
    await login(page, users.studi1, `/${SEEDED}`);

    await page.locator('#side-navigation a[href^="/course/"]').first().click();
    await expect(page.getByRole("button", { name: /^(Pin|Pinned)$/ })).toBeVisible();
  });
});

test.describe("the URLs the old page handed out", () => {
  // One path and a `view` parameter, driven by an Alpine state machine. They are in
  // bookmarks and in browser history, and the client router translates them.

  test("?view=1 becomes the enrolled listing", async ({ page }) => {
    await login(page, users.studi1, `/${SEEDED}&view=1`);

    await expect(page).toHaveURL(/\/courses\/mine\?/);
  });

  test("?view=2 becomes the public listing", async ({ page }) => {
    await page.goto(`/${SEEDED}&view=2`);

    await expect(page).toHaveURL(/\/courses\/public\?/);
    await expect(page.getByRole("heading", { name: "Public Courses" })).toBeVisible();
  });

  test("?view=3 becomes the course page", async ({ page }) => {
    await page.goto(`/${SEEDED}&slug=brauereiwesen&view=3`);

    await expect(page).toHaveURL(/\/course\/2022\/S\/brauereiwesen$/);
    await expect(page.locator(".tum-live-course-view .name")).toHaveText(
      courses.brauereiwesen.name,
    );
  });

  test("/semester/:year/:term still lands on the start page", async ({ page }) => {
    await page.goto("/semester/2022/S");

    await expect(page).toHaveURL(/\/\?year=2022&term=S$/);
  });
});
