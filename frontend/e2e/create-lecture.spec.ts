import { expect, test, type APIRequestContext, type Page } from "@playwright/test";

import { apiAs, login } from "./helpers";
import { users } from "./seed";

/**
 * The lectures tab's "create lecture" wizard, on Praktikum: Golang (course 3), which
 * prof1 administers alone.
 *
 * Not on course 1: other files run alongside this one, and course-lectures.spec.ts
 * asserts on the order of course 1's list, which a lecture made here would change.
 * Nothing else reads course 3's lectures. Every lecture made here has a title no
 * other test uses and is dated in 1234, the test year, so nothing listed by date can
 * meet one; it is deleted in a finally through the API, found by that title, so a
 * failing page cannot leave one behind.
 */

const PREFIX = "E2E-Wizard";
const COURSE = 3;
const lecturesPage = `/admin/courses/${COURSE}/lectures`;

interface Listed {
  id: number;
  name: string;
  start: string;
  seriesIdentifier?: string;
}

async function listed(api: APIRequestContext): Promise<Listed[]> {
  const response = await api.get(`/api/v2/courses/${COURSE}/lectures/admin`);
  expect(response.status()).toBe(200);
  return (await response.json()).lectures ?? [];
}

/** Deletes the lectures whose title starts with `title`, this test's own. */
async function cleanUp(api: APIRequestContext, title: string): Promise<void> {
  const ids = (await listed(api)).filter((l) => l.name.startsWith(title)).map((l) => l.id);
  if (!ids.length) return;
  const response = await api.post(`/api/v2/courses/${COURSE}/streams/delete`, { data: { streamIds: ids } });
  expect(response.status()).toBe(200);
}

const cards = (page: Page) => page.locator('li[id^="lecture-"]');
const card = (page: Page, name: string) =>
  cards(page).filter({ has: page.getByRole("heading", { name, exact: true }) });
const wizard = (page: Page) => page.getByRole("region", { name: "Create a lecture" });

async function openWizard(page: Page): Promise<void> {
  await page.getByRole("button", { name: "Create lecture" }).click();
  await expect(wizard(page)).toBeVisible();
}

test.describe("creating lectures from the lectures tab", () => {
  test("creates a scheduled livestream and opens it in the list", async ({ page, playwright }) => {
    const prof1 = await apiAs(playwright, users.prof1);
    const title = `${PREFIX} Einzeltermin`;
    try {
      await login(page, users.prof1, lecturesPage);
      await openWizard(page);
      const w = wizard(page);

      // A livestream, scheduled, is where the wizard starts.
      await expect(w.getByRole("radio", { name: /Livestream/ })).toBeChecked();
      await expect(w.getByRole("radio", { name: /Schedule/ })).toBeChecked();
      await w.getByRole("button", { name: "Continue" }).click();

      // Nothing to create yet: the problems are listed, and the step stays.
      await w.getByRole("button", { name: "Create lecture" }).click();
      await expect(w.getByRole("list", { name: "Problems" })).toContainText("Enter a title.");

      await w.getByRole("textbox", { name: "Title", exact: true }).fill(title);
      await w.getByLabel("Start").fill("1234-05-02T10:00");
      await w.getByLabel("End").fill("1234-05-02T11:30");
      await expect(w.getByText("(1h 30min)")).toBeVisible();
      await w.getByRole("button", { name: "Create lecture" }).click();

      await expect(wizard(page)).toHaveCount(0);
      await expect(page.getByRole("status").filter({ hasText: "Created 1 lecture." })).toBeVisible();
      const created = card(page, title);
      await expect(created).toHaveCount(1);
      // Opened, so its stream key and settings are right there.
      const id = (await created.getAttribute("id"))!.replace("lecture-", "");
      await expect(page.locator(`#lecture-${id}-name`)).toHaveValue(title);

      const found = (await listed(prof1)).filter((l) => l.name === title);
      expect(found).toHaveLength(1);
    } finally {
      await cleanUp(prof1, title);
    }
  });

  test("creates a weekly series of three, one with its own title", async ({ page, playwright }) => {
    const prof1 = await apiAs(playwright, users.prof1);
    const title = `${PREFIX} Reihe`;
    const own = `${PREFIX} Reihe (Exkursion)`;
    try {
      await login(page, users.prof1, lecturesPage);
      await openWizard(page);
      const w = wizard(page);
      await w.getByRole("button", { name: "Continue" }).click();

      await w.getByRole("textbox", { name: "Title", exact: true }).fill(title);
      await w.getByLabel("Start").fill("1234-05-02T10:00");
      await w.getByLabel("End").fill("1234-05-02T11:30");
      await w.getByRole("checkbox", { name: /Recurring/ }).check();
      await expect(w.getByLabel("Repeat")).toHaveValue("weekly");
      await w.getByRole("spinbutton", { name: "Number of lectures" }).fill("3");

      const rows = w.getByRole("list", { name: "Lectures of the series" }).getByRole("listitem");
      await expect(rows).toHaveCount(3);
      await w.getByRole("textbox", { name: "Title of lecture 2" }).fill(own);
      await w.getByRole("button", { name: "Create 3 lectures" }).click();

      await expect(page.getByRole("status").filter({ hasText: "Created 3 lectures." })).toBeVisible();
      await expect(card(page, title)).toHaveCount(2);
      await expect(card(page, own)).toHaveCount(1);

      const series = (await listed(prof1)).filter((l) => l.name.startsWith(title));
      expect(series).toHaveLength(3);
      expect(new Set(series.map((l) => l.seriesIdentifier)).size).toBe(1);
      expect(series[0].seriesIdentifier).toBeTruthy();
      // A week apart, the second being the one with its own title.
      const byStart = [...series].sort((a, b) => a.start.localeCompare(b.start));
      expect(byStart[1].name).toBe(own);
      const days = byStart.map((l) => new Date(l.start).getTime() / 86_400_000);
      expect(days[1] - days[0]).toBeCloseTo(7);
      expect(days[2] - days[1]).toBeCloseTo(7);
    } finally {
      await cleanUp(prof1, title);
    }
  });

  test("offers the lecture hall to server administrators only", async ({ page }) => {
    await login(page, users.admin, lecturesPage);
    await openWizard(page);
    await wizard(page).getByRole("button", { name: "Continue" }).click();
    const hall = wizard(page).getByRole("combobox", { name: "Lecture hall" });
    await expect(hall).toBeVisible();
    await expect(hall.getByRole("option", { name: "None (self-streamed)" })).toHaveCount(1);
    expect(await hall.getByRole("option").count()).toBeGreaterThan(1);

    await page.context().clearCookies();
    await login(page, users.prof1, lecturesPage);
    await openWizard(page);
    await wizard(page).getByRole("button", { name: "Continue" }).click();
    await expect(wizard(page).getByRole("textbox", { name: "Title", exact: true })).toBeVisible();
    await expect(wizard(page).getByRole("combobox", { name: "Lecture hall" })).toHaveCount(0);
    await expect(wizard(page).getByText(/Self-streamed: the stream key/)).toBeVisible();
  });

  test("creates a video upload and says when no worker takes the file", async ({ page, playwright }) => {
    const prof1 = await apiAs(playwright, users.prof1);
    const title = `${PREFIX} Aufzeichnung`;
    try {
      await login(page, users.prof1, lecturesPage);
      await openWizard(page);
      const w = wizard(page);
      // The radio is visually hidden inside its card; a person clicks the card.
      await w.getByText("Video upload", { exact: true }).click();
      await expect(w.getByRole("radio", { name: /Video upload/ })).toBeChecked();
      await expect(w.getByRole("radio", { name: /Schedule/ })).toHaveCount(0);
      await w.getByRole("button", { name: "Continue" }).click();

      await w.getByRole("textbox", { name: "Title", exact: true }).fill(title);
      await w.getByLabel("Date").fill("1234-05-02T10:00");
      await expect(w.getByLabel("End")).toHaveCount(0);
      await w.getByRole("button", { name: "Continue" }).click();

      await w.getByRole("button", { name: "Create lecture" }).click();
      await expect(w.getByRole("list", { name: "Problems" })).toContainText("Choose at least one video.");

      // The fixture's workers are dead (no heartbeat), so the upload has nowhere to go.
      await w.getByLabel("Combined video").setInputFiles({
        name: "lecture.mp4",
        mimeType: "video/mp4",
        buffer: Buffer.from("not really a video"),
      });
      await w.getByRole("button", { name: "Create lecture" }).click();

      await expect(page.getByRole("alert")).toHaveText(
        "No worker is available to receive the upload right now. The lecture was created; upload the video later from its card.",
      );
      await expect(card(page, title)).toHaveCount(1);
      expect((await listed(prof1)).filter((l) => l.name === title)).toHaveLength(1);
    } finally {
      await cleanUp(prof1, title);
    }
  });
});
