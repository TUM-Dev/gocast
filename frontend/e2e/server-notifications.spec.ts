import { expect, test } from "@playwright/test";

import { apiAs, bearerToken, login } from "./helpers";
import { serverNotifications, users } from "./seed";

/**
 * The server notifications admin page renders the fixture, and the endpoints behind
 * it refuse everyone without server.administer -- a hidden control being no
 * substitute for that.
 */

test.describe("the server notifications admin page", () => {
  test("is served the SPA shell", async ({ page }) => {
    await login(page, users.admin);

    const response = await page.goto("/admin/server-notifications");
    expect((await response?.text()) ?? "").toContain("/spa-assets/");
  });

  test("lists the seeded notifications by text and type", async ({ page }) => {
    await login(page, users.admin, "/admin/server-notifications");

    const info = page.locator("li").filter({ hasText: serverNotifications[0].text });
    await expect(info).toContainText("Info");

    const warning = page.locator("li").filter({ hasText: serverNotifications[1].text });
    await expect(warning).toContainText("Warning");
  });

  test("offers the administration menu entry", async ({ page }) => {
    await login(page, users.admin, "/admin/server-notifications");

    await expect(
      page.getByRole("navigation", { name: "Administration" }).getByRole("link", {
        name: "Server Notifications",
      }),
    ).toBeVisible();
  });

  test("is refused to a lecturer, who administers courses but not the server", async ({
    page,
  }) => {
    await login(page, users.prof1);

    const response = await page.goto("/admin/server-notifications");
    expect(response?.status()).toBe(403);
    expect((await response?.text()) ?? "").not.toContain("/spa-assets/");
  });
});

test.describe("the server notifications admin API", () => {
  test.describe("refuses everyone but a server administrator", () => {
    for (const account of ["studi1", "prof1"] as const) {
      test(`${account} may not list server notifications for administration`, async ({
        playwright,
      }) => {
        const context = await apiAs(playwright, users[account]);
        const token = await bearerToken(context);

        const response = await context.get("/api/v2/admin/server-notifications", {
          headers: { Authorization: `Bearer ${token}` },
        });

        expect(response.status()).toBe(403);
      });

      test(`${account} may not create a server notification`, async ({ playwright }) => {
        const context = await apiAs(playwright, users[account]);
        const token = await bearerToken(context);

        const response = await context.post("/api/v2/admin/server-notifications", {
          headers: { Authorization: `Bearer ${token}` },
          data: {
            text: "Should not exist",
            warn: false,
            start: "2026-01-01T00:00:00Z",
            expires: "2026-01-02T00:00:00Z",
          },
        });

        expect(response.status()).toBe(403);
      });
    }

    test("an anonymous caller may not list server notifications for administration", async ({
      playwright,
    }) => {
      const context = await apiAs(playwright);

      expect((await context.get("/api/v2/admin/server-notifications")).status()).toBe(401);
    });
  });

  test("refuses a notification whose expiry is before its start", async ({ playwright }) => {
    const context = await apiAs(playwright, users.admin);
    const token = await bearerToken(context);

    const response = await context.post("/api/v2/admin/server-notifications", {
      headers: { Authorization: `Bearer ${token}` },
      data: {
        text: "Backwards window",
        warn: false,
        start: "2026-01-02T00:00:00Z",
        expires: "2026-01-01T00:00:00Z",
      },
    });

    expect(response.status()).toBe(400);
  });
});

/**
 * One flow rather than three separate tests, so create/edit/delete run against the
 * notification this suite itself created -- nothing seeded can be reused for all
 * three without one test depending on another's mutation surviving out of order.
 */
test.describe("managing a server notification end to end", () => {
  test("a notification created, edited and deleted behaves at every step", async ({ page }) => {
    await login(page, users.admin, "/admin/server-notifications");

    // Create.
    await page.getByRole("button", { name: "Add notification" }).click();
    const createForm = page
      .locator("form")
      .filter({ has: page.getByRole("button", { name: "Submit notification" }) });
    await createForm.getByLabel("Message").fill("Scheduled downtime tonight.");
    await createForm.getByLabel("From").fill("2026-01-01T00:00");
    await createForm.getByLabel("Expires").fill("2026-01-02T00:00");
    await createForm.getByRole("button", { name: "Submit notification" }).click();

    const row = page.locator("li").filter({ hasText: "Scheduled downtime tonight." });
    await expect(row).toBeVisible();
    await expect(row).toContainText("Info");

    // Edit: switch it to a warning.
    await row.getByRole("button", { name: "Edit Scheduled downtime tonight." }).click();
    const editForm = page.locator("li").filter({ has: page.getByRole("button", { name: "Save" }) });
    await editForm.getByLabel("Warning").check();
    await editForm.getByRole("button", { name: "Save" }).click();
    await expect(row).toContainText("Warning");

    // Delete.
    page.once("dialog", (dialog) => dialog.accept());
    await row.getByRole("button", { name: "Delete Scheduled downtime tonight." }).click();
    await expect(row).toHaveCount(0);
  });
});
