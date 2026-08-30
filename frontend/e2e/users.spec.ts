import { expect, test, type Page } from "@playwright/test";

import { apiAs, bearerToken, login } from "./helpers";
import { users } from "./seed";

/**
 * User management: the first page behind users.manage rather than server.administer,
 * so also where the two are shown to be enforced apart.
 *
 * Idle it lists staff unmasked; searching it reaches every account and masks them.
 */

/** The names in the table, in order. */
async function rows(page: Page): Promise<string[]> {
  await expect(page.getByRole("table")).toBeVisible();
  return page.locator("tbody tr td:first-child").allInnerTexts();
}

test.describe("who may open the page", () => {
  test("a user manager is served the SPA shell", async ({ page }) => {
    await login(page, users.admin);

    const response = await page.goto("/admin/users");
    expect((await response?.text()) ?? "").toContain("/spa-assets/");
  });

  test("a lecturer is refused, holding no users.manage", async ({ page }) => {
    // Both permissions belong to admins today, so this keeps them distinct.
    await login(page, users.prof1);

    expect((await page.goto("/admin/users"))?.status()).toBe(403);
  });

  test("a student is refused", async ({ page }) => {
    await login(page, users.studi1);

    expect((await page.goto("/admin/users"))?.status()).toBe(403);
  });
});

test.describe("the two listings", () => {
  test("lists the administrators and lecturers, and nobody else", async ({ page }) => {
    await login(page, users.admin, "/admin/users");

    const names = await rows(page);
    expect(names).toContain(users.admin.name);
    expect(names).toContain(users.prof1.name);
    expect(names).toContain(users.prof2.name);
    // Students are reachable by searching, never by idling on the page.
    expect(names).not.toContain(users.studi1.name);
  });

  test("shows staff contact details unmasked", async ({ page }) => {
    await login(page, users.admin, "/admin/users");

    const row = page.getByRole("row", { name: new RegExp(users.prof1.name) });
    await expect(row).toContainText(users.prof1.username);
  });

  test("searching reaches students, whose details arrive masked", async ({ page }) => {
    await login(page, users.admin, "/admin/users");

    await page.getByLabel("Search", { exact: true }).fill("Stephanie");

    const row = page.getByRole("row", { name: new RegExp(users.studi1.name) });
    await expect(row).toBeVisible();
    // The seeded logins have no `@`, so masking drops them. Either way the stored
    // value must not appear, which the staff list above does show for its own rows.
    await expect(row).not.toContainText(users.studi1.username);
  });

  test("says which of the two lists is on screen", async ({ page }) => {
    // Mistaking a search for the staff list means misreading who exists.
    await login(page, users.admin, "/admin/users");
    await expect(page.getByText("Administrators and lecturers")).toBeVisible();

    await page.getByLabel("Search", { exact: true }).fill("Stephanie");
    await expect(page.getByText(/Emails and logins are masked/)).toBeVisible();
  });

  test("does not search until there is enough to search for", async ({ page }) => {
    await login(page, users.admin, "/admin/users");

    const searches: string[] = [];
    page.on("request", (request) => {
      if (request.url().includes("/admin/users/search")) searches.push(request.url());
    });

    await page.getByLabel("Search", { exact: true }).fill("St");
    await expect(page.getByText("Administrators and lecturers")).toBeVisible();
    expect(searches).toHaveLength(0);
  });

  test("a role on its own is a search", async ({ page }) => {
    await login(page, users.admin, "/admin/users");

    await page.getByLabel("Role", { exact: true }).selectOption({ label: "Student" });

    // The staff table is already on screen, so the results have to be waited for by
    // something that changes — reading the rows straight away reads the old ones.
    await expect(page.getByText(/Emails and logins are masked/)).toBeVisible();

    const names = await rows(page);
    expect(names).toContain(users.studi1.name);
    expect(names).not.toContain(users.admin.name);
  });
});

test.describe("the API", () => {
  const paths = ["/api/v2/admin/users", "/api/v2/admin/users/search?query=prof"];

  for (const account of ["studi1", "prof1"] as const) {
    for (const path of paths) {
      test(`${account} may not GET ${path}`, async ({ playwright }) => {
        const context = await apiAs(playwright, users[account]);
        const token = await bearerToken(context);

        const response = await context.get(path, {
          headers: { Authorization: `Bearer ${token}` },
        });

        expect(response.status()).toBe(403);
      });
    }

    test(`${account} may not create an account`, async ({ playwright }) => {
      const context = await apiAs(playwright, users[account]);
      const token = await bearerToken(context);

      const response = await context.post("/api/v2/admin/users", {
        headers: { Authorization: `Bearer ${token}` },
        data: { name: "Sneaky", email: "sneaky@example.org" },
      });

      expect(response.status()).toBe(403);
    });

    test(`${account} may not change a role`, async ({ playwright }) => {
      // The one that would matter most: granting yourself users.manage.
      const context = await apiAs(playwright, users[account]);
      const token = await bearerToken(context);

      const response = await context.patch("/api/v2/admin/users/4/role", {
        headers: { Authorization: `Bearer ${token}` },
        data: { role: 1 },
      });

      expect(response.status()).toBe(403);
    });

    test(`${account} may not delete an account`, async ({ playwright }) => {
      const context = await apiAs(playwright, users[account]);
      const token = await bearerToken(context);

      const response = await context.delete("/api/v2/admin/users/4", {
        headers: { Authorization: `Bearer ${token}` },
      });

      expect(response.status()).toBe(403);
    });
  }

  test("an anonymous caller may not list accounts", async ({ playwright }) => {
    const context = await apiAs(playwright);

    expect((await context.get("/api/v2/admin/users")).status()).toBe(401);
  });

  test("a short search is refused rather than listing everyone", async ({ playwright }) => {
    const context = await apiAs(playwright, users.admin);
    const token = await bearerToken(context);

    const response = await context.get("/api/v2/admin/users/search?query=ab", {
      headers: { Authorization: `Bearer ${token}` },
    });

    expect(response.status()).toBe(400);
  });

  test("an administrator may not be deleted", async ({ playwright }) => {
    // Otherwise this page removes everyone able to undo it.
    const context = await apiAs(playwright, users.admin);
    const token = await bearerToken(context);

    const response = await context.delete("/api/v2/admin/users/1", {
      headers: { Authorization: `Bearer ${token}` },
    });

    expect(response.status()).toBe(400);
  });

  test("an administrator may not change their own role", async ({ playwright }) => {
    // Removes the permission that allowed it, irreversibly on a single-admin server.
    const context = await apiAs(playwright, users.admin);
    const token = await bearerToken(context);

    const response = await context.patch("/api/v2/admin/users/1/role", {
      headers: { Authorization: `Bearer ${token}` },
      data: { role: 4 },
    });

    expect(response.status()).toBe(400);
  });
});

/* The tests below write, so they come last. `make test_e2e` reloads before each run. */

test.describe("changing what an account may do", () => {
  /**
   * Creates an account through the form and returns once it is on screen.
   *
   * These tests act on accounts they made themselves: the fixture reloads once per
   * run, not per file, and visibility.spec.ts asserts against the seeded ones.
   */
  async function createAccount(page: Page, name: string, email: string): Promise<void> {
    await page.getByLabel("Name", { exact: true }).fill(name);
    await page.getByLabel("Email", { exact: true }).fill(email);
    await page.getByRole("button", { name: "Create" }).click();
    await expect(page.getByRole("status")).toContainText("emailed an invitation");
    await expect(page.getByRole("row", { name: new RegExp(name) })).toBeVisible();
  }

  test("offers no delete for an administrator", async ({ page }) => {
    await login(page, users.admin, "/admin/users");

    const row = page.getByRole("row", { name: new RegExp(users.admin.name) });
    await expect(row.getByRole("button", { name: `Delete ${users.admin.name}` })).toHaveCount(0);
    // The lecturer beside them has one, so this is the rule and not an empty column.
    const lecturer = page.getByRole("row", { name: new RegExp(users.prof2.name) });
    const deleteLecturer = lecturer.getByRole("button", { name: `Delete ${users.prof2.name}` });
    await expect(deleteLecturer).toBeVisible();
  });

  test("creates an account and masks its address in search", async ({ page }) => {
    // In one flow, because no seeded login can show what a masked address looks like.
    await login(page, users.admin, "/admin/users");

    await createAccount(page, "Nina Neu", "nina.neu@example.org");

    await page.getByLabel("Search", { exact: true }).fill("Nina");
    const row = page.getByRole("row", { name: /Nina Neu/ });
    await expect(row).toContainText("n*******@example.org");
    await expect(row).not.toContainText("nina.neu@example.org");
  });

  test("refuses a second account for the same address", async ({ page }) => {
    // The email column is unique; this used to surface as a constraint violation.
    await login(page, users.admin, "/admin/users");

    await createAccount(page, "First Claim", "taken@example.org");

    await page.getByLabel("Name", { exact: true }).fill("Second Claim");
    await page.getByLabel("Email", { exact: true }).fill("taken@example.org");
    await page.getByRole("button", { name: "Create" }).click();

    await expect(page.getByRole("alert")).toContainText("already exists");
  });

  test("a role change moves an account in and out of the staff list", async ({ page }) => {
    await login(page, users.admin, "/admin/users");

    // Created accounts are lecturers, so they start in the staff list.
    await createAccount(page, "Rolle Wechsel", "rolle.wechsel@example.org");
    const staffRow = page.getByRole("row", { name: /Rolle Wechsel/ });

    await staffRow.getByRole("combobox").selectOption({ label: "Student" });
    await expect(page.getByRole("status")).toContainText("is now student");
    // Students are not staff, so the row leaves the list it is standing in.
    await expect(page.getByRole("row", { name: /Rolle Wechsel/ })).toHaveCount(0);

    // And comes back on the way up, which is what the page is for.
    await page.getByLabel("Search", { exact: true }).fill("Rolle");
    await page.getByRole("row", { name: /Rolle Wechsel/ }).getByRole("combobox")
      .selectOption({ label: "Lecturer" });
    await expect(page.getByRole("status")).toContainText("is now lecturer");

    await page.getByLabel("Search", { exact: true }).fill("");
    await expect(page.getByText("Administrators and lecturers")).toBeVisible();
    await expect(page.getByRole("row", { name: /Rolle Wechsel/ })).toBeVisible();
  });

  test("deletes an account", async ({ page }) => {
    await login(page, users.admin, "/admin/users");

    await createAccount(page, "Weg Damit", "weg.damit@example.org");
    const row = page.getByRole("row", { name: /Weg Damit/ });

    page.once("dialog", (dialog) => dialog.accept());
    await row.getByRole("button", { name: "Delete Weg Damit" }).click();

    await expect(row).toHaveCount(0);
  });

  test("continuing as another account replaces the session", async ({ page }) => {
    // Still the v1 endpoint: it creates a session, which the SPA does not manage.
    await login(page, users.admin, "/admin/users");

    await page.getByLabel("Search", { exact: true }).fill("Stephanie");
    const row = page.getByRole("row", { name: new RegExp(users.studi1.name) });

    page.once("dialog", (dialog) => dialog.accept());
    await row.getByRole("button", { name: `Continue as ${users.studi1.name}` }).click();

    await expect(page).toHaveURL("/");

    const me = await page.evaluate(async () => {
      const token = await (await fetch("/api/v2/auth/token", { method: "POST" })).json();
      const res = await fetch("/api/v2/users/me", {
        headers: { Authorization: `Bearer ${token.access_token}` },
      });
      return res.json();
    });

    expect(me.user.name).toBe(users.studi1.name);
  });
});
