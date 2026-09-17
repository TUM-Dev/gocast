import { expect, test } from "@playwright/test";

import { apiAs, bearerToken, login } from "./helpers";
import { audits, users } from "./seed";

/**
 * The audits page renders the fixture, paginates it ten at a time as the old page
 * did, and the endpoint behind it refuses everyone without server.administer — a
 * hidden control being no substitute for that.
 */

test.describe("the audits page", () => {
  test("is served the SPA shell", async ({ page }) => {
    await login(page, users.admin);

    const response = await page.goto("/admin/audits");
    expect((await response?.text()) ?? "").toContain("/spa-assets/");
  });

  test("lists the newest entry first, with its type and message", async ({ page }) => {
    await login(page, users.admin, "/admin/audits");

    const row = page.getByText(audits.newest.message).locator("..");
    await expect(row).toContainText(audits.newest.type);
  });

  test("shows who made an entry, alongside their user id", async ({ page }) => {
    await login(page, users.admin, "/admin/audits");

    const row = page.getByText(audits.newest.message).locator("..");
    // GetLoginString falls back to the "email" column, which the fixture admin
    // account has set to its username rather than a real address.
    await expect(row).toContainText(users.admin.username);
    await expect(row).toContainText("(1)");
  });

  test("shows no user for a system entry", async ({ page }) => {
    await login(page, users.admin, "/admin/audits");

    const row = page.getByText(audits.system.message).locator("..");
    // Rendered only when userId is non-zero; a system audit's user_id is null.
    await expect(row.locator("i.fa-user")).toHaveCount(0);
  });

  test("pages forward and back through the log", async ({ page }) => {
    await login(page, users.admin, "/admin/audits");

    await expect(page.getByText(audits.newest.message)).toBeVisible();
    // Twelve rows seeded, ten to a page: the oldest two are not on the first page.
    await expect(page.getByText(audits.oldest.message)).not.toBeVisible();

    const previous = page.getByRole("button", { name: "Previous" });
    const next = page.getByRole("button", { name: "Next" });
    await expect(previous).toBeDisabled();

    await next.click();

    await expect(page.getByText(audits.oldest.message)).toBeVisible();
    await expect(page.getByText(audits.newest.message)).not.toBeVisible();
    // Two rows on the second page: nothing further to fetch.
    await expect(next).toBeDisabled();

    await previous.click();

    await expect(page.getByText(audits.newest.message)).toBeVisible();
  });

  test("offers the administration menu entry", async ({ page }) => {
    await login(page, users.admin, "/admin/audits");

    await expect(
      page.getByRole("navigation", { name: "Administration" }).getByRole("link", {
        name: "Audits",
      }),
    ).toBeVisible();
  });

  test("is refused to a lecturer, who administers courses but not the server", async ({
    page,
  }) => {
    await login(page, users.prof1);

    const response = await page.goto("/admin/audits");
    expect(response?.status()).toBe(403);
  });

  test("is refused to a student by the server, not by the page", async ({ page }) => {
    await login(page, users.studi1);

    const response = await page.goto("/admin/audits");
    expect(response?.status()).toBe(403);
    expect((await response?.text()) ?? "").not.toContain("/spa-assets/");
  });
});

test.describe("the audits API", () => {
  test("answers a server administrator with the newest page first", async ({ playwright }) => {
    const context = await apiAs(playwright, users.admin);
    const token = await bearerToken(context);

    const response = await context.get("/api/v2/admin/audits?limit=10&offset=0", {
      headers: { Authorization: `Bearer ${token}` },
    });

    expect(response.status()).toBe(200);
    const messages = (await response.json()).audits.map((a: { message: string }) => a.message);
    expect(messages[0]).toBe(audits.newest.message);
    expect(messages).toHaveLength(10);
  });

  test("answers a second page from the given offset", async ({ playwright }) => {
    const context = await apiAs(playwright, users.admin);
    const token = await bearerToken(context);

    const response = await context.get("/api/v2/admin/audits?limit=10&offset=10", {
      headers: { Authorization: `Bearer ${token}` },
    });

    const messages = (await response.json()).audits.map((a: { message: string }) => a.message);
    expect(messages).toContain(audits.oldest.message);
    expect(messages).toHaveLength(2);
  });

  test.describe("refuses everyone else", () => {
    for (const account of ["studi1", "prof1"] as const) {
      test(`${account} may not list audits`, async ({ playwright }) => {
        const context = await apiAs(playwright, users[account]);
        const token = await bearerToken(context);

        const response = await context.get("/api/v2/admin/audits", {
          headers: { Authorization: `Bearer ${token}` },
        });

        expect(response.status()).toBe(403);
      });
    }

    test("an anonymous caller may not list audits", async ({ playwright }) => {
      const context = await apiAs(playwright);

      expect((await context.get("/api/v2/admin/audits")).status()).toBe(401);
    });
  });
});
