import { expect, test } from "@playwright/test";

import { apiAs, bearerToken, login } from "./helpers";
import { users } from "./seed";

/**
 * The user notifications admin page renders the fixture, and the endpoints behind it
 * refuse everyone without server.administer — a hidden control being no substitute
 * for that.
 */

test.describe("the user notifications admin page", () => {
  test("is served the SPA shell", async ({ page }) => {
    await login(page, users.admin);

    const response = await page.goto("/admin/notifications");
    expect((await response?.text()) ?? "").toContain("/spa-assets/");
  });

  test("offers the administration menu entry", async ({ page }) => {
    await login(page, users.admin, "/admin/notifications");

    await expect(
      page.getByRole("navigation", { name: "Administration" }).getByRole("link", {
        name: "User Notifications",
      }),
    ).toBeVisible();
  });

  test("is refused to a lecturer, who administers courses but not the server", async ({
    page,
  }) => {
    await login(page, users.prof1);

    const response = await page.goto("/admin/notifications");
    expect(response?.status()).toBe(403);
    expect((await response?.text()) ?? "").not.toContain("/spa-assets/");
  });
});

test.describe("the user notifications admin API", () => {
  test.describe("refuses everyone but a server administrator", () => {
    for (const account of ["studi1", "prof1"] as const) {
      test(`${account} may not list notifications for administration`, async ({ playwright }) => {
        const context = await apiAs(playwright, users[account]);
        const token = await bearerToken(context);

        const response = await context.get("/api/v2/admin/notifications", {
          headers: { Authorization: `Bearer ${token}` },
        });

        expect(response.status()).toBe(403);
      });

      test(`${account} may not create a notification`, async ({ playwright }) => {
        const context = await apiAs(playwright, users[account]);
        const token = await bearerToken(context);

        const response = await context.post("/api/v2/admin/notifications", {
          headers: { Authorization: `Bearer ${token}` },
          data: { body: "should not exist", target: 1 },
        });

        expect(response.status()).toBe(403);
      });
    }

    test("an anonymous caller may not list notifications for administration", async ({
      playwright,
    }) => {
      const context = await apiAs(playwright);

      expect((await context.get("/api/v2/admin/notifications")).status()).toBe(401);
    });
  });
});

/**
 * One flow rather than two separate tests, so create/delete run against the
 * notification this suite itself created rather than depending on fixture state.
 */
test.describe("broadcasting a notification end to end", () => {
  test("a notification created and deleted behaves at every step", async ({ page }) => {
    await login(page, users.admin, "/admin/notifications");

    await page.getByLabel("Notification Target").selectOption({ label: "Admins" });
    await page.getByLabel("Title (optional)").fill("Scheduled maintenance");
    await page.getByLabel("Body (you can use Markdown)").fill("**Downtime** tonight at 10pm.");
    await page.getByRole("button", { name: "Create" }).click();

    const row = page.locator("li").filter({ hasText: "Scheduled maintenance" });
    await expect(row).toBeVisible();
    // The body is rendered from Markdown server-side, not shown as raw source.
    await expect(row.locator("strong")).toHaveText("Downtime");

    // The form clears and is ready for the next one.
    await expect(page.getByLabel("Title (optional)")).toHaveValue("");

    page.once("dialog", (dialog) => dialog.accept());
    await page.getByRole("button", { name: "Delete Scheduled maintenance" }).click();
    await expect(row).toHaveCount(0);
  });

  test("a notification without a title is labeled and deletable", async ({ page }) => {
    await login(page, users.admin, "/admin/notifications");

    await page.getByLabel("Body (you can use Markdown)").fill("No title on this one.");
    await page.getByRole("button", { name: "Create" }).click();

    const row = page.locator("li").filter({ hasText: "No title on this one." });
    await expect(row).toContainText("No title");

    page.once("dialog", (dialog) => dialog.accept());
    await page.getByRole("button", { name: "Delete this notification" }).click();
    await expect(row).toHaveCount(0);
  });
});
