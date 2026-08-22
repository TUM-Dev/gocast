import { expect, test, type Page } from "@playwright/test";

import { login } from "./helpers";
import { users } from "./seed";

/**
 * The settings page is the first migrated page that writes. What matters here is not
 * that the controls move, but that what they store is read back correctly — by this
 * page after a reload, and by the server-rendered pages reading the same rows.
 *
 * Every test changes a setting away from whatever is currently stored rather than to a
 * fixed value. The account outlives the run, so a test that assumes a starting value
 * passes once and then fails on the state its predecessor left behind.
 */

const GREETINGS = ["Moin", "Servus"] as const;
type Greeting = (typeof GREETINGS)[number];

async function storedGreeting(page: Page): Promise<Greeting> {
  return (await page.getByRole("radio", { name: "Servus" }).isChecked()) ? "Servus" : "Moin";
}

async function storedSeekingTime(page: Page): Promise<number> {
  for (const seconds of [5, 10, 30]) {
    if (await page.getByRole("radio", { name: `${seconds}s` }).isChecked()) return seconds;
  }
  throw new Error("no seeking time is selected");
}

test.beforeEach(async ({ page }) => {
  await login(page, users.studi2, "/settings");
  await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();
});

test("a saved greeting survives a reload", async ({ page }) => {
  const next = (await storedGreeting(page)) === "Servus" ? "Moin" : "Servus";

  await page.getByRole("radio", { name: next }).check();
  await expect(page.getByRole("status")).toHaveText("Greeting saved.");

  await page.reload();
  await expect(page.getByRole("radio", { name: next })).toBeChecked();
});

test("a saved greeting is stored as the bytes its Go getter reads", async ({ page }) => {
  // The regression this guards: settings whose Go getter returns the stored string
  // verbatim must not be written as JSON. Encoding the greeting put literal quotes on
  // every page that displayed it.
  //
  // This used to read the server-rendered start page, which was the only place the
  // quotes showed. That page is the SPA now, and a client that encodes on write and
  // decodes on read hides the fault from itself — so this asserts the stored value
  // over the wire instead, which is what every other reader of it sees.
  const next = (await storedGreeting(page)) === "Servus" ? "Moin" : "Servus";

  await page.getByRole("radio", { name: next }).check();
  await expect(page.getByRole("status")).toHaveText("Greeting saved.");

  // The API accepts the session cookie as well as a bearer token, so the request
  // context carries enough on its own.
  const response = await page.request.get("/api/v2/users/me");
  expect(response.ok()).toBe(true);
  const body = await response.json();
  const stored = body.user.settings.find(
    (setting: { type: string }) => setting.type === "GREETING",
  );

  expect(stored?.value).toBe(next);

  // And that the start page renders it, which is what the quotes were visible in.
  await page.goto("/");
  const greeting = page.getByText(new RegExp(next)).first();
  await expect(greeting).toBeVisible();
  await expect(greeting).not.toContainText('"');
});

test("seeking time is stored as a number the Go getter accepts", async ({ page }) => {
  // GetSeekingTime silently falls back to 10 for anything outside validSeekingTimes,
  // so a value that fails to round-trip looks like the page ignoring the click.
  const next = (await storedSeekingTime(page)) === 30 ? 5 : 30;

  await page.getByRole("radio", { name: `${next}s` }).check();
  await expect(page.getByRole("status")).toHaveText("Seeking time saved.");

  await page.reload();
  await expect(page.getByRole("radio", { name: `${next}s` })).toBeChecked();
});

test("auto skip toggles and stays toggled", async ({ page }) => {
  const autoSkip = page.getByRole("checkbox", { name: /Skip the silence/ });
  const before = await autoSkip.isChecked();

  await autoSkip.setChecked(!before);
  await expect(page.getByRole("status")).toHaveText("Auto skip saved.");

  await page.reload();
  await expect(page.getByRole("checkbox", { name: /Skip the silence/ })).toBeChecked({
    checked: !before,
  });
});

test("a custom playback speed can be added and removed", async ({ page }) => {
  const speed = "1.35";
  const chip = page.getByRole("button", { name: `${speed} ✕` });

  // Leaving the chip behind would eventually fill the three-speed allowance, so the
  // test removes it again — and that removal is worth asserting anyway.
  if ((await chip.count()) > 0) {
    await chip.click();
    await expect(page.getByRole("status")).toHaveText("Custom speeds saved.");
  }

  await page.getByLabel("New custom speed").fill(speed);
  await page.getByRole("button", { name: "Add", exact: true }).click();
  await expect(page.getByRole("status")).toHaveText("Custom speeds saved.");

  await page.reload();
  await expect(page.getByRole("button", { name: `${speed} ✕` })).toBeVisible();

  await page.getByRole("button", { name: `${speed} ✕` }).click();
  await expect(page.getByRole("status")).toHaveText("Custom speeds saved.");
  await page.reload();
  await expect(page.getByRole("button", { name: `${speed} ✕` })).toHaveCount(0);
});

test("an out-of-range speed is refused before it reaches the server", async ({ page }) => {
  const writes: string[] = [];
  page.on("request", (request) => {
    if (request.method() === "PATCH") writes.push(request.url());
  });

  await page.getByLabel("New custom speed").fill("9");
  await page.getByRole("button", { name: "Add", exact: true }).click();

  await expect(page.getByRole("alert")).toContainText("between 0.25 and 5");
  expect(writes).toEqual([]);
});
