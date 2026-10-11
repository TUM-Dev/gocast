import { randomBytes } from "node:crypto";

import { expect, test } from "@playwright/test";

import { baseURL } from "../playwright.config";
import { apiAs, bearerToken, login } from "./helpers";
import { users } from "./seed";

test("authorizes an application, cancels, and revokes access in course settings", async ({
  page,
  playwright,
}) => {
  const admin = await apiAs(playwright, users.admin);
  const token = await bearerToken(admin);
  const name = `Course portal ${Date.now()}`;
  const response = await admin.post("/api/v2/admin/integrations", {
    headers: { Authorization: `Bearer ${token}` },
    data: { name, returnUrl: new URL("/about", baseURL).href },
  });
  expect(response.status()).toBe(200);
  const { id } = await response.json();
  const state = randomBytes(32).toString("base64url");
  const authorization = `/integration/authorize/${id}?state=${state}`;
  await login(page, users.prof1, authorization);
  await page.getByLabel("Course", { exact: true }).selectOption("1");
  await page
    .getByRole("button", { name: "Authorize access", exact: true })
    .click();
  await expect(page).toHaveURL(/\/about\?/);
  expect(new URL(page.url()).searchParams.get("state")).toBe(state);
  expect(new URL(page.url()).searchParams.get("code")).toMatch(
    /^[A-Za-z0-9_-]{43}$/,
  );
  await page.goto("/admin/courses/1/settings");
  const applications = page.getByRole("table", {
    name: "Authorized applications",
  });
  const row = applications.getByRole("row").filter({ hasText: name });
  await expect(row).toBeVisible();
  await page.reload();
  await expect(row).toBeVisible();
  page.once("dialog", (dialog) => dialog.dismiss());
  await row.getByRole("button", { name: `Revoke access for ${name}` }).click();
  await expect(row).toBeVisible();
  page.once("dialog", (dialog) => dialog.accept());
  await row.getByRole("button", { name: `Revoke access for ${name}` }).click();
  await expect(row).toHaveCount(0);
  await page.goto(authorization);
  await page.getByRole("button", { name: "Cancel", exact: true }).click();
  await expect(page).toHaveURL(/error=access_denied/);
  await page.goto("/admin/courses/1/settings");
  await expect(row).toHaveCount(0);
  await page.context().clearCookies();
  await login(page, users.studi1, authorization);
  await expect(
    page.getByRole("button", { name: "Authorize access", exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByText("You do not administer any courses that can be connected."),
  ).toBeVisible();
  expect(
    (await page.request.get("/api/v2/courses/1/integrations")).status(),
  ).toBe(404);
  await admin.dispose();
});
