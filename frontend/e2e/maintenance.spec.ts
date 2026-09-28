import { expect, test } from "@playwright/test";

import { apiAs, bearerToken, login } from "./helpers";
import { emailFailures, transcodingFailures, users } from "./seed";

/**
 * The maintenance page renders the fixture, and the endpoints behind it refuse
 * everyone without server.administer — a hidden control being no substitute for that.
 *
 * Thumbnail regeneration itself is not exercised end to end beyond starting it: each
 * file's regeneration dials a transcoding worker over gRPC, and the e2e environment
 * registers none, so a run finishes immediately with every file skipped. What is
 * covered is the part visible to an administrator — the button disabling itself the
 * moment a run starts.
 */

test.describe("the maintenance page", () => {
  test("is served the SPA shell", async ({ page }) => {
    await login(page, users.admin);

    const response = await page.goto("/admin/maintenance");
    expect((await response?.text()) ?? "").toContain("/spa-assets/");
  });

  test("offers the administration menu entry", async ({ page }) => {
    await login(page, users.admin, "/admin/maintenance");

    await expect(
      page
        .getByRole("navigation", { name: "Administration" })
        .getByRole("link", { name: "Maintenance" }),
    ).toBeVisible();
  });

  test("is refused to a lecturer, who administers courses but not the server", async ({
    page,
  }) => {
    await login(page, users.prof1);

    const response = await page.goto("/admin/maintenance");
    expect(response?.status()).toBe(403);
    expect((await response?.text()) ?? "").not.toContain("/spa-assets/");
  });

  test.describe("thumbnails", () => {
    test("starts a run and disables the button while it is in progress", async ({ page }) => {
      await login(page, users.admin, "/admin/maintenance");

      // Matched by a pattern rather than the exact label: the button's own text
      // switches to "Regenerating…" once a run starts, and re-querying by its old
      // name would then find nothing.
      const button = page.getByRole("button", { name: /Regenerat/ });
      await button.click();

      // The server marks the run started before the request returns, precisely so
      // this is not a race against a background goroutine.
      await expect(button).toBeDisabled();
    });
  });

  test.describe("cron jobs", () => {
    test("lists the jobs the scheduler registered at boot", async ({ page }) => {
      await login(page, users.admin, "/admin/maintenance");

      const select = page.getByRole("combobox", { name: "Cron job" });
      await expect(select).toContainText("fetchCourses");
    });

    test("the run button stays disabled until a job is chosen", async ({ page }) => {
      await login(page, users.admin, "/admin/maintenance");

      await expect(page.getByRole("button", { name: "Run" })).toBeDisabled();
    });

    test("runs a chosen job and reports success", async ({ page }) => {
      await login(page, users.admin, "/admin/maintenance");

      // reapStaleStreams only reads and reconciles state for streams stuck live; the
      // fixture has none, so running it here changes nothing it would break.
      await page.getByRole("combobox", { name: "Cron job" }).selectOption("reapStaleStreams");
      await page.getByRole("button", { name: "Run" }).click();

      await expect(page.getByRole("status")).toHaveText("Job has been triggered.");
    });
  });

  test.describe("failed transcodings", () => {
    test("lists a failure with its stream, time and worker", async ({ page }) => {
      await login(page, users.admin, "/admin/maintenance");

      const row = page
        .locator("li")
        .filter({ hasText: `${transcodingFailures.kept.streamId} - ${transcodingFailures.kept.version}` })
        .filter({ hasText: transcodingFailures.kept.hostname })
        .first();
      await expect(row).toBeVisible();
    });

    test("expands to show the file path and logs", async ({ page }) => {
      await login(page, users.admin, "/admin/maintenance");

      const row = page
        .locator("li")
        .filter({ hasText: `${transcodingFailures.kept.streamId} - ${transcodingFailures.kept.version}` })
        .filter({ hasText: transcodingFailures.kept.hostname })
        .first();
      await row.getByRole("button", { name: "Expand" }).click();

      await expect(row.getByText("ffmpeg: could not open input file")).toBeVisible();
    });
  });

  test.describe("failed emails", () => {
    test("lists a failure with its recipient and attempt count", async ({ page }) => {
      await login(page, users.admin, "/admin/maintenance");

      const row = page
        .locator("li")
        .filter({ hasText: emailFailures.kept.to })
        .filter({ hasText: `${emailFailures.kept.retries} attempts` })
        .first();
      await expect(row).toBeVisible();
    });

    test("expands to show the body and errors", async ({ page }) => {
      await login(page, users.admin, "/admin/maintenance");

      const row = page
        .locator("li")
        .filter({ hasText: emailFailures.kept.to })
        .filter({ hasText: `${emailFailures.kept.retries} attempts` })
        .first();
      await row.getByRole("button", { name: "Expand" }).click();

      await expect(row.getByText("550 5.1.1 unknown recipient")).toBeVisible();
    });
  });
});

test.describe("the maintenance API", () => {
  test.describe("refuses everyone but a server administrator", () => {
    for (const account of ["studi1", "prof1"] as const) {
      test(`${account} may not list transcoding failures`, async ({ playwright }) => {
        const context = await apiAs(playwright, users[account]);
        const token = await bearerToken(context);

        const response = await context.get("/api/v2/admin/maintenance/transcoding-failures", {
          headers: { Authorization: `Bearer ${token}` },
        });

        expect(response.status()).toBe(403);
      });

      test(`${account} may not run a cron job`, async ({ playwright }) => {
        const context = await apiAs(playwright, users[account]);
        const token = await bearerToken(context);

        const response = await context.post("/api/v2/admin/maintenance/cron/run", {
          headers: { Authorization: `Bearer ${token}` },
          data: { job: "fetchCourses" },
        });

        expect(response.status()).toBe(403);
      });
    }

    test("an anonymous caller may not list failed emails", async ({ playwright }) => {
      const context = await apiAs(playwright);

      expect((await context.get("/api/v2/admin/maintenance/email-failures")).status()).toBe(401);
    });
  });

  test("refuses a cron job name the scheduler does not know", async ({ playwright }) => {
    const context = await apiAs(playwright, users.admin);
    const token = await bearerToken(context);

    const response = await context.post("/api/v2/admin/maintenance/cron/run", {
      headers: { Authorization: `Bearer ${token}` },
      data: { job: "not-a-real-job" },
    });

    expect(response.status()).toBe(404);
  });
});

/**
 * Last in the file: it consumes the `consumed` fixtures, which nothing can recreate.
 * Acceptable only because `make test_e2e` reloads the dump every run.
 */
test.describe("dismissing a failure", () => {
  test("removes a transcoding failure and leaves the other alone", async ({ page }) => {
    await login(page, users.admin, "/admin/maintenance");

    const consumedRow = page
      .locator("li")
      .filter({ hasText: `${transcodingFailures.consumed.streamId} - ${transcodingFailures.consumed.version}` })
      .filter({ hasText: transcodingFailures.consumed.hostname })
      .first();
    await expect(consumedRow).toBeVisible();

    page.once("dialog", (dialog) => dialog.accept());
    await consumedRow.getByRole("button", { name: /Dismiss failure for stream/ }).click();

    await expect(consumedRow).toHaveCount(0);
    await expect(
      page
        .locator("li")
        .filter({ hasText: `${transcodingFailures.kept.streamId} - ${transcodingFailures.kept.version}` })
        .filter({ hasText: transcodingFailures.kept.hostname })
        .first(),
    ).toBeVisible();
  });

  test("removes a failed email and leaves the other alone", async ({ page }) => {
    await login(page, users.admin, "/admin/maintenance");

    const consumedRow = page
      .locator("li")
      .filter({ hasText: emailFailures.consumed.to })
      .filter({ hasText: `${emailFailures.consumed.retries} attempts` })
      .first();
    await expect(consumedRow).toBeVisible();

    page.once("dialog", (dialog) => dialog.accept());
    await consumedRow.getByRole("button", { name: `Dismiss failed email to ${emailFailures.consumed.to}` }).click();

    await expect(consumedRow).toHaveCount(0);
    await expect(
      page
        .locator("li")
        .filter({ hasText: emailFailures.kept.to })
        .filter({ hasText: `${emailFailures.kept.retries} attempts` })
        .first(),
    ).toBeVisible();
  });
});
