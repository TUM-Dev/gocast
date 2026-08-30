import { expect, test } from "@playwright/test";

import { login } from "./helpers";
import { users } from "./seed";

/**
 * The side-by-side wiring itself: which frontend answers a given path, and the bridge
 * that lets a page served by one use the API built for the other. web/spa_test.go
 * covers the Go half in isolation; these assert it end to end, against the binary as
 * it actually serves.
 */

test.describe("route ownership", () => {
  test("migrated paths are served the SPA shell", async ({ page }) => {
    const paths = [
      "/login",
      "/settings",
      "/",
      "/courses/mine",
      "/courses/public",
      "/course/2024/W/some-slug",
    ];
    for (const path of paths) {
      const response = await page.goto(path);
      const body = (await response?.text()) ?? "";
      expect(body, `${path} should be the SPA shell`).toContain("/spa-assets/");
      // The shell must never be cached: it is the same bytes for every route, and a
      // cached copy would outlive a route being moved back to its template.
      expect(response?.headers()["cache-control"]).toContain("no-store");
    }
  });

  test("paths that have not moved are still rendered by Go", async ({ page }) => {
    await login(page, users.studi1, "/settings");

    const response = await page.goto("/about");
    const body = (await response?.text()) ?? "";
    expect(body).not.toContain("/spa-assets/");
  });

  test("the SPA hands an unknown path back to the server", async ({ page }) => {
    await login(page, users.studi1, "/settings");

    // Client-side navigation to a path the SPA router does not own must become a real
    // navigation, or the user lands on a blank shell. Not /semester/:year/:term, which
    // redirects into the start page and so now lands on the shell after all.
    await page.evaluate(() => window.history.pushState({}, "", "/settings"));
    const response = await page.goto("/privacy");
    expect((await response?.text()) ?? "").not.toContain("/spa-assets/");
  });
});

test.describe("the cookie-to-bearer bridge", () => {
  test("refuses to mint a token without a session", async ({ request }) => {
    const response = await request.post("/api/v2/auth/token");
    expect(response.status()).toBe(401);
  });

  test("mints a short-lived token for a signed-in browser", async ({ page }) => {
    await login(page, users.studi1, "/settings");
    // Settled before evaluating: the page is still fetching when it loads, and a
    // navigation while the expression runs destroys the context it runs in.
    await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();

    const token = await page.evaluate(async () => {
      const res = await fetch("/api/v2/auth/token", { method: "POST" });
      return res.ok ? await res.json() : null;
    });

    expect(token).not.toBeNull();
    expect(token.token_type).toBe("Bearer");
    expect(typeof token.access_token).toBe("string");
    expect(token.expires_in).toBeGreaterThan(0);
  });

  test("the API is called with a bearer token, not the cookie alone", async ({ page }) => {
    // Only the endpoints behind authentication. /login-options and the token endpoint
    // itself are deliberately called without one — the login page has no session to
    // present, and sending the bearer there would be the actual mistake.
    const authorized: string[] = [];
    page.on("request", (request) => {
      if (request.url().includes("/api/v2/users/")) {
        authorized.push(request.headers()["authorization"] ?? "");
      }
    });

    await login(page, users.studi1, "/settings");
    await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();

    expect(authorized.length).toBeGreaterThan(0);
    for (const header of authorized) {
      expect(header).toMatch(/^Bearer .+/);
    }
  });

  test("a token is minted once and reused across navigations", async ({ page }) => {
    // The token lives in memory, so a full page load reasonably mints a new one; what
    // must not happen is one request per API call. Counting starts after signing in,
    // because the page login lands on is itself a full load with its own token.
    await login(page);

    let mints = 0;
    page.on("request", (request) => {
      if (request.url().includes("/api/v2/auth/token")) mints += 1;
    });

    await page.goto("/settings");
    await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();

    // A write as well as the initial read, so the count covers both directions.
    const autoSkip = page.getByRole("checkbox", { name: /Skip the silence/ });
    await autoSkip.setChecked(!(await autoSkip.isChecked()));
    await expect(page.getByRole("status")).toBeVisible();

    expect(mints).toBe(1);
  });
});

/** The odd spellings moved to kebab-case; bookmarks outlive a rename. */
test.describe("renamed admin paths", () => {
  const renamed = [
    ["/admin/lectureHalls", "/admin/lecture-halls"],
    ["/admin/lectureHalls/new", "/admin/lecture-halls/new"],
    ["/admin/infopages", "/admin/info-pages"],
  ];

  for (const [from, to] of renamed) {
    test(`${from} redirects to ${to}`, async ({ page }) => {
      await login(page, users.admin);

      await page.goto(from);
      await expect(page).toHaveURL(to);
    });
  }

  test("the redirect answers before checking who is asking", async ({ request }) => {
    // Outside the permission groups, so the old path points a visitor at the new one
    // instead of 403ing. Indistinguishable in the routing table, hence asserted.
    const response = await request.get("/admin/lectureHalls", { maxRedirects: 0 });

    expect(response.status()).toBe(301);
    expect(response.headers()["location"]).toBe("/admin/lecture-halls");
  });
});
