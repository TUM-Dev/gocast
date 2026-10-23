import { expect, test } from "@playwright/test";

import { login } from "./helpers";
import { users } from "./seed";

/**
 * Saving lectures offline. Saving itself needs a runner and the installed app, neither
 * of which the browser tests have; what is covered here is where the feature is not
 * offered, and the offline page. Read-only, so safe in parallel.
 */

// The enrolled-only course allows downloads, and studi1 is enrolled in it.
const downloadable = "/w/bierkunde/9";

test("offers downloads but not offline saving in a browser tab", async ({ page }) => {
  await login(page, users.studi1, downloadable);
  await page.getByRole("button", { name: "Attachments and downloads" }).click();
  const menu = page.getByRole("menu", { name: "Attachments and downloads" });
  await expect(menu.getByRole("menuitem", { name: /^Download / }).first()).toBeVisible();
  await expect(menu.getByTestId("save-offline")).toHaveCount(0);
});

test("the offline page opens without signing in", async ({ page }) => {
  await page.goto("/offline");
  await expect(page.getByRole("heading", { name: "Offline lectures" })).toBeVisible();
  await expect(page.getByTestId("offline-empty")).toBeVisible();
});
