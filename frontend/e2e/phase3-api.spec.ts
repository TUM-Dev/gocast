import { expect, test } from "@playwright/test";

import { apiAs } from "./helpers";
import { users } from "./seed";

/**
 * The v2 RPCs behind the search, course-token, set-password and onboarding pages.
 * All serve anonymous callers; what they refuse is the interesting part here, since
 * the dev stack has no Meilisearch, the fixture's courses carry no token, no reset
 * key is outstanding, and users exist. Nothing below changes the fixture.
 */

test.describe("the search API", () => {
  test("answers 503 while Meilisearch is unreachable", async ({ playwright }) => {
    const anonymous = await apiAs(playwright);
    expect((await anonymous.get("/api/v2/search?query=bier")).status()).toBe(503);
  });

  test("refuses a missing query and a malformed semester", async ({ playwright }) => {
    const anonymous = await apiAs(playwright);
    expect((await anonymous.get("/api/v2/search?query=%20")).status()).toBe(400);
    expect((await anonymous.get("/api/v2/search?query=bier&semesters=2022X")).status()).toBe(400);
  });

  test("tells an anonymous caller a non-public course does not exist", async ({ playwright }) => {
    const anonymous = await apiAs(playwright);
    // "geheim" is the hidden course; a slug nobody has answers the same.
    expect((await anonymous.get("/api/v2/search?query=bier&courses=nonexistent2022S")).status()).toBe(404);
  });

  test("lets a student search inside a course they may watch, up to Meilisearch", async ({ playwright }) => {
    const studi1 = await apiAs(playwright, users.studi1);
    expect((await studi1.get("/api/v2/search?query=bier&courses=brauereiwesen2022S")).status()).toBe(503);
  });
});

test.describe("the course token API", () => {
  test("answers 404 for a token no course has, and 400 for none", async ({ playwright }) => {
    const anonymous = await apiAs(playwright);
    for (const path of ["/api/v2/courses/by-token", "/api/v2/courses/by-token/opt-in", "/api/v2/courses/by-token/opt-out"]) {
      expect((await anonymous.post(path, { data: { token: "not-a-token" } })).status()).toBe(404);
      expect((await anonymous.post(path, { data: { token: "" } })).status()).toBe(400);
    }
  });
});

test.describe("the password reset API", () => {
  test("answers 404 for a key that was never issued", async ({ playwright }) => {
    const anonymous = await apiAs(playwright);
    expect((await anonymous.get("/api/v2/users/password-reset/never-issued")).status()).toBe(404);
    expect(
      (await anonymous.post("/api/v2/users/password-reset/never-issued", { data: { password: "correct horse" } })).status(),
    ).toBe(404);
  });
});

test.describe("the onboarding API", () => {
  test("refuses to create a first user while users exist", async ({ playwright }) => {
    const anonymous = await apiAs(playwright);
    const response = await anonymous.post("/api/v2/users/first", {
      data: { name: "Root", email: "root@example.com", password: "correct horse" },
    });
    expect(response.status()).toBe(403);
  });
});
