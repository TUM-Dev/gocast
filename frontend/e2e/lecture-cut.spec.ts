import { expect, test } from "@playwright/test";

import { login } from "./helpers";
import { users } from "./seed";

/**
 * The cut editor's old address redirects to its place under the lecture, so bookmarks
 * and emailed links keep working. Read-only, so safe to run in parallel.
 */
test.describe("the lecture cut page", () => {
  test("the old address redirects to the editor", async ({ page }) => {
    await login(page, users.prof2);

    await page.goto("/admin/cut/2/7");
    await expect(page).toHaveURL(/\/admin\/courses\/2\/lectures\/7\/cut$/);
    await expect(page.getByRole("heading", { name: "Cut video" })).toBeVisible();
  });

  test("refuses people who don't administer the course", async ({ page }) => {
    await login(page, users.studi1, "/settings");

    await page.goto("/admin/cut/2/7");
    await expect(page).not.toHaveURL(/\/admin\/courses\/2\/lectures\/7\/cut$/);
  });
});
