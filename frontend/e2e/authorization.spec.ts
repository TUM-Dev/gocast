import { expect, test, type APIRequestContext } from "@playwright/test";

import { apiAs, bearerToken } from "./helpers";
import { users, type SeedUser } from "./seed";

/**
 * What the v2 API lets each caller reach.
 *
 * apiv2/server/policy_test.go covers the interceptor in isolation, with the policy
 * table in front of it and a mocked database behind. These assert the same decisions
 * end to end: through the gateway, against real sessions of the seeded accounts, on
 * the paths the proto's http annotations actually publish.
 *
 * The distinction that matters is between a policy and a handler. A policy decides
 * whether a caller reaches the endpoint at all — the subject here. What a handler
 * then shows them is the subject of visibility.spec.ts.
 */

/** GET as `token`'s owner, or anonymously when it is null. */
async function get(
  context: APIRequestContext,
  path: string,
  token: string | null,
): Promise<number> {
  const response = await context.get(`/api/v2${path}`, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  });
  return response.status();
}

/**
 * Endpoints answering before anyone has signed in — the ones driving the login page
 * and the public listings. Each is listed in TestOnlyExpectedMethodsArePublic too, so
 * that widening access is an edit in two places that disagree loudly.
 */
const publicPaths = [
  "/status",
  "/config",
  "/semesters",
  "/login-options",
  "/server-notifications",
  "/courses",
  "/courses/live",
];

/** Endpoints acting on one account, which an anonymous caller has none of. */
const authenticatedPaths = [
  "/users/me",
  "/users/export",
  "/notifications",
  "/courses/enrolled",
  "/courses/pinned",
  "/bookmarks",
];

test.describe("endpoints that answer anonymous callers", () => {
  for (const path of publicPaths) {
    test(`${path} answers without credentials`, async ({ playwright }) => {
      const context = await apiAs(playwright);

      // No Authorization header at all, which is what a visitor's browser sends
      // before it has a session to trade for a token.
      expect(await get(context, path, null)).toBe(200);
    });
  }
});

test.describe("endpoints that act on an account", () => {
  for (const path of authenticatedPaths) {
    test(`${path} refuses an anonymous caller`, async ({ playwright }) => {
      const context = await apiAs(playwright);

      expect(await get(context, path, null)).toBe(401);
    });
  }

  for (const path of authenticatedPaths) {
    test(`${path} answers an ordinary student`, async ({ playwright }) => {
      // A student holds no permission whatsoever, so anything they reach is reached
      // by being signed in and nothing else — which is what `authenticated` means.
      const context = await apiAs(playwright, users.studi1);
      const token = await bearerToken(context);
      expect(token).not.toBeNull();

      expect(await get(context, path, token)).toBe(200);
    });
  }
});

test.describe("credentials", () => {
  test("a session that does not exist mints no token", async ({ playwright }) => {
    const context = await apiAs(playwright);

    expect(await bearerToken(context)).toBeNull();
  });

  test("every seeded account can obtain a token", async ({ playwright }) => {
    // Roles differ in what they may do, never in whether they may authenticate. A
    // role that could not get a token would look like a permission problem.
    for (const user of Object.values(users) as SeedUser[]) {
      const context = await apiAs(playwright, user);

      const token = await bearerToken(context);
      expect(token, `${user.username} could not obtain a token`).not.toBeNull();
    }
  });

  test("a rejected token is refused rather than treated as anonymous", async ({ playwright }) => {
    // The difference between "nobody is signed in" and "these credentials are no
    // good". An endpoint serving anonymous callers must not quietly downgrade the
    // second to the first: the client needs the 401 to know to refresh.
    const context = await apiAs(playwright);

    expect(await get(context, "/courses", "not-a-real-token")).toBe(401);
  });
});
