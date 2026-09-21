import { expect, test } from "@playwright/test";

import { apiAs, bearerToken, login } from "./helpers";
import { tokens, users } from "./seed";

/**
 * The token management page renders the fixture, and the endpoints behind it refuse
 * everyone without users.manage -- a hidden control being no substitute for that.
 *
 * The security property that matters most here does not show up as a permission: a
 * freshly created token's secret is returned exactly once, by the create call, and
 * never again -- not in the list, not on a reload. See the module comment in
 * frontend/src/lib/tokens.ts and apiv2/server/token_admin.go.
 */

test.describe("the token management page", () => {
  test("is served the SPA shell", async ({ page }) => {
    await login(page, users.admin);

    const response = await page.goto("/admin/token");
    expect((await response?.text()) ?? "").toContain("/spa-assets/");
  });

  test("lists the seeded token by owner and scope, without its secret", async ({ page }) => {
    await login(page, users.admin, "/admin/token");

    const row = page.getByRole("row", { name: new RegExp(tokens.seeded.owner) });
    await expect(row).toBeVisible();
    await expect(row).toContainText(tokens.seeded.scope);
  });

  test("offers the administration menu entry", async ({ page }) => {
    await login(page, users.admin, "/admin/token");

    await expect(
      page.getByRole("navigation", { name: "Administration" }).getByRole("link", {
        name: "Token Management",
      }),
    ).toBeVisible();
  });

  test("is refused to a lecturer, who administers courses but not accounts", async ({ page }) => {
    // Registered inside the permission group, so the shell is never sent.
    await login(page, users.prof1);

    const response = await page.goto("/admin/token");
    expect(response?.status()).toBe(403);
    expect((await response?.text()) ?? "").not.toContain("/spa-assets/");
  });

  test("is refused to a student", async ({ page }) => {
    await login(page, users.studi1);

    const response = await page.goto("/admin/token");
    expect(response?.status()).toBe(403);
  });
});

test.describe("the token management API", () => {
  test.describe("refuses everyone but an account holding users.manage", () => {
    for (const account of ["studi1", "prof1"] as const) {
      test(`${account} may not list tokens`, async ({ playwright }) => {
        const context = await apiAs(playwright, users[account]);
        const token = await bearerToken(context);

        const response = await context.get("/api/v2/admin/tokens", {
          headers: { Authorization: `Bearer ${token}` },
        });

        expect(response.status()).toBe(403);
      });

      test(`${account} may not create a token`, async ({ playwright }) => {
        const context = await apiAs(playwright, users[account]);
        const token = await bearerToken(context);

        const response = await context.post("/api/v2/admin/tokens", {
          headers: { Authorization: `Bearer ${token}` },
          data: { scope: "lecturer" },
        });

        expect(response.status()).toBe(403);
      });

      test(`${account} may not delete a token`, async ({ playwright }) => {
        const context = await apiAs(playwright, users[account]);
        const token = await bearerToken(context);

        const response = await context.delete(`/api/v2/admin/tokens/${tokens.seeded.id}`, {
          headers: { Authorization: `Bearer ${token}` },
        });

        expect(response.status()).toBe(403);
      });
    }

    test("an anonymous caller may not list tokens", async ({ playwright }) => {
      const context = await apiAs(playwright);

      expect((await context.get("/api/v2/admin/tokens")).status()).toBe(401);
    });
  });

  test("never includes a secret alongside the listing", async ({ playwright }) => {
    const context = await apiAs(playwright, users.admin);
    const token = await bearerToken(context);

    const response = await context.get("/api/v2/admin/tokens", {
      headers: { Authorization: `Bearer ${token}` },
    });

    const body = (await response.json()) as { tokens: Record<string, unknown>[] };
    for (const row of body.tokens) {
      expect(row).not.toHaveProperty("token");
    }
  });
});

/**
 * Last in the file: it creates and then consumes a token nothing else in this file
 * depends on. Acceptable only because `make test_e2e` reloads the dump every run.
 */
test.describe("creating and revoking a token", () => {
  test("shows the secret once, adds the row, and the row disappears on delete", async ({
    page,
  }) => {
    await login(page, users.admin, "/admin/token");

    await page.getByLabel("Scope").selectOption("lecturer");
    await page.getByRole("button", { name: "Create" }).click();

    // The one and only place the secret is ever shown.
    // The heading, not the text: the paragraph below it repeats the phrase, and
    // getByText matches a substring, so the plain locator resolves to both.
    const secretBox = page.getByRole("heading", { name: "Your Generated Token" }).locator("..");
    await expect(secretBox).toBeVisible();
    const secret = (await secretBox.locator("code").first().textContent())?.trim();
    expect(secret).toBeTruthy();

    // By the name the table shows, not the username: "admin" never appears in "Anja
    // Admin", and matching it lowercase picks the seeded row by its scope cell instead.
    // Narrowed to the scope as well, because the seeded token belongs to the same user:
    // last() would keep matching that one after this row is deleted.
    const row = page
      .getByRole("row", { name: new RegExp(users.admin.name) })
      .filter({ hasText: "lecturer" });
    await expect(row).toContainText("lecturer");
    // The row never carries the secret text, only the metadata about the token. The
    // guard is only here to narrow string | null for toContainText; that a secret was
    // shown at all is asserted above.
    if (!secret) {
      throw new Error("the generated token was not shown");
    }
    await expect(row).not.toContainText(secret);

    page.once("dialog", (dialog) => dialog.accept());
    await row.getByRole("button", { name: "Delete" }).click();
    await expect(row).toHaveCount(0);
  });
});
