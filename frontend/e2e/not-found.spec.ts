import { expect, test } from "@playwright/test";

import { apiAs, login } from "./helpers";
import { users } from "./seed";

/**
 * The 404 page. Go answers a path nothing owns with the SPA shell and that status;
 * the client renders the not-found page on that first load, but still hands an
 * in-app navigation to a page Go renders back to it.
 */

test.describe("a path nothing owns", () => {
  test("gets the SPA shell with a 404 and the not-found page", async ({ page }) => {
    const response = await page.goto("/no/such/page");

    expect(response?.status()).toBe(404);
    expect((await response?.text()) ?? "").toContain("/spa-assets/");
    await expect(page.getByRole("heading", { name: "This page does not exist." })).toBeVisible();
    await page.getByRole("link", { name: "Back to the start page" }).click();
    await expect(page).toHaveURL(/\/$/);
  });

  test("is a 404 for a signed-in user as well", async ({ page }) => {
    await login(page, users.studi1);
    const response = await page.goto("/no/such/page");

    expect(response?.status()).toBe(404);
    await expect(page.getByRole("heading", { name: "This page does not exist." })).toBeVisible();
  });

  test("under /api/ keeps the plain error page", async ({ playwright }) => {
    const anonymous = await apiAs(playwright);
    const response = await anonymous.get("/api/no/such/route");

    expect(response.status()).toBe(404);
    expect(await response.text()).not.toContain("/spa-assets/");
    expect(await response.text()).toContain("This page does not exist.");
  });
});
