import { expect, test } from "@playwright/test";

/**
 * The pages a lecturer reaches from the course-import mail: /edit-course?token= and
 * /edit-course/opt-out?token=. The fixture's courses carry no token, so what is
 * covered is the shell, a link without a token and a token no course has; the
 * opt-in and opt-out themselves are unit tested on both sides.
 */

test.describe("the course token pages", () => {
  test("are served the SPA shell without a session", async ({ page }) => {
    for (const path of ["/edit-course?token=x", "/edit-course/opt-out?token=x"]) {
      const response = await page.goto(path);
      expect(response?.status()).toBe(200);
      expect((await response?.text()) ?? "").toContain("/spa-assets/");
    }
  });

  test("say when the link carries no token", async ({ page }) => {
    await page.goto("/edit-course");
    await expect(page.getByRole("alert")).toContainText("carries no token");
  });

  test("say when no course matches the token, on both pages", async ({ page }) => {
    await page.goto("/edit-course?token=not-a-token");
    await expect(page.getByRole("heading", { name: "Enable your course" })).toBeVisible();
    await expect(page.getByRole("alert")).toContainText("No course matches this link");

    await page.goto("/edit-course/opt-out?token=not-a-token");
    await expect(page.getByRole("heading", { name: "Opt out of your course" })).toBeVisible();
    await expect(page.getByRole("alert")).toContainText("No course matches this link");
    await expect(page.getByRole("button", { name: "Confirm" })).toHaveCount(0);
  });
});
