import { expect, test } from "@playwright/test";

import { apiAs } from "./helpers";

/**
 * The set-password page at /setPassword/:key. No key is outstanding in the fixture
 * and none can be minted through the API without a mail, so what is covered is the
 * page's handling of a dead link and its own validation; the happy path is unit
 * tested on both sides.
 */

test.describe("the set-password page", () => {
  test("is served the SPA shell without a session", async ({ page }) => {
    const response = await page.goto("/setPassword/never-issued");
    expect(response?.status()).toBe(200);
    expect((await response?.text()) ?? "").toContain("/spa-assets/");
  });

  test("says when the link is dead and points at the login page", async ({ page }) => {
    await page.goto("/setPassword/never-issued");

    await expect(page.getByRole("alert")).toContainText("This link is not valid any more");
    await page.getByRole("link", { name: "login page" }).click();
    await expect(page).toHaveURL(/\/login$/);
  });

  test("no longer takes a form post", async ({ playwright }) => {
    const anonymous = await apiAs(playwright);
    const response = await anonymous.post("/setPassword/never-issued", {
      form: { password: "correct horse", passwordConfirm: "correct horse" },
    });
    expect(response.status()).toBe(404);
  });
});
