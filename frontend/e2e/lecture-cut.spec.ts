import { expect, test } from "@playwright/test";

import { login } from "./helpers";
import { users } from "./seed";

/**
 * The cut editor was dropped; its URL only redirects, so old bookmarks and emailed
 * links land on the course's lectures tab. Read-only, so safe to run in parallel.
 */
test.describe("the removed lecture cut page", () => {
  test("redirects to the course's lectures tab", async ({ page }) => {
    await login(page, users.prof2);

    await page.goto("/admin/cut/2/7");
    await expect(page).toHaveURL(/\/admin\/courses\/2\/lectures$/);
  });

  test("still refuses people who don't administer the course", async ({ page }) => {
    await login(page, users.studi1, "/settings");

    await page.goto("/admin/cut/2/7");
    await expect(page).not.toHaveURL(/\/admin\/courses\/2\/lectures/);
  });
});
