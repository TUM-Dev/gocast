import { expect, test } from "@playwright/test";

import { login } from "./helpers";
import { users } from "./seed";

test("registers an integration and manages its one-time API key", async ({ page }) => {
  await login(page, users.admin, "/admin/integrations");
  const name = `Integration test ${Date.now()}`;
  await page.getByLabel("Name", { exact: true }).fill(name);
  await page.getByLabel("Return URL").fill("https://portal.example/callback");
  await page.getByRole("button", { name: "Register integration", exact: true }).click();

  const key = page.getByLabel("API key", { exact: true });
  await expect(key).not.toHaveValue("");
  const original = await key.inputValue();
  const row = page.getByRole("row").filter({ hasText: name });
  await expect(row).toContainText("Active");
  await page.reload();
  await expect(row).toContainText("Active");
  await expect(key).toHaveCount(0);

  await row.getByRole("button", { name: "Regenerate key" }).click();
  await expect(key).toBeVisible();
  await expect(key).not.toHaveValue(original);
  await row.getByRole("button", { name: "Revoke key" }).click();
  await expect(key).toHaveCount(0);
  await expect(row).toContainText("Revoked");
  await page.reload();
  await expect(row).toContainText("Revoked");
  await expect(row.getByRole("button", { name: "Revoke key" })).toBeDisabled();
});

test("serves the SPA to admins and refuses students", async ({ page }) => {
  await login(page, users.admin);
  const response = await page.goto("/admin/integrations");
  expect(await response?.text()).toContain("/spa-assets/");
  await page.context().clearCookies();
  await login(page, users.studi1);
  expect((await page.goto("/admin/integrations"))?.status()).toBe(403);
});
