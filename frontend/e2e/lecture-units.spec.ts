import { type APIRequestContext, expect, test } from "@playwright/test";

import { apiAs, login } from "./helpers";
import { users } from "./seed";

/**
 * A lecture's units page, for its course's lecturers.
 *
 * Works on "VL 2: Wie mache ich Bier?" (stream 2 of course 1, which prof1 and prof2
 * administer; the specs that change lectures leave it alone) and gives every unit a
 * title of its own, so the tests of this file can run in parallel with each other and
 * with the specs that list course 1's lectures. Each removes what it added, through
 * the API, in a finally. The dump has no units. Its recording lives on a host the test
 * machine may or may not reach, so nothing here depends on the player loading it.
 *
 * Stream 7 is course 2's, which prof1 does not administer.
 */

const page2 = "/admin/courses/1/lectures/2/units";
const units2 = "/api/v2/courses/1/streams/2/units";

let counter = 0;
const uniqueTitle = (label: string) => `${label} ${test.info().workerIndex}-${Date.now()}-${counter++}`;

interface ApiUnit {
  id: number;
  title: string;
  fromMs?: number;
  toMs?: number;
}

async function listed(context: APIRequestContext, path = units2): Promise<ApiUnit[]> {
  const response = await context.get(path);
  expect(response.status()).toBe(200);
  return (await response.json()).units ?? [];
}

/** Removes every unit with this title, whatever state the test left it in. */
async function cleanUp(context: APIRequestContext, title: string): Promise<void> {
  for (const unit of (await listed(context)).filter((u) => u.title === title)) {
    await context.delete(`${units2}/${unit.id}`);
  }
}

test.describe("the lecture units page", () => {
  test("is served the SPA shell", async ({ page }) => {
    await login(page, users.prof1);

    const response = await page.goto(page2);
    expect((await response?.text()) ?? "").toContain("/spa-assets/");
  });

  test("moved from the old path", async ({ page }) => {
    await login(page, users.prof1);

    await page.goto("/admin/units/1/2");
    await expect(page).toHaveURL(page2);
  });

  test("names the lecture and says when it has no units", async ({ page }) => {
    await login(page, users.prof1, page2);

    await expect(page.getByRole("heading", { name: "Lecture Units" })).toBeVisible();
    await expect(page.getByText("VL 2: Wie mache ich Bier?")).toBeVisible();
    await expect(page.getByText("This lecture has no units yet.")).toBeVisible();
  });

  test("adds a unit, shows its Markdown description and deletes it again", async ({ page, playwright }) => {
    const title = uniqueTitle("Hopfen");
    const prof1 = await apiAs(playwright, users.prof1);
    try {
      await login(page, users.prof1, page2);

      await page.getByRole("button", { name: "New Unit" }).click();
      await page.getByRole("textbox", { name: /Title/ }).fill(title);
      await page.getByLabel("Description").fill("**fett** und <script>window.__unitXss = 1</script>");
      await page.getByLabel("Start", { exact: true }).fill("0:01:05");
      await page.getByLabel("End", { exact: true }).fill("0:02:30");
      await page.getByRole("button", { name: "Save Unit" }).click();

      const row = page.locator("li", { hasText: title });
      await expect(row).toBeVisible();
      await expect(row).toContainText("00:01:05");
      await expect(row).toContainText("00:02:30");
      await expect(row.locator("strong")).toHaveText("fett");
      // DOMPurify drops the script; a surviving one would have run.
      await expect(row.locator("script")).toHaveCount(0);
      expect(await page.evaluate(() => (window as unknown as { __unitXss?: number }).__unitXss)).toBeUndefined();

      // It is stored, not just drawn.
      const stored = (await listed(prof1)).find((u) => u.title === title);
      expect(stored).toMatchObject({ fromMs: 65000, toMs: 150000 });

      await page.reload();
      await expect(page.locator("li", { hasText: title })).toBeVisible();

      // The first click only arms the button.
      await page.getByRole("button", { name: `Delete ${title}` }).click();
      await expect(page.locator("li", { hasText: title })).toBeVisible();
      await page.getByRole("button", { name: `Confirm deleting ${title}` }).click();
      await expect(page.locator("li", { hasText: title })).toHaveCount(0);
      expect((await listed(prof1)).map((u) => u.title)).not.toContain(title);
    } finally {
      await cleanUp(prof1, title);
    }
  });

  test("says what is wrong with a unit before saving it", async ({ page, playwright }) => {
    const title = uniqueTitle("Malz");
    const prof1 = await apiAs(playwright, users.prof1);
    try {
      await login(page, users.prof1, page2);
      await page.getByRole("button", { name: "New Unit" }).click();

      await page.getByRole("button", { name: "Save Unit" }).click();
      await expect(page.getByRole("alert")).toContainText("Give the unit a title.");

      await page.getByRole("textbox", { name: /Title/ }).fill(title);
      await page.getByLabel("Start", { exact: true }).fill("0:02:00");
      await page.getByLabel("End", { exact: true }).fill("0:01:00");
      await page.getByRole("button", { name: "Save Unit" }).click();
      await expect(page.getByRole("alert")).toContainText("must end after it starts");

      expect((await listed(prof1)).map((u) => u.title)).not.toContain(title);
    } finally {
      await cleanUp(prof1, title);
    }
  });

  test("is linked from a recorded lecture's card", async ({ page }) => {
    await login(page, users.prof1, "/admin/courses/1/lectures");

    await page.locator("#lecture-2").getByRole("link", { name: "Units" }).click();
    await expect(page).toHaveURL(page2);
  });

  test("is refused by the server to a lecturer of other courses", async ({ page }) => {
    await login(page, users.prof1);

    const response = await page.goto("/admin/courses/2/lectures/7/units");
    expect(response?.status()).not.toBe(200);
    expect((await response?.text()) ?? "").not.toContain("/spa-assets/");
  });
});

test.describe("the lecture unit RPCs", () => {
  test("refuse a lecturer of another course, a student and an anonymous caller", async ({ playwright }) => {
    const foreign = "/api/v2/courses/2/streams/7/units";
    for (const [user, status] of [
      [users.prof1, 404],
      [users.studi1, 404],
      [undefined, 401],
    ] as const) {
      const context = await apiAs(playwright, user);
      expect((await context.get(foreign)).status(), `list as ${user?.username}`).toBe(status);
      const created = await context.post(foreign, { data: { title: "x", fromMs: 0, toMs: 1000 } });
      expect(created.status(), `create as ${user?.username}`).toBe(status);
      expect((await context.delete(`${foreign}/1`)).status(), `delete as ${user?.username}`).toBe(status);
    }
  });

  test("answer a lecture that is not the course's as missing", async ({ playwright }) => {
    // prof2 administers both courses, so only the lecture can be at fault.
    const prof2 = await apiAs(playwright, users.prof2);

    expect((await prof2.get("/api/v2/courses/1/streams/7/units")).status()).toBe(404);
    const created = await prof2.post("/api/v2/courses/1/streams/7/units", { data: { title: "x", fromMs: 0, toMs: 1000 } });
    expect(created.status()).toBe(404);
  });

  test("refuse to delete another lecture's unit through this lecture", async ({ playwright }) => {
    const title = uniqueTitle("Gerste");
    const prof2 = await apiAs(playwright, users.prof2);
    try {
      const created = await prof2.post(units2, { data: { title, fromMs: 0, toMs: 1000 } });
      expect(created.status()).toBe(200);
      const { id } = await created.json();

      // Stream 7 is course 2's; the unit is stream 2's.
      expect((await prof2.delete(`/api/v2/courses/2/streams/7/units/${id}`)).status()).toBe(404);
      expect((await listed(prof2)).map((u) => u.title)).toContain(title);
    } finally {
      await cleanUp(prof2, title);
    }
  });

  test("refuse a unit with no title or no length", async ({ playwright }) => {
    const prof1 = await apiAs(playwright, users.prof1);
    const before = (await listed(prof1)).length;

    expect((await prof1.post(units2, { data: { title: "  ", fromMs: 0, toMs: 1000 } })).status()).toBe(400);
    expect((await prof1.post(units2, { data: { title: "x", fromMs: 1000, toMs: 1000 } })).status()).toBe(400);
    expect((await prof1.post(units2, { data: { title: "x", fromMs: 2000, toMs: 1000 } })).status()).toBe(400);

    expect(await listed(prof1)).toHaveLength(before);
  });
});
