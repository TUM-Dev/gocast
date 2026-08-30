import { expect, test, type APIRequestContext } from "@playwright/test";

import { apiAs, bearerToken } from "./helpers";
import { permissionsByRole, users, type SeedUser } from "./seed";

/**
 * What the v2 API lets each caller reach, end to end. policy_test.go covers the
 * interceptor in isolation; visibility.spec.ts covers what a handler then shows.
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

/** Also listed in TestOnlyExpectedMethodsArePublic, which disagrees loudly. */
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

      // No Authorization header, as a visitor's browser sends.
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
      // A student holds no permission, so this is `authenticated` and nothing more.
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
    // Roles differ in what they may do, never in whether they may authenticate.
    for (const user of Object.values(users) as SeedUser[]) {
      const context = await apiAs(playwright, user);

      const token = await bearerToken(context);
      expect(token, `${user.username} could not obtain a token`).not.toBeNull();
    }
  });

  test("a rejected token is refused rather than treated as anonymous", async ({ playwright }) => {
    // Not the same as "nobody is signed in": the client needs the 401 to refresh.
    const context = await apiAs(playwright);

    expect(await get(context, "/courses", "not-a-real-token")).toBe(401);
  });
});

/**
 * The capabilities GET /users/me reports. The SPA offers controls from these rather
 * than from `role`, so a role gaining or losing one has to show up here.
 */
test.describe("the permissions a caller is told they hold", () => {
  for (const [key, user] of Object.entries(users)) {
    test(`${key} is sent exactly the permissions of role ${user.role}`, async ({ playwright }) => {
      const context = await apiAs(playwright, user);
      const token = await bearerToken(context);

      const response = await context.get("/api/v2/users/me", {
        headers: { Authorization: `Bearer ${token}` },
      });
      expect(response.status()).toBe(200);

      // protojson omits an empty repeated field, so absent means none.
      const held: string[] = (await response.json()).user.permissions ?? [];

      expect(held.sort()).toEqual([...permissionsByRole[user.role]].sort());
    });
  }

  test("a student is told they may do nothing at all", async ({ playwright }) => {
    // The assertion above passes trivially if the field stops being sent at all.
    const context = await apiAs(playwright, users.studi1);
    const token = await bearerToken(context);

    const response = await context.get("/api/v2/users/me", {
      headers: { Authorization: `Bearer ${token}` },
    });
    const body = await response.json();

    expect(body.user.permissions ?? []).toEqual([]);
    // The admin holds five, so the field is not simply missing from every response.
    const adminContext = await apiAs(playwright, users.admin);
    const adminToken = await bearerToken(adminContext);
    const adminResponse = await adminContext.get("/api/v2/users/me", {
      headers: { Authorization: `Bearer ${adminToken}` },
    });

    expect((await adminResponse.json()).user.permissions).toHaveLength(5);
  });
});
