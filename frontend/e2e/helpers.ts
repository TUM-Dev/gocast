import { expect, type APIRequest, type APIRequestContext, type Page } from "@playwright/test";

import { baseURL } from "../playwright.config";
import { password, users, type SeedUser } from "./seed";

export { password };

/** The cookie the Go server sets on a successful login. */
export const SESSION_COOKIE = "jwt";

/**
 * Signs in the way a person does: through the real form, letting the browser follow
 * the redirect the server sends. Nothing here reaches into the API, so a break in the
 * session handling shows up as a failing test rather than being papered over.
 *
 * The user is named rather than defaulted wherever what they can see is the point.
 * `users.studi1` is the default for the tests where it is not: an ordinary student
 * with no administrative rights anywhere.
 */
export async function login(page: Page, user: SeedUser = users.studi1, to = "/"): Promise<void> {
  await page.goto("/login");

  await page.getByLabel("Username").fill(user.username);
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

/**
 * An API context calling as `user`, or anonymously when none is given.
 *
 * A context of its own per caller, rather than the shared `request` fixture, because
 * the session lives in that context's cookie jar: one test comparing what two callers
 * are allowed would otherwise have them overwrite each other.
 */
export async function apiAs(
  playwright: { request: APIRequest },
  user?: SeedUser,
): Promise<APIRequestContext> {
  const context = await playwright.request.newContext({ baseURL });

  if (user) {
    // The real form post, as in login(): the server sets the session cookie, and the
    // context keeps it for everything called through it afterwards.
    const response = await context.post("/login", {
      form: { username: user.username, password },
      maxRedirects: 0,
    });
    expect(response.status(), `could not sign in as ${user.username}`).toBe(302);
  }

  return context;
}

/**
 * The bearer token a context's session is worth, as the SPA obtains it.
 *
 * Returns null when the context has no session: refusing to mint a token is how the
 * server says nobody is signed in, which is an answer rather than a failure.
 */
export async function bearerToken(context: APIRequestContext): Promise<string | null> {
  const response = await context.post("/api/v2/auth/token");
  if (!response.ok()) {
    return null;
  }
  return (await response.json()).access_token;
}
