import { type APIRequestContext, expect, test } from "@playwright/test";

import { apiAs, login } from "./helpers";
import { schedule, users } from "./seed";

/**
 * The administration schedule at /admin.
 *
 * Today's lecture (`schedule.today`) is Einführung Brauereiwesen's -- course 1, which
 * prof1 and prof2 administer -- in lecture hall HS001, like every lecture in the dump
 * that has a hall. Spieleentwicklung's live lecture (course 2, prof2's but not
 * prof1's) also runs today. The day view opens on today.
 *
 * One test renames today's lecture; it puts the name back in a finally, because the
 * start page's specs look for it by name.
 */

const courseOfToday = "Einführung Brauereiwesen";
const prof2Only = "Spieleentwicklung für Dummies";

/** Today's lecture as the schedule API reports it. */
async function todaysLecture(context: APIRequestContext) {
  const from = new Date();
  from.setHours(0, 0, 0, 0);
  const to = new Date(from.getTime() + 24 * 60 * 60 * 1000);
  const response = await context.get(
    `/api/v2/schedule?from=${from.toISOString()}&to=${to.toISOString()}&allLectureHalls=true`,
  );
  expect(response.status()).toBe(200);
  const lectures: { streamId: number; courseId: number; name?: string }[] =
    (await response.json()).lectures ?? [];
  const lecture = lectures.find((l) => l.name === schedule.today);
  expect(lecture, `${schedule.today} is not on today's schedule`).toBeTruthy();
  return lecture!;
}

test.describe("the schedule page", () => {
  test("is served the SPA shell", async ({ page }) => {
    await login(page, users.prof1);

    const response = await page.goto("/admin");
    expect((await response?.text()) ?? "").toContain("/spa-assets/");
  });

  test("is refused to a student by the server", async ({ page }) => {
    await login(page, users.studi1);

    const response = await page.goto("/admin");
    expect(response?.status()).toBe(403);
  });

  test("shows today's lectures of the lecturer's own courses only", async ({ page }) => {
    await login(page, users.prof1, "/admin");

    await expect(page.locator(".fc-event", { hasText: courseOfToday })).toBeVisible();
    await expect(page.locator(".fc-event", { hasText: prof2Only })).toHaveCount(0);
  });

  test("shows another lecturer theirs", async ({ page }) => {
    await login(page, users.prof2, "/admin");

    await expect(page.locator(".fc-event", { hasText: prof2Only })).toBeVisible();
  });

  test("names the lecture hall beside the time", async ({ page }) => {
    await login(page, users.prof1, "/admin");

    await expect(page.locator(".fc-event", { hasText: courseOfToday })).toContainText("HS001");
  });

  test("hides a lecture hall's lectures when it is unticked", async ({ page }) => {
    await login(page, users.prof1, "/admin");
    await expect(page.locator(".fc-event", { hasText: courseOfToday })).toBeVisible();

    await page.getByRole("button", { name: "Lecture halls" }).click();
    await page.getByRole("checkbox", { name: "HS001" }).uncheck();

    await expect(page.locator(".fc-event", { hasText: courseOfToday })).toHaveCount(0);
    await page.getByRole("checkbox", { name: "All" }).check();
    await expect(page.locator(".fc-event", { hasText: courseOfToday })).toBeVisible();
  });

  test("renames a lecture from its popover", async ({ page, playwright }) => {
    const api = await apiAs(playwright, users.prof1);
    const lecture = await todaysLecture(api);

    try {
      await login(page, users.prof1, "/admin");
      await page.locator(".fc-event", { hasText: courseOfToday }).click();

      const dialog = page.getByRole("dialog", { name: courseOfToday });
      await expect(dialog).toBeVisible();
      const title = dialog.getByRole("textbox", { name: "Lecture title" });
      await expect(title).toHaveValue(schedule.today);

      await title.fill("VL 5: Umbenannt");
      await dialog.getByRole("button", { name: "Save" }).first().click();
      await expect(dialog.getByRole("status")).toHaveText("Title saved.");

      const after = await api.get(
        `/api/v2/schedule?from=${new Date(Date.now() - 864e5).toISOString()}` +
          `&to=${new Date(Date.now() + 864e5).toISOString()}&allLectureHalls=true`,
      );
      const names = ((await after.json()).lectures ?? []).map((l: { name?: string }) => l.name);
      expect(names).toContain("VL 5: Umbenannt");
    } finally {
      const restored = await api.patch(`/api/v2/courses/${lecture.courseId}/streams/${lecture.streamId}`, {
        data: { name: schedule.today },
      });
      expect(restored.status()).toBe(200);
    }
  });

  test("closes the popover on Escape", async ({ page }) => {
    await login(page, users.prof1, "/admin");
    await page.locator(".fc-event", { hasText: courseOfToday }).click();
    await expect(page.getByRole("dialog")).toBeVisible();

    await page.keyboard.press("Escape");
    await expect(page.getByRole("dialog")).toHaveCount(0);
  });
});

test.describe("the schedule API", () => {
  test("refuses a student", async ({ playwright }) => {
    const context = await apiAs(playwright, users.studi1);
    const now = new Date().toISOString();

    expect((await context.get(`/api/v2/schedule?from=${now}&to=${now}`)).status()).toBe(403);
    expect((await context.get("/api/v2/schedule/lecture-halls")).status()).toBe(403);
  });

  test("refuses a window of more than 100 days", async ({ playwright }) => {
    const context = await apiAs(playwright, users.prof1);
    const from = new Date();
    const to = new Date(from.getTime() + 101 * 864e5);

    const response = await context.get(
      `/api/v2/schedule?from=${from.toISOString()}&to=${to.toISOString()}&allLectureHalls=true`,
    );
    expect(response.status()).toBe(400);
  });

  test("will not rename another course's lecture through one's own course", async ({ playwright }) => {
    const prof2 = await apiAs(playwright, users.prof2);
    const from = new Date(Date.now() - 864e5).toISOString();
    const to = new Date(Date.now() + 864e5).toISOString();
    const lectures: { streamId: number; courseId: number }[] =
      (await (await prof2.get(`/api/v2/schedule?from=${from}&to=${to}&allLectureHalls=true`)).json())
        .lectures ?? [];
    const course2Lecture = lectures.find((l) => l.courseId === 2);
    expect(course2Lecture, "no course 2 lecture today").toBeTruthy();

    // prof1 administers course 1 but not course 2.
    const prof1 = await apiAs(playwright, users.prof1);
    const response = await prof1.patch(`/api/v2/courses/1/streams/${course2Lecture!.streamId}`, {
      data: { name: "Hijacked" },
    });
    expect(response.status()).toBe(404);
  });
});
