import { expect, test as setup } from "@playwright/test";

import { baseURL } from "../playwright.config";
import { authStatePath } from "./helpers";
import { password, users } from "./seed";

/**
 * Signs every seeded account in once, before the suite, and keeps each session where
 * `login()` and `apiAs()` in helpers.ts pick it up.
 *
 * Signing in through the real form used to happen in every test: three navigations
 * and a full SPA load before the test could begin, four hundred times a run. The form
 * is still exercised, by login.spec.ts, which is the one place where the act of
 * signing in is what is under test. Everything else only needs to be somebody.
 *
 * Runs as its own project; playwright.config.ts makes the browser project depend on
 * it, so the files are written fresh for every run and never outlive the server they
 * were issued by.
 */
setup("sign in every seeded account", async ({ playwright }) => {
  for (const user of Object.values(users)) {
    const context = await playwright.request.newContext({ baseURL });
    const response = await context.post("/login", {
      form: { username: user.username, password },
      maxRedirects: 0,
    });
    expect(response.status(), `could not sign in as ${user.username}`).toBe(302);

    await context.storageState({ path: authStatePath(user) });
    await context.dispose();
  }
});
