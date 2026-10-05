import { expect, test } from "@playwright/test";

import { apiAs, bearerToken, login } from "./helpers";
import { users } from "./seed";

/**
 * The lecture halls admin page renders the fixture, and the endpoints behind it
 * refuse everyone without server.administer -- a hidden control being no substitute
 * for that.
 *
 * Camera preset management (the grid of images fetched from a hall's camera,
 * refreshing it, marking a default, taking a snapshot) reaches a physical camera on
 * the real server, which nothing in this environment provides -- so those tests stub
 * the v2 endpoints at the network boundary rather than exercising a real camera.
 */

test.describe("the lecture halls admin page", () => {
  test("is served the SPA shell", async ({ page }) => {
    await login(page, users.admin);

    const response = await page.goto("/admin/lecture-halls");
    expect((await response?.text()) ?? "").toContain("/spa-assets/");
  });

  test("lists the seeded hall by name", async ({ page }) => {
    await login(page, users.admin, "/admin/lecture-halls");

    await expect(
      page.locator("form").filter({ hasText: "HS001" }),
    ).toBeVisible();
  });

  test("offers the administration menu entry", async ({ page }) => {
    await login(page, users.admin, "/admin/lecture-halls");

    await expect(
      page
        .getByRole("navigation", { name: "Administration" })
        .getByRole("link", {
          name: "Lecture Halls",
        }),
    ).toBeVisible();
  });

  test("filters the list by name", async ({ page }) => {
    await login(page, users.admin, "/admin/lecture-halls");

    await page.getByPlaceholder("Filter by name").fill("nothing-matches-this");
    await expect(
      page.getByText('No lecture hall matches "nothing-matches-this".'),
    ).toBeVisible();
    await expect(page.locator("form").filter({ hasText: "HS001" })).toHaveCount(
      0,
    );
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
      test(`${account} may not list lecture halls for administration`, async ({
        playwright,
      }) => {
        const context = await apiAs(playwright, users[account]);
        const token = await bearerToken(context);

        const response = await context.get("/api/v2/admin/lecture-halls", {
          headers: { Authorization: `Bearer ${token}` },
        });

        expect(response.status()).toBe(403);
      });

      test(`${account} may not create a lecture hall`, async ({
        playwright,
      }) => {
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

      expect((await context.get("/api/v2/admin/lecture-halls")).status()).toBe(
        401,
      );
    });
  });
});

/**
 * One flow rather than three separate tests, so create/edit/delete run against the
 * hall this suite itself created -- nothing else in the fixture can be reused for all
 * three without one test depending on another's mutation surviving out of order.
 */
test.describe("managing a lecture hall end to end", () => {
  test("a hall created, edited and deleted behaves at every step", async ({
    page,
  }) => {
    // Create, on its own page just as the legacy admin page had.
    await login(page, users.admin, "/admin/lecture-halls");
    await page.getByRole("link", { name: "+ New Lecture Hall" }).click();
    await expect(page).toHaveURL("/admin/lecture-halls/new");

    // By role rather than by label: the create form marks Name required with an
    // aria-hidden asterisk inside the <label>, so the label's text is "Name *" and
    // only the accessible name is "Name". getByLabel matches the former.
    await page
      .getByRole("textbox", { name: "Name", exact: true })
      .fill("E2E_HS1");
    await page.getByLabel("Camera", { exact: true }).fill("rtsp://0.0.0.0/cam");
    await page.getByRole("button", { name: "Create" }).click();

    // Back on the list, with the new hall's form visible.
    await expect(page).toHaveURL("/admin/lecture-halls");
    const row = page.locator("form").filter({ hasText: "E2E_HS1" });
    await expect(row).toBeVisible();
    await expect(row.getByLabel("Camera", { exact: true })).toHaveValue(
      "rtsp://0.0.0.0/cam",
    );

    // Edit. The Save button stays disabled until a field actually changes.
    await expect(row.getByRole("button", { name: "Save" })).toBeDisabled();
    await row
      .getByLabel("Presentation", { exact: true })
      .fill("rtsp://0.0.0.0/pres");
    await expect(row.getByText("Unsaved changes")).toBeVisible();
    await row.getByRole("button", { name: "Save" }).click();
    await expect(row.getByText("Saved")).toBeVisible();
    await expect(row.getByRole("button", { name: "Save" })).toBeDisabled();

    // The edit survives a reload, i.e. it was actually persisted.
    await page.reload();
    const reloadedRow = page.locator("form").filter({ hasText: "E2E_HS1" });
    await expect(
      reloadedRow.getByLabel("Presentation", { exact: true }),
    ).toHaveValue("rtsp://0.0.0.0/pres");

    // Delete.
    page.once("dialog", (dialog) => dialog.accept());
    await reloadedRow.getByRole("button", { name: "Delete E2E_HS1" }).click();
    await expect(
      page.locator("form").filter({ hasText: "E2E_HS1" }),
    ).toHaveCount(0);
  });

  test("Reset discards unsaved changes without calling the API", async ({
    page,
  }) => {
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

test.describe("camera preset management", () => {
  test("a hall with no Axis camera shows no preset section", async ({
    page,
  }) => {
    await login(page, users.admin, "/admin/lecture-halls");

    // The seeded HS001 has no camera_ip, same as any VMP backed hall.
    const row = page.locator("form").filter({ hasText: "HS001" });
    await expect(row.getByText("Camera presets")).toHaveCount(0);
  });

  /**
   * A camera is unreachable from this test environment, so every request that would
   * hit one is stubbed at the network boundary. The point of this test is the page's
   * behaviour around those calls (rendering what refresh/default/snapshot return,
   * updating the right tile) not the camera integration itself, which camera_test.go
   * and lecture_hall_admin_test.go already cover server side.
   */
  test("refreshing presets, marking a default and taking a snapshot", async ({
    page,
  }) => {
    await login(page, users.admin, "/admin/lecture-halls");
    await page.getByRole("link", { name: "+ New Lecture Hall" }).click();
    await page
      .getByRole("textbox", { name: "Name", exact: true })
      .fill("E2E_HS_CAM");
    await page.getByLabel("Axis camera", { exact: true }).fill("10.0.0.9");
    await page.getByRole("button", { name: "Create" }).click();
    await expect(page).toHaveURL("/admin/lecture-halls");

    const row = page.locator("form").filter({ hasText: "E2E_HS_CAM" });
    await expect(row.getByText("Camera presets")).toBeVisible();
    await expect(
      row.getByText("No presets fetched from this camera yet."),
    ).toBeVisible();

    const idText = await row
      .locator("span")
      .filter({ hasText: /^#\d+$/ })
      .textContent();
    const hallId = Number(idText?.replace("#", ""));

    await page.route(
      `**/api/v2/admin/lecture-halls/${hallId}/presets/refresh`,
      (route) =>
        route.fulfill({
          json: {
            id: hallId,
            name: "E2E_HS_CAM",
            cameraIp: "10.0.0.9",
            cameraPresets: [
              {
                lectureHallId: hallId,
                presetId: 1,
                name: "Front",
                isDefault: false,
              },
            ],
          },
        }),
    );
    await row.getByRole("button", { name: "Reload presets" }).click();
    await expect(row.getByText("Front", { exact: true })).toBeVisible();
    await expect(row.locator("img[alt='preset preview']")).toHaveAttribute(
      "src",
      "/public/noPreset.jpg",
    );

    await page.route(
      `**/api/v2/admin/lecture-halls/${hallId}/presets/1/default`,
      (route) => route.fulfill({ json: {} }),
    );
    const setDefaultButton = row.getByRole("button", {
      name: "Set Front as default",
    });
    await setDefaultButton.click();
    await expect(setDefaultButton).not.toHaveClass(/opacity-0/);

    await page.route(
      `**/api/v2/admin/lecture-halls/${hallId}/presets/1/snapshot`,
      (route) =>
        route.fulfill({
          json: {
            lectureHallId: hallId,
            presetId: 1,
            name: "Front",
            image: "e2e-snapshot.jpg",
            isDefault: true,
          },
        }),
    );
    await row
      .getByRole("button", { name: "Take a new snapshot for Front" })
      .click();
    await expect(row.locator("img[alt='preset preview']")).toHaveAttribute(
      "src",
      "/public/e2e-snapshot.jpg",
    );

    // Cleanup.
    page.once("dialog", (dialog) => dialog.accept());
    await row.getByRole("button", { name: "Delete E2E_HS_CAM" }).click();
    await expect(
      page.locator("form").filter({ hasText: "E2E_HS_CAM" }),
    ).toHaveCount(0);
  });

  test("a failed refresh reports the error rather than silently doing nothing", async ({
    page,
  }) => {
    await login(page, users.admin, "/admin/lecture-halls");
    await page.getByRole("link", { name: "+ New Lecture Hall" }).click();
    await page
      .getByRole("textbox", { name: "Name", exact: true })
      .fill("E2E_HS_CAM_FAIL");
    await page.getByLabel("Axis camera", { exact: true }).fill("10.0.0.9");
    await page.getByRole("button", { name: "Create" }).click();
    await expect(page).toHaveURL("/admin/lecture-halls");

    const row = page.locator("form").filter({ hasText: "E2E_HS_CAM_FAIL" });
    const idText = await row
      .locator("span")
      .filter({ hasText: /^#\d+$/ })
      .textContent();
    const hallId = Number(idText?.replace("#", ""));

    await page.route(
      `**/api/v2/admin/lecture-halls/${hallId}/presets/refresh`,
      (route) =>
        route.fulfill({
          status: 503,
          json: { code: 14, message: "the camera is not answering" },
        }),
    );
    await row.getByRole("button", { name: "Reload presets" }).click();
    await expect(row.getByText("the camera is not answering")).toBeVisible();

    // Cleanup.
    page.once("dialog", (dialog) => dialog.accept());
    await row.getByRole("button", { name: "Delete E2E_HS_CAM_FAIL" }).click();
  });
});
