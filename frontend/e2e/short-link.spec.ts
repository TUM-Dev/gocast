import { expect, test } from "@playwright/test";

import { courses, live, schedule, shortLinks } from "./seed";

/**
 * A course short link (`/<link>`) redirects to the watch page of the course's current
 * or next lecture, and that page is what enforces the course's visibility. It used to
 * render the lecture itself without those checks, so an anonymous visitor read the
 * title and description of a logged-in-only or enrolled-only course.
 *
 * The links come from tum-live-starter.sql. Read-only, so safe next to every other file.
 */

const lectureTitles = {
  games101: live[0].lecture,
  bierkunde: schedule.comingUp.lecture,
} as const;

test.describe("a course short link", () => {
  for (const slug of ["games101", "bierkunde"] as const) {
    test(`${courses[slug].visibility} course: redirects an anonymous visitor without naming the lecture`, async ({
      page,
      request,
    }) => {
      // The first hop on its own: a 302 to the watch page whose body says nothing
      // about the lecture.
      const hop = await request.get(`/${shortLinks[slug]}`, { maxRedirects: 0 });
      expect(hop.status()).toBe(302);
      expect(hop.headers()["location"]).toMatch(new RegExp(`^/w/${slug}/\\d+$`));
      expect(await hop.text()).not.toContain(lectureTitles[slug]);

      // The browser follows it and is refused there: sent to sign in, or shown the
      // access error, but never the lecture.
      const response = await page.goto(`/${shortLinks[slug]}`);
      const refused = page.url().includes("/login") || response?.status() === 403;
      expect(refused, `ended at ${page.url()} with ${response?.status()}`).toBe(true);
      await expect(page.getByText(lectureTitles[slug])).toHaveCount(0);
    });
  }

  test("public course: lands on the lecture's watch page", async ({ page, request }) => {
    const hop = await request.get(`/${shortLinks.brauereiwesen}`, { maxRedirects: 0 });
    expect(hop.status()).toBe(302);
    expect(hop.headers()["location"]).toMatch(/^\/w\/brauereiwesen\/\d+$/);

    const response = await page.goto(`/${shortLinks.brauereiwesen}`);
    expect(response?.status()).toBe(200);
    await expect(page).toHaveURL(/\/w\/brauereiwesen\/\d+$/);
  });

  test("an unknown link is a 404", async ({ page }) => {
    const response = await page.goto("/no-such-short-link");
    expect(response?.status()).toBe(404);
  });
});
