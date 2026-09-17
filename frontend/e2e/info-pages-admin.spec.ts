import { expect, test } from "@playwright/test";

import { apiAs, bearerToken, login } from "./helpers";
import { dynamicInfoPage, infoPages, users } from "./seed";

/**
 * The info pages admin page renders the fixture, and the endpoints behind it refuse
 * everyone without server.administer — a hidden control being no substitute for that.
 */

test.describe("the info pages admin page", () => {
  test("is served the SPA shell", async ({ page }) => {
    await login(page, users.admin);

    const response = await page.goto("/admin/info-pages");
    expect((await response?.text()) ?? "").toContain("/spa-assets/");
  });

  test("lists the built-in pages by title and slug", async ({ page }) => {
    await login(page, users.admin, "/admin/info-pages");

    const row = page.getByText(infoPages.privacy.title).locator("..");
    await expect(row).toContainText("/privacy");
  });

  test("offers the administration menu entry", async ({ page }) => {
    await login(page, users.admin, "/admin/info-pages");

    await expect(
      page.getByRole("navigation", { name: "Administration" }).getByRole("link", {
        name: "Info Pages",
      }),
    ).toBeVisible();
  });

  test("is refused to a lecturer, who administers courses but not the server", async ({
    page,
  }) => {
    await login(page, users.prof1);

    const response = await page.goto("/admin/info-pages");
    expect(response?.status()).toBe(403);
    expect((await response?.text()) ?? "").not.toContain("/spa-assets/");
  });
});

test.describe("the info pages admin API", () => {
  test.describe("refuses everyone but a server administrator", () => {
    for (const account of ["studi1", "prof1"] as const) {
      test(`${account} may not list info pages for administration`, async ({ playwright }) => {
        const context = await apiAs(playwright, users[account]);
        const token = await bearerToken(context);

        const response = await context.get("/api/v2/admin/info-pages", {
          headers: { Authorization: `Bearer ${token}` },
        });

        expect(response.status()).toBe(403);
      });

      test(`${account} may not create an info page`, async ({ playwright }) => {
        const context = await apiAs(playwright, users[account]);
        const token = await bearerToken(context);

        const response = await context.post("/api/v2/admin/info-pages", {
          headers: { Authorization: `Bearer ${token}` },
          data: { slug: "should-not-exist", name: "Should Not Exist", rawContent: "# No" },
        });

        expect(response.status()).toBe(403);
      });
    }

    test("an anonymous caller may not list info pages for administration", async ({
      playwright,
    }) => {
      const context = await apiAs(playwright);

      expect((await context.get("/api/v2/admin/info-pages")).status()).toBe(401);
    });
  });

  test("refuses a slug that collides with a route the server already owns", async ({
    playwright,
  }) => {
    const context = await apiAs(playwright, users.admin);
    const token = await bearerToken(context);

    const response = await context.post("/api/v2/admin/info-pages", {
      headers: { Authorization: `Bearer ${token}` },
      data: { slug: "login", name: "Not Actually Login", rawContent: "# No" },
    });

    expect(response.status()).toBe(409);
  });
});

/**
 * One flow rather than three separate tests, so create/edit/delete run against the
 * page this suite itself created — nothing else in the fixture can be reused for all
 * three without one test depending on another's mutation surviving out of order.
 */
test.describe("managing an info page end to end", () => {
  test("a page created, edited and deleted behaves at every step", async ({ page }) => {
    await login(page, users.admin, "/admin/info-pages");

    // Create. Scoped to the form itself: its labels ("Title", "Slug") are not unique
    // on the page once a row is also being edited.
    const createForm = page.locator("form").filter({ has: page.getByRole("button", { name: "Create" }) });
    await createForm.getByLabel("Title", { exact: true }).fill("Terms of Use");
    await createForm.getByLabel("Slug", { exact: true }).fill("terms-of-use");
    await createForm.getByLabel("Content (Markdown)").fill("# Terms\n\nBe reasonable.");
    await createForm.getByRole("button", { name: "Create" }).click();

    // Scoped to the list item: the status message left by creating the page also
    // contains its title, which a bare text match would ambiguously match too.
    const row = page.locator("li").filter({ hasText: "Terms of Use" });
    await expect(row).toContainText("/terms-of-use");

    // The page is reachable immediately, with no deploy in between.
    await page.goto("/terms-of-use");
    await expect(page.getByRole("heading", { name: "Terms", level: 1 })).toBeVisible();

    // Edit. Scoped to the row's own form, which replaces its static view.
    await page.goto("/admin/info-pages");
    await page.getByRole("button", { name: "Edit Terms of Use" }).click();
    const editForm = page.locator("li").filter({ has: page.getByRole("button", { name: "Save" }) });
    await editForm.getByLabel("Title", { exact: true }).fill("Terms & Conditions");
    await editForm.getByRole("button", { name: "Save" }).click();
    const renamedRow = page.locator("li").filter({ hasText: "Terms & Conditions" });
    await expect(renamedRow).toBeVisible();

    // Delete.
    page.once("dialog", (dialog) => dialog.accept());
    await page.getByRole("button", { name: "Delete Terms & Conditions" }).click();
    await expect(renamedRow).toHaveCount(0);

    // The route stops resolving immediately, same as the admin list.
    const response = await page.request.get("/api/v2/info-pages/terms-of-use");
    expect(response.status()).toBe(404);
  });
});

test.describe("a page added after the built-in three", () => {
  test("is editable from the same list as privacy, imprint and about", async ({ page }) => {
    await login(page, users.admin, "/admin/info-pages");

    const row = page.getByText(dynamicInfoPage.title).locator("..");
    await expect(row).toContainText(`/${dynamicInfoPage.slug}`);
  });
});
