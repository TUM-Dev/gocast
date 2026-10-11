import { expect, test } from "@playwright/test";

import { login } from "./helpers";
import { users } from "./seed";

/**
 * The search page at /search. The dev stack has no Meilisearch, so a query always
 * ends in the "not available" notice; what is covered is the page, its query
 * parameter, the filters and the header field landing here. The hit lists are unit
 * tested against canned responses.
 */

test.describe("the search page", () => {
  test("is served the SPA shell, signed in or not", async ({ page }) => {
    const anonymous = await page.goto("/search");
    expect(anonymous?.status()).toBe(200);
    expect((await anonymous?.text()) ?? "").toContain("/spa-assets/");

    await login(page, users.studi1);
    const response = await page.goto("/search?q=bier");
    expect((await response?.text()) ?? "").toContain("/spa-assets/");
  });

  test("takes the query from the URL and keeps it there", async ({ page }) => {
    await page.goto("/search?q=bier");
    const box = page.getByRole("searchbox", { name: /Search for courses/ });
    await expect(box).toHaveValue("bier");

    await box.fill("hopfen");
    await expect(page).toHaveURL(/\/search\?q=hopfen$/);
  });

  test("says when search is not available", async ({ page }) => {
    await page.goto("/search?q=bier");
    await expect(page.getByRole("alert")).toHaveText("Search is not available right now. Please try again later.");
  });

  test("asks for three characters first", async ({ page }) => {
    await page.goto("/search?q=bi");
    await expect(page.getByText("Type at least 3 characters to search.")).toBeVisible();
    await expect(page.getByRole("alert")).toHaveCount(0);
  });

  test("offers the semesters and, once one is picked, its courses", async ({ page }) => {
    await login(page, users.studi1, "/search?q=bier");

    await page.getByRole("button", { name: "Semester" }).click();
    await page.getByRole("checkbox", { name: "Summer 2022" }).check();
    await page.getByRole("button", { name: /^Courses/ }).click();
    await expect(page.getByRole("checkbox", { name: /Einführung Brauereiwesen/ })).toBeVisible();
  });

  test("is where the header's search field goes", async ({ page }) => {
    await login(page, users.studi1, "/settings");

    await page.getByRole("textbox", { name: "Search courses" }).fill("bier");
    await page.getByRole("textbox", { name: "Search courses" }).press("Enter");
    await expect(page).toHaveURL(/\/search\?q=bier$/);
    await expect(page.getByRole("heading", { name: "Search" })).toBeVisible();
  });
});
