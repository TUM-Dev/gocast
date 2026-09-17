import { expect, test } from "@playwright/test";

import { apiAs, bearerToken, login } from "./helpers";
import { users } from "./seed";

/**
 * The lecture halls admin page renders the fixture, and the endpoints behind it
 * refuse everyone without server.administer -- a hidden control being no substitute
 * for that.
 *
 * Camera preset management (the grid of images fetched from a hall's camera,
 * refreshing it, marking a default, taking a snapshot) is not part of this page yet;
 * see frontend/src/lib/lecture-halls.ts for why.
 */

test.describe("the lecture halls admin page", () => {
  test("is served the SPA shell", async ({ page }) => {
    await login(page, users.admin);

    const response = await page.goto("/admin/lecture-halls");
    expect((await response?.text()) ?? "").toContain("/spa-assets/");
  });

  test("lists the seeded hall by name", async ({ page }) => {
    await login(page, users.admin, "/admin/lecture-halls");

    await expect(page.locator("form").filter({ hasText: "HS001" })).toBeVisible();
  });

  test("offers the administration menu entry", async ({ page }) => {
    await login(page, users.admin, "/admin/lecture-halls");

    await expect(
      page.getByRole("navigation", { name: "Administration" }).getByRole("link", {
        name: "Lecture Halls",
      }),
    ).toBeVisible();
  });

  test("filters the list by name", async ({ page }) => {
    await login(page, users.admin, "/admin/lecture-halls");

    await page.getByPlaceholder("Filter by name").fill("nothing-matches-this");
    await expect(page.getByText('No lecture hall matches "nothing-matches-this".')).toBeVisible();
    await expect(page.locator("form").filter({ hasText: "HS001" })).toHaveCount(0);
  });

  test("is refused to a lecturer, who administers courses but not the server", async ({
    page,
  }) => {
    await login(page, users.prof1);

    const response = await page.goto("/admin/lecture-halls");
    expect(response?.status()).toBe(403);
    expect((await response?.text()) ?? "").not.toContain("/spa-assets/");
  });
});

test.describe("the lecture halls admin API", () => {
  test.describe("refuses everyone but a server administrator", () => {
    for (const account of ["studi1", "prof1"] as const) {
      test(`${account} may not list lecture halls for administration`, async ({ playwright }) => {
        const context = await apiAs(playwright, users[account]);
        const token = await bearerToken(context);

        const response = await context.get("/api/v2/admin/lecture-halls", {
          headers: { Authorization: `Bearer ${token}` },
        });

        expect(response.status()).toBe(403);
      });

      test(`${account} may not create a lecture hall`, async ({ playwright }) => {
        const context = await apiAs(playwright, users[account]);
        const token = await bearerToken(context);

        const response = await context.post("/api/v2/admin/lecture-halls", {
          headers: { Authorization: `Bearer ${token}` },
          data: { name: "Should Not Exist", streamProtocol: 1 },
        });

        expect(response.status()).toBe(403);
      });
    }

    test("an anonymous caller may not list lecture halls for administration", async ({
      playwright,
    }) => {
      const context = await apiAs(playwright);

      expect((await context.get("/api/v2/admin/lecture-halls")).status()).toBe(401);
    });
  });
});

/**
 * One flow rather than three separate tests, so create/edit/delete run against the
 * hall this suite itself created -- nothing else in the fixture can be reused for all
 * three without one test depending on another's mutation surviving out of order.
 */
test.describe("managing a lecture hall end to end", () => {
  test("a hall created, edited and deleted behaves at every step", async ({ page }) => {
    // Create, on its own page just as the legacy admin page had.
    await login(page, users.admin, "/admin/lecture-halls");
    await page.getByRole("link", { name: "+ New Lecture Hall" }).click();
    await expect(page).toHaveURL("/admin/lecture-halls/new");

    await page.getByLabel("Name", { exact: true }).fill("E2E_HS1");
    await page.getByLabel("Camera", { exact: true }).fill("rtsp://0.0.0.0/cam");
    await page.getByRole("button", { name: "Create" }).click();

    // Back on the list, with the new hall's form visible.
    await expect(page).toHaveURL("/admin/lecture-halls");
    const row = page.locator("form").filter({ hasText: "E2E_HS1" });
    await expect(row).toBeVisible();
    await expect(row.getByLabel("Camera", { exact: true })).toHaveValue("rtsp://0.0.0.0/cam");

    // Edit. The Save button stays disabled until a field actually changes.
    await expect(row.getByRole("button", { name: "Save" })).toBeDisabled();
    await row.getByLabel("Presentation", { exact: true }).fill("rtsp://0.0.0.0/pres");
    await expect(row.getByText("Unsaved changes")).toBeVisible();
    await row.getByRole("button", { name: "Save" }).click();
    await expect(row.getByText("Saved")).toBeVisible();
    await expect(row.getByRole("button", { name: "Save" })).toBeDisabled();

    // The edit survives a reload, i.e. it was actually persisted.
    await page.reload();
    const reloadedRow = page.locator("form").filter({ hasText: "E2E_HS1" });
    await expect(reloadedRow.getByLabel("Presentation", { exact: true })).toHaveValue(
      "rtsp://0.0.0.0/pres",
    );

    // Delete.
    page.once("dialog", (dialog) => dialog.accept());
    await reloadedRow.getByRole("button", { name: "Delete E2E_HS1" }).click();
    await expect(page.locator("form").filter({ hasText: "E2E_HS1" })).toHaveCount(0);
  });

  test("Reset discards unsaved changes without calling the API", async ({ page }) => {
    await login(page, users.admin, "/admin/lecture-halls");

    const row = page.locator("form").filter({ hasText: "HS001" });
    const nameInput = row.getByLabel("Name", { exact: true });
    const original = await nameInput.inputValue();

    await nameInput.fill(`${original}-changed`);
    await expect(row.getByText("Unsaved changes")).toBeVisible();

    await row.getByRole("button", { name: "Reset" }).click();
    await expect(nameInput).toHaveValue(original);
    await expect(row.getByText("Unsaved changes")).toHaveCount(0);
  });
});
