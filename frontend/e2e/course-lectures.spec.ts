import { expect, test, type Page } from "@playwright/test";

import { apiAs, login } from "./helpers";
import { recordings, schedule, users } from "./seed";

/**
 * A course's lecture list, the edit-course page's lectures tab.
 *
 * Course 1 (Einführung Brauereiwesen) is administered by prof1 and prof2; course 2 is
 * prof2's alone. The one test that renames a lecture puts the exact name back in a
 * finally, through the API so a failing page cannot leave it changed: other specs
 * look lectures up by name. No test here moves a lecture in time.
 */

const page1 = "/admin/courses/1/lectures";
/** Planned, so renaming it touches nothing other specs read about recordings. */
const renamed = schedule.planned[1];

const cards = (page: Page) => page.locator('li[id^="lecture-"]');
const card = (page: Page, name: string) =>
  cards(page).filter({ has: page.getByRole("heading", { name, exact: true }) });

test.describe("the course lectures page", () => {
  test("is served the SPA shell", async ({ page }) => {
    await login(page, users.prof1);

    const response = await page.goto(page1);
    expect((await response?.text()) ?? "").toContain("/spa-assets/");
  });

  test("lists the course's lectures with their state", async ({ page }) => {
    await login(page, users.prof1, page1);

    await expect(page.getByRole("heading", { name: "Lectures", level: 1 })).toBeVisible();
    await expect(page.getByRole("link", { name: "Einführung Brauereiwesen" })).toBeVisible();
    for (const name of [...recordings.brauereiwesen, schedule.today, ...schedule.planned]) {
      await expect(card(page, name)).toHaveCount(1);
    }
    await expect(card(page, "VL 1: Was ist Bier?").getByRole("listitem").first()).toHaveText("VoD");
    await expect(card(page, schedule.planned[0]).getByRole("listitem").first()).toHaveText("Scheduled");
  });

  test("sorts newest or oldest first, and remembers the choice", async ({ page }) => {
    await login(page, users.prof1, page1);
    const headings = cards(page).locator("h2");
    await expect(headings.first()).toBeVisible();

    const newestFirst = await headings.allTextContents();
    expect(newestFirst.at(-1)).toBe("VL 1: Was ist Bier?");

    // The button names the order shown.
    await page.getByRole("button", { name: "Newest first" }).click();
    await expect(headings.first()).toHaveText("VL 1: Was ist Bier?");

    await page.reload();
    await expect(headings.first()).toHaveText("VL 1: Was ist Bier?");

    // Back to the default, so the next test starts where a new visitor does.
    await page.getByRole("button", { name: "Oldest first" }).click();
    await expect(headings.last()).toHaveText("VL 1: Was ist Bier?");
  });

  test("renames a lecture from its card", async ({ page, playwright }) => {
    await login(page, users.prof1, page1);

    const target = card(page, renamed);
    const id = (await target.getAttribute("id"))!.replace("lecture-", "");
    const newName = `${renamed} (umbenannt)`;

    try {
      await target.getByRole("button", { name: "Edit" }).click();
      const title = page.locator(`#lecture-${id}-name`);
      await expect(title).toHaveValue(renamed);
      await title.fill(newName);
      await title.locator("xpath=ancestor::form").getByRole("button", { name: "Save" }).click();
      await expect(page.locator(`#lecture-${id}`).getByRole("status")).toHaveText(
        "Title and description saved.",
      );

      await page.reload();
      await expect(page.locator(`#lecture-${id} h2`)).toHaveText(newName);
    } finally {
      const prof1 = await apiAs(playwright, users.prof1);
      const restored = await prof1.patch(`/api/v2/courses/1/streams/${id}`, { data: { name: renamed } });
      expect(restored.status()).toBe(200);
    }

    await page.reload();
    await expect(page.locator(`#lecture-${id} h2`)).toHaveText(renamed);
  });

  test("opens the card the fragment names", async ({ page }) => {
    await login(page, users.prof1);

    await page.goto(`${page1}#lecture-1`);
    await expect(page.locator("#lecture-1-name")).toHaveValue("VL 1: Was ist Bier?");
    await expect(page.locator("#lecture-1").getByRole("button", { name: "Close" })).toBeVisible();
    // Only that one.
    await expect(page.getByRole("button", { name: "Close" })).toHaveCount(1);
  });

  test("is refused by the server to a lecturer of other courses", async ({ page }) => {
    await login(page, users.prof1);

    const response = await page.goto("/admin/courses/2/lectures");
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
