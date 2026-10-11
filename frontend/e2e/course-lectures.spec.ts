import { expect, test, type APIRequestContext, type Page } from "@playwright/test";

import { apiAs, login } from "./helpers";
import { recordings, schedule, users } from "./seed";

/**
 * A course's lecture list, the lectures tab of its administration page.
 *
 * Course 1 (Einführung Brauereiwesen) is administered by prof1 and prof2; course 2 is
 * prof2's alone. The one test that renames a lecture puts the exact name back in a
 * finally, through the API so a failing page cannot leave it changed: other specs
 * look lectures up by name. No test here moves a lecture in time.
 */

const page1 = "/admin/courses/1/lectures";
/** Planned, so renaming it touches nothing other specs read about recordings. */
const renamed = schedule.planned[1];

const cards = (page: Page) => page.locator('li[id^="lecture-"]');
const card = (page: Page, name: string) =>
  cards(page).filter({ has: page.getByRole("heading", { name, exact: true }) });

test.describe("the course lectures page", () => {
  test("is served the SPA shell", async ({ page }) => {
    await login(page, users.prof1);

    const response = await page.goto(page1);
    expect((await response?.text()) ?? "").toContain("/spa-assets/");
  });

  test("is where the old server-rendered course page now sends its visitors", async ({ page }) => {
    await login(page, users.prof1);

    await page.goto("/admin/course/1");
    await expect(page).toHaveURL(page1);
  });

  test("keeps the old course page's URL from revealing anything to a student", async ({ page }) => {
    await login(page, users.studi1);

    // The redirect itself is public; its destination is what checks the course.
    const response = await page.goto("/admin/course/1");
    expect(response?.status()).toBe(403);
    await expect(page).toHaveURL(page1);
  });

  test("lists the course's lectures with their state", async ({ page }) => {
    await login(page, users.prof1, page1);

    // Inside the course's administration page, as its lectures tab.
    await expect(page.getByRole("heading", { level: 1 })).toHaveText("Einführung Brauereiwesen");
    await expect(
      page.getByRole("navigation", { name: "Course administration" }).getByRole("link", { name: "Lectures" }),
    ).toHaveAttribute("aria-current", "page");
    await expect(page.getByRole("heading", { name: "Lectures", level: 2 })).toBeVisible();
    for (const name of [...recordings.brauereiwesen, schedule.today, ...schedule.planned]) {
      await expect(card(page, name)).toHaveCount(1);
    }
    await expect(card(page, "VL 1: Was ist Bier?").getByRole("listitem").first()).toHaveText("VoD");
    await expect(card(page, schedule.planned[0]).getByRole("listitem").first()).toHaveText("Scheduled");
  });

  test("sorts newest or oldest first, and remembers the choice", async ({ page }) => {
    await login(page, users.prof1, page1);
    const headings = cards(page).locator("h2");
    await expect(headings.first()).toBeVisible();

    const newestFirst = await headings.allTextContents();
    expect(newestFirst.at(-1)).toBe("VL 1: Was ist Bier?");

    // The button names the order shown.
    await page.getByRole("button", { name: "Newest first" }).click();
    await expect(headings.first()).toHaveText("VL 1: Was ist Bier?");

    await page.reload();
    await expect(headings.first()).toHaveText("VL 1: Was ist Bier?");

    // Back to the default, so the next test starts where a new visitor does.
    await page.getByRole("button", { name: "Oldest first" }).click();
    await expect(headings.last()).toHaveText("VL 1: Was ist Bier?");
  });

  test("renames a lecture from its card", async ({ page, playwright }) => {
    await login(page, users.prof1, page1);

    const target = card(page, renamed);
    const id = (await target.getAttribute("id"))!.replace("lecture-", "");
    const newName = `${renamed} (umbenannt)`;

    try {
      await target.getByRole("button", { name: "Edit" }).click();
      const title = page.locator(`#lecture-${id}-name`);
      await expect(title).toHaveValue(renamed);
      await title.fill(newName);
      await title.locator("xpath=ancestor::form").getByRole("button", { name: "Save" }).click();
      await expect(page.locator(`#lecture-${id}`).getByRole("status")).toHaveText(
        "Title and description saved.",
      );

      await page.reload();
      await expect(page.locator(`#lecture-${id} h2`)).toHaveText(newName);
    } finally {
      const prof1 = await apiAs(playwright, users.prof1);
      const restored = await prof1.patch(`/api/v2/courses/1/streams/${id}`, { data: { name: renamed } });
      expect(restored.status()).toBe(200);
    }

    await page.reload();
    await expect(page.locator(`#lecture-${id} h2`)).toHaveText(renamed);
  });

  test("opens the card the fragment names", async ({ page }) => {
    await login(page, users.prof1);

    await page.goto(`${page1}#lecture-1`);
    await expect(page.locator("#lecture-1-name")).toHaveValue("VL 1: Was ist Bier?");
    await expect(page.locator("#lecture-1").getByRole("button", { name: "Close" })).toBeVisible();
    // Only that one.
    await expect(page.getByRole("button", { name: "Close" })).toHaveCount(1);
  });

  test("is refused by the server to a lecturer of other courses", async ({ page }) => {
    await login(page, users.prof1);

    const response = await page.goto("/admin/courses/2/lectures");
    expect(response?.ok()).toBe(false);
    expect((await response?.text()) ?? "").not.toContain("/spa-assets/");
  });

  test("is refused by the server to a student of the course", async ({ page }) => {
    await login(page, users.studi1);

    const response = await page.goto(page1);
    expect(response?.ok()).toBe(false);
    expect((await response?.text()) ?? "").not.toContain("/spa-assets/");
  });
});

/**
 * Lecture halls are shared across courses, so only server administrators move
 * lectures between them. The fixture's lectures in a hall are all in HS001, ID 1;
 * every test that moves one puts it back there in a finally.
 */
test.describe("changing a lecture's hall", () => {
  const HS001 = 1;
  const moved = [schedule.planned[0], schedule.planned[1]];

  async function idsOf(context: APIRequestContext, names: readonly string[]): Promise<number[]> {
    const response = await context.get("/api/v2/courses/1/lectures/admin");
    expect(response.status()).toBe(200);
    const listed: { id: number; name: string; lectureHallId?: number }[] = (await response.json()).lectures;
    return names.map((name) => {
      const found = listed.find((l) => l.name === name);
      expect(found, `${name} is not listed`).toBeTruthy();
      return found!.id;
    });
  }

  async function hallsOf(context: APIRequestContext, ids: number[]): Promise<number[]> {
    const response = await context.get("/api/v2/courses/1/lectures/admin");
    const listed: { id: number; lectureHallId?: number }[] = (await response.json()).lectures;
    return ids.map((id) => listed.find((l) => l.id === id)?.lectureHallId ?? 0);
  }

  test("is refused to a lecturer by the API", async ({ playwright }) => {
    const prof1 = await apiAs(playwright, users.prof1);
    const [id] = await idsOf(prof1, [moved[0]]);

    expect((await prof1.patch(`/api/v2/courses/1/streams/${id}`, { data: { lectureHallId: 0 } })).status()).toBe(
      403,
    );
    expect(
      (await prof1.put("/api/v2/courses/1/streams/lecture-hall", { data: { streamIds: [id], lectureHallId: 0 } }))
        .status(),
    ).toBe(403);
    // Everything else still saves for them.
    expect((await prof1.patch(`/api/v2/courses/1/streams/${id}`, { data: { name: moved[0] } })).status()).toBe(200);

    expect(await hallsOf(prof1, [id])).toEqual([HS001]);
  });

  test("is not offered to a lecturer on the page", async ({ page }) => {
    await login(page, users.prof1, page1);

    const target = card(page, moved[0]);
    await target.getByRole("button", { name: "Edit" }).click();
    await expect(target.getByText("Streamed from")).toBeVisible();
    await expect(target.locator("select")).toHaveCount(1); // the copy target only
    await expect(page.getByLabel("Move selected to")).toHaveCount(0);
  });

  test("moves selected lectures in bulk for a server administrator", async ({ page, playwright }) => {
    const admin = await apiAs(playwright, users.admin);
    const ids = await idsOf(admin, moved);

    try {
      await login(page, users.admin, page1);
      for (const name of moved) {
        await page.getByRole("checkbox", { name: `Select ${name}` }).check();
      }
      await page.getByLabel("Move selected to").selectOption({ label: "Self-streaming" });
      await page.getByRole("button", { name: "Move", exact: true }).click();
      await expect(page.getByRole("status")).toHaveText("Moved 2 lectures to self-streaming.");
      for (const name of moved) {
        await expect(card(page, name)).toContainText("Self-streamed");
      }

      expect(await hallsOf(admin, ids)).toEqual([0, 0]);
    } finally {
      const restored = await admin.put("/api/v2/courses/1/streams/lecture-hall", {
        data: { streamIds: ids, lectureHallId: HS001 },
      });
      expect(restored.status()).toBe(200);
    }

    expect(await hallsOf(admin, ids)).toEqual([HS001, HS001]);
  });
});
