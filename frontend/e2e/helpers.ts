import { expect, type Page } from "@playwright/test";

import { password, username } from "../playwright.config";

export { password, username };

/** The cookie the Go server sets on a successful login. */
export const SESSION_COOKIE = "jwt";

/**
 * Signs in the way a person does: through the real form, letting the browser follow
 * the redirect the server sends. Nothing here reaches into the API, so a break in the
 * session handling shows up as a failing test rather than being papered over.
 */
export async function login(page: Page, to = "/"): Promise<void> {
  await page.goto("/login");

  await page.getByLabel("Username").fill(username);
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Login" }).click();
  await expect(page).not.toHaveURL(/\/login/);

  // Navigate explicitly rather than relying on the post-login redirect: that redirect
  // is itself under test in login.spec.ts, and a helper every other test depends on
  // should not fail for the same reason.
  if (new URL(page.url()).pathname !== to) {
    await page.goto(to);
  }
}

/** Reads the current session cookie, or undefined when there is none. */
export async function sessionCookie(page: Page): Promise<string | undefined> {
  const cookies = await page.context().cookies();
  return cookies.find((cookie) => cookie.name === SESSION_COOKIE)?.value;
}
