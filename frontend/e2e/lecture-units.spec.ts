import { expect, test } from "@playwright/test";

import { login } from "./helpers";
import { users } from "./seed";

/**
 * Lecture units were dropped; the page's URLs only redirect, so old bookmarks and
 * emailed links land on the course's lectures tab. Read-only, so safe to run in
 * parallel.
 */
test.describe("the removed lecture units page", () => {
  for (const path of ["/admin/units/2/7", "/admin/courses/2/lectures/7/units"]) {
    test(`${path} redirects to the course's lectures tab`, async ({ page }) => {
      await login(page, users.prof2);

      await page.goto(path);
      await expect(page).toHaveURL(/\/admin\/courses\/2\/lectures$/);
    });

    test(`${path} still refuses people who don't administer the course`, async ({ page }) => {
      await login(page, users.studi1, "/settings");

      await page.goto(path);
      await expect(page).not.toHaveURL(/\/admin\/courses\/2\/lectures/);
    });
  }
});
