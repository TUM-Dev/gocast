import { expect, test } from "@playwright/test";

/**
 * The onboarding page at /onboarding, where a fresh deployment creates its first
 * administrator. The fixture has users, so what can be covered is the page refusing
 * and the start page not redirecting; the fresh case is unit tested on both sides.
 */

test.describe("the onboarding page", () => {
  test("is served the SPA shell without a session", async ({ page }) => {
    const response = await page.goto("/onboarding");
    expect(response?.status()).toBe(200);
    expect((await response?.text()) ?? "").toContain("/spa-assets/");
  });

  test("says the deployment is set up and points at the login", async ({ page }) => {
    await page.goto("/onboarding");

    await expect(page.getByRole("status")).toContainText("already set up");
    await expect(page.getByRole("button", { name: "Finish setup" })).toHaveCount(0);
    await page.getByRole("link", { name: "Sign in" }).click();
    await expect(page).toHaveURL(/\/login$/);
  });

  test("is not where the start page sends a deployment with users", async ({ page }) => {
    await page.goto("/");
    await expect(page).toHaveURL(/\/$/);
  });
});
