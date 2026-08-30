import { expect, test } from "@playwright/test";

import { apiAs, bearerToken, login } from "./helpers";
import { runners, users } from "./seed";

/**
 * The runners page renders the fixture, and the endpoints behind it refuse everyone
 * without server.administer — a hidden control being no substitute for that.
 */

test.describe("the runners page", () => {
  test("is served the SPA shell", async ({ page }) => {
    await login(page, users.admin);

    const response = await page.goto("/admin/runners");
    expect((await response?.text()) ?? "").toContain("/spa-assets/");
  });

  test("lists the registered runners with their version and workload", async ({ page }) => {
    await login(page, users.admin, "/admin/runners");

    const alpha = page.getByRole("row", { name: new RegExp(runners.alpha.hostname) });
    await expect(alpha).toBeVisible();
    await expect(alpha).toContainText(runners.alpha.version);
    await expect(alpha).toContainText(`${runners.alpha.jobCount} Jobs`);
  });

  test("shows a runner that has not been heard from as dead", async ({ page }) => {
    // No fixture can seed a live runner: liveness is a heartbeat within five seconds.
    await login(page, users.admin, "/admin/runners");

    const alpha = page.getByRole("row", { name: new RegExp(runners.alpha.hostname) });
    await expect(alpha).toContainText("Dead");
  });

  test("marks a draining runner as such", async ({ page }) => {
    // Why a runner stops taking work while still registered.
    await login(page, users.admin, "/admin/runners");

    const beta = page.getByRole("row", { name: new RegExp(runners.beta.hostname) });
    await expect(beta).toContainText("draining");
  });

  test("offers the administration menu the account actually has", async ({ page }) => {
    await login(page, users.admin, "/admin/runners");

    const nav = page.getByRole("navigation", { name: "Administration" });
    await expect(nav.getByRole("link", { name: "Runners" })).toBeVisible();
    // Behind users.manage rather than server.administer, and an admin holds both.
    await expect(nav.getByRole("link", { name: "Users" })).toBeVisible();
  });

  test("is refused to a student by the server, not by the page", async ({ page }) => {
    // Registered inside the permission group, so the shell is never sent.
    await login(page, users.studi1);

    const response = await page.goto("/admin/runners");
    expect(response?.status()).toBe(403);
    expect((await response?.text()) ?? "").not.toContain("/spa-assets/");
  });

  test("is refused to a lecturer, who administers courses but not the server", async ({ page }) => {
    await login(page, users.prof1);

    const response = await page.goto("/admin/runners");
    expect(response?.status()).toBe(403);
  });
});

test.describe("the runners API", () => {
  test("answers a server administrator", async ({ playwright }) => {
    const context = await apiAs(playwright, users.admin);
    const token = await bearerToken(context);

    const response = await context.get("/api/v2/admin/runners", {
      headers: { Authorization: `Bearer ${token}` },
    });

    expect(response.status()).toBe(200);
    const hostnames = (await response.json()).runners.map((r: { hostname: string }) => r.hostname);
    expect(hostnames).toContain(runners.alpha.hostname);
  });

  test.describe("refuses everyone else", () => {
    for (const account of ["studi1", "prof1"] as const) {
      test(`${account} may not list runners`, async ({ playwright }) => {
        const context = await apiAs(playwright, users[account]);
        const token = await bearerToken(context);

        const response = await context.get("/api/v2/admin/runners", {
          headers: { Authorization: `Bearer ${token}` },
        });

        expect(response.status()).toBe(403);
      });

      test(`${account} may not delete a runner`, async ({ playwright }) => {
        // The one that matters: a refused write, not just a hidden page.
        const context = await apiAs(playwright, users[account]);
        const token = await bearerToken(context);

        const response = await context.delete(`/api/v2/admin/runners/${runners.alpha.hostname}`, {
          headers: { Authorization: `Bearer ${token}` },
        });

        expect(response.status()).toBe(403);
      });
    }

    test("an anonymous caller may not list runners", async ({ playwright }) => {
      const context = await apiAs(playwright);

      expect((await context.get("/api/v2/admin/runners")).status()).toBe(401);
    });
  });
});

/**
 * Last in the file: it consumes `runner-beta`, which nothing can recreate. Acceptable
 * only because `make test_e2e` reloads the dump every run.
 */
test.describe("removing a runner", () => {
  test("removes the row and leaves the others alone", async ({ page }) => {
    await login(page, users.admin, "/admin/runners");

    const beta = page.getByRole("row", { name: new RegExp(runners.beta.hostname) });
    await expect(beta).toBeVisible();

    // The prompt has to be accepted rather than dismissed.
    page.once("dialog", (dialog) => dialog.accept());
    await page.getByRole("button", { name: `Remove ${runners.beta.hostname}` }).click();

    await expect(beta).toHaveCount(0);
    await expect(
      page.getByRole("row", { name: new RegExp(runners.alpha.hostname) }),
    ).toBeVisible();
  });
});
