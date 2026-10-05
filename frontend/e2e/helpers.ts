import { readFileSync } from "node:fs";
import { join } from "node:path";

import { expect, type APIRequest, type APIRequestContext, type Page } from "@playwright/test";

import { authStateDir, baseURL } from "../playwright.config";
import { password, users, type SeedUser } from "./seed";

export { password };

/** The cookie the Go server sets on a successful login. */
export const SESSION_COOKIE = "jwt";

/** Where auth.setup.ts leaves a user's signed-in session for the run. */
export const authStatePath = (user: SeedUser): string => join(authStateDir, `${user.username}.json`);

interface StoredCookie {
  name: string;
  value: string;
  domain: string;
  path: string;
  expires: number;
  httpOnly: boolean;
  secure: boolean;
  sameSite: "Strict" | "Lax" | "None";
}

const sessions = new Map<string, StoredCookie[]>();

/**
 * The session cookies auth.setup.ts stored for `user`, read once per worker.
 *
 * Playwright's storageState file, so the same file serves `apiAs()` directly.
 */
function sessionCookies(user: SeedUser): StoredCookie[] {
  let cookies = sessions.get(user.username);
  if (!cookies) {
    try {
      cookies = (JSON.parse(readFileSync(authStatePath(user), "utf8")) as { cookies: StoredCookie[] }).cookies;
    } catch (cause) {
      throw new Error(
        `no stored session for ${user.username}: the setup project in playwright.config.ts has to run first`,
        { cause },
      );
    }
    sessions.set(user.username, cookies);
  }
  return cookies;
}

/**
 * Makes the page's browser `user`, then opens `to`.
 *
 * The session is the one auth.setup.ts obtained through the real form; putting its
 * cookie in the jar is what the browser would have done itself after the redirect,
 * minus the three navigations. The form itself, the redirect it follows and the
 * cookie it sets are covered by login.spec.ts, which signs in through `loginViaForm`
 * because there the act of signing in is the point.
 *
 * The user is named rather than defaulted wherever what they can see is the point.
 * `users.studi1` is the default for the tests where it is not: an ordinary student
 * with no administrative rights anywhere.
 */
export async function login(page: Page, user: SeedUser = users.studi1, to = "/"): Promise<void> {
  await page.context().addCookies(sessionCookies(user));
  await page.goto(to);
}

/**
 * Signs in the way a person does: through the real form, letting the browser follow
 * the redirect the server sends. Nothing here reaches into the API, so a break in the
 * session handling shows up as a failing test rather than being papered over.
 *
 * Navigates explicitly afterwards rather than relying on the post-login redirect: that
 * redirect is itself under test in login.spec.ts. Compare the full URL, not just the
 * path, so a redirect to the same page but a different semester still lands on the
 * intended query string.
 */
export async function loginViaForm(page: Page, user: SeedUser = users.studi1, to = "/"): Promise<void> {
  await page.goto("/login");

  await page.getByLabel("Username").fill(user.username);
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Login" }).click();
  await expect(page).not.toHaveURL(/\/login/);

  const here = new URL(page.url());
  const target = new URL(to, baseURL);
  if (here.pathname !== target.pathname || here.search !== target.search) {
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
 * are allowed would otherwise have them overwrite each other. The jar starts out as
 * the one auth.setup.ts saved, so no request is made here.
 */
export async function apiAs(
  playwright: { request: APIRequest },
  user?: SeedUser,
): Promise<APIRequestContext> {
  return playwright.request.newContext({
    baseURL,
    storageState: user ? authStatePath(user) : undefined,
  });
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
