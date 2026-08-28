import { expect, test } from "@playwright/test";

import { login, password, sessionCookie } from "./helpers";
import { users } from "./seed";

/**
 * Signing in spans both frontends: the page is rendered by the SPA, the credentials
 * are posted to Go, and the session cookie and stored redirect are the server's. No
 * unit test can see that seam, which is exactly why it is worth a browser test.
 */

test.describe("login", () => {
  test("sends an anonymous visitor to the login page and back afterwards", async ({ page }) => {
    // The redirect target is recorded in a cookie by SetLoginRedirectCookie before the
    // shell is served, so it has to survive the POST and the redirect that follows.
    await page.goto("/settings");
    await expect(page).toHaveURL(/\/login/);

    await page.getByLabel("Username").fill(users.studi1.username);
    await page.getByLabel("Password").fill(password);
    await page.getByRole("button", { name: "Login" }).click();

    await expect(page).toHaveURL(/\/settings$/);
    await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();
  });

  test("reports a wrong password without starting a session", async ({ page }) => {
    await page.goto("/login");
    await page.getByLabel("Username").fill(users.studi1.username);
    await page.getByLabel("Password").fill("not-the-password");
    await page.getByRole("button", { name: "Login" }).click();

    // A failed attempt redirects rather than rendering the template inline, which
    // would replace the SPA mid-flow.
    await expect(page).toHaveURL(/\/login\?error=1$/);
    await expect(page.getByText("Couldn't log in")).toBeVisible();
    expect(await sessionCookie(page)).toBeUndefined();
  });

  test("keeps the session across a page the SPA owns and one it does not", async ({ page }) => {
    await login(page, users.studi1, "/settings");
    const cookie = await sessionCookie(page);
    expect(cookie).toBeTruthy();

    // The start page is still a template. Both frontends read the same cookie, so the
    // user stays signed in crossing between them.
    await page.goto("/");
    expect(await sessionCookie(page)).toBe(cookie);
    await expect(page.locator("body")).not.toContainText("Sign in");

    await page.goto("/settings");
    await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();
  });

  test("signing out clears the session", async ({ page }) => {
    await login(page, users.studi1, "/settings");
    await page.goto("/logout");

    expect(await sessionCookie(page)).toBeFalsy();

    await page.goto("/settings");
    await expect(page).toHaveURL(/\/login/);
  });

  test("password is never sent to the API", async ({ page }) => {
    // Credentials belong to the Go form post; if they ever start flowing through
    // /api/v2 the session handling has been reimplemented by accident.
    const apiRequests: string[] = [];
    page.on("request", (request) => {
      if (request.url().includes("/api/v2") && request.postData()?.includes(password)) {
        apiRequests.push(request.url());
      }
    });

    await login(page, users.studi1, "/settings");
    expect(apiRequests).toEqual([]);
  });
});
