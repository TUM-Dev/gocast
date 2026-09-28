import { expect, test } from "@playwright/test";

import { apiAs, bearerToken, login } from "./helpers";
import { users, workers } from "./seed";

/**
 * The workers page renders the fixture, and the endpoints behind it refuse everyone
 * without server.administer — a hidden control being no substitute for that.
 *
 * NOT RUN as part of this change: see the PR description for why.
 */

test.describe("the workers page", () => {
  test("is served the SPA shell", async ({ page }) => {
    await login(page, users.admin);

    const response = await page.goto("/admin/workers");
    expect((await response?.text()) ?? "").toContain("/spa-assets/");
  });

  test("lists the registered workers with their version and workload", async ({ page }) => {
    await login(page, users.admin, "/admin/workers");

    const alpha = page.getByRole("row", { name: new RegExp(workers.alpha.host) });
    await expect(alpha).toBeVisible();
    await expect(alpha).toContainText(workers.alpha.version);
    await expect(alpha).toContainText(String(workers.alpha.workload));
  });

  test("shows a worker that has not been heard from as dead", async ({ page }) => {
    // No fixture can seed a live worker: liveness is a heartbeat within six minutes.
    await login(page, users.admin, "/admin/workers");

    const alpha = page.getByRole("row", { name: new RegExp(workers.alpha.host) });
    await expect(alpha).toContainText("Dead");
  });

  test("shows the token and add-a-worker instructions", async ({ page }) => {
    await login(page, users.admin, "/admin/workers");

    await expect(page.getByRole("heading", { name: "How to add a worker" })).toBeVisible();
    await expect(page.getByText("export Token=")).toBeVisible();

    await page.getByRole("button", { name: "Docker", exact: true }).click();
    await expect(page.getByText("docker run")).toBeVisible();
  });

  test("offers the administration menu the account actually has", async ({ page }) => {
    await login(page, users.admin, "/admin/workers");

    const nav = page.getByRole("navigation", { name: "Administration" });
    await expect(nav.getByRole("link", { name: "Workers" })).toBeVisible();
    // Behind users.manage rather than server.administer, and an admin holds both.
    await expect(nav.getByRole("link", { name: "Users" })).toBeVisible();
  });

  test("is refused to a student by the server, not by the page", async ({ page }) => {
    // Registered inside the permission group, so the shell is never sent.
    await login(page, users.studi1);

    const response = await page.goto("/admin/workers");
    expect(response?.status()).toBe(403);
    expect((await response?.text()) ?? "").not.toContain("/spa-assets/");
  });

  test("is refused to a lecturer, who administers courses but not the server", async ({ page }) => {
    await login(page, users.prof1);

    const response = await page.goto("/admin/workers");
    expect(response?.status()).toBe(403);
  });
});

test.describe("the workers API", () => {
  test("answers a server administrator", async ({ playwright }) => {
    const context = await apiAs(playwright, users.admin);
    const token = await bearerToken(context);

    const response = await context.get("/api/v2/admin/workers", {
      headers: { Authorization: `Bearer ${token}` },
    });

    expect(response.status()).toBe(200);
    const ids = (await response.json()).workers.map((w: { workerId: string }) => w.workerId);
    expect(ids).toContain(workers.alpha.workerId);
  });

  test.describe("refuses everyone else", () => {
    for (const account of ["studi1", "prof1"] as const) {
      test(`${account} may not list workers`, async ({ playwright }) => {
        const context = await apiAs(playwright, users[account]);
        const token = await bearerToken(context);

        const response = await context.get("/api/v2/admin/workers", {
          headers: { Authorization: `Bearer ${token}` },
        });

        expect(response.status()).toBe(403);
      });

      test(`${account} may not delete a worker`, async ({ playwright }) => {
        // The one that matters: a refused write, not just a hidden page.
        const context = await apiAs(playwright, users[account]);
        const token = await bearerToken(context);

        const response = await context.delete(`/api/v2/admin/workers/${workers.alpha.workerId}`, {
          headers: { Authorization: `Bearer ${token}` },
        });

        expect(response.status()).toBe(403);
      });
    }

    test("an anonymous caller may not list workers", async ({ playwright }) => {
      const context = await apiAs(playwright);

      expect((await context.get("/api/v2/admin/workers")).status()).toBe(401);
    });
  });
});

/**
 * Last in the file: it consumes `worker-beta`, which nothing can recreate. Acceptable
 * only because `make test_e2e` reloads the dump every run.
 */
test.describe("removing a worker", () => {
  test("removes the row and leaves the others alone", async ({ page }) => {
    await login(page, users.admin, "/admin/workers");

    const beta = page.getByRole("row", { name: new RegExp(workers.beta.host) });
    await expect(beta).toBeVisible();

    // The prompt has to be accepted rather than dismissed.
    page.once("dialog", (dialog) => dialog.accept());
    await page.getByRole("button", { name: `Remove ${workers.beta.host}` }).click();

    await expect(beta).toHaveCount(0);
    await expect(
      page.getByRole("row", { name: new RegExp(workers.alpha.host) }),
    ).toBeVisible();
  });
});
