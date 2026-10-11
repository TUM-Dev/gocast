import { expect, test } from "@playwright/test";

import {
  courses,
  expected,
  live,
  pinned,
  recordings,
  schedule,
  serverNotifications,
  users,
  type CourseKey,
  type UserKey,
} from "./seed";
import { expectSlugs, sidebarGroup, startPage } from "./visibility";

/**
 * What each kind of caller is shown on the start page, asserted against the rendered
 * page rather than the API alone. A listing that the server filters correctly and the
 * page then renders from the wrong array is exactly as wrong, and neither the Go tests
 * nor the component tests would notice. visibility-course-page.spec.ts is the other
 * half of the matrix: what the same callers see once they open a course.
 *
 * The fixture is tum-live-starter.sql; e2e/seed.ts restates it. These read only, so
 * they neither depend on nor disturb the tests that write.
 */

test.describe("the public listing", () => {
  test("shows an anonymous visitor only the public courses", async ({ page }) => {
    // games101 is `loggedin`: not merely unopenable, but absent from the listing.
    await startPage(page, null, "summer2022");

    await expectSlugs(sidebarGroup(page, "Public Courses"), expected.summer2022.listed.anonymous);
    await expect(page.getByText(courses.games101.name)).toHaveCount(0);
  });

  test("never contains the enrolled-only or the hidden course", async ({ page }) => {
    // Not even for the people who can open them: `enrolled` and `hidden` courses reach
    // their own members through "My Courses", never through the public listing.
    for (const user of ["admin", "prof1", "prof2", "studi1", "studi2"] as UserKey[]) {
      await startPage(page, user, "summer2022");

      const listing = sidebarGroup(page, "Public Courses");
      await expectSlugs(listing, expected.summer2022.listed.signedIn);
    }
  });

  for (const user of Object.keys(users) as UserKey[]) {
    test(`shows ${user} the logged-in-only course as well`, async ({ page }) => {
      await startPage(page, user, "summer2022");

      await expectSlugs(sidebarGroup(page, "Public Courses"), expected.summer2022.listed.signedIn);
    });
  }

  test("is per semester, not cumulative", async ({ page }) => {
    await startPage(page, null, "winter2021");

    await expectSlugs(sidebarGroup(page, "Public Courses"), expected.winter2021.listed.anonymous);
  });
});

test.describe("each caller's own courses", () => {
  for (const semester of ["summer2022", "winter2021"] as const) {
    for (const [user, want] of Object.entries(expected[semester].enrolled) as [
      UserKey,
      CourseKey[],
    ][]) {
      test(`${user} in ${semester} sees ${want.length ? want.join(", ") : "none"}`, async ({
        page,
      }) => {
        await startPage(page, user, semester);

        const group = sidebarGroup(page, "My Courses");
        if (want.length === 0) {
          // The group is left out entirely rather than rendered empty.
          await expect(group).toHaveCount(0);
          await expect(page.locator("#my-courses")).toHaveCount(0);
          return;
        }

        await expectSlugs(group, want);
        // And the same set in the page beside the sidebar, which reads the same store
        // but could easily read a different array from it.
        await expectSlugs(page.locator("#my-courses"), want);
      });
    }
  }

  test("an anonymous visitor has none", async ({ page }) => {
    await startPage(page, null, "summer2022");

    await expect(sidebarGroup(page, "My Courses")).toHaveCount(0);
  });
});

test.describe("the live lectures", () => {
  test("an anonymous visitor is shown none, both courses being out of reach", async ({ page }) => {
    await startPage(page, null, "summer2022");

    await expect(page.locator("#livestreams")).toHaveCount(0);
    for (const { lecture } of live) {
      await expect(page.getByText(lecture)).toHaveCount(0);
    }
  });

  for (const { course, lecture, shownTo } of live) {
    for (const user of Object.keys(users) as UserKey[]) {
      const visible = shownTo === "signedIn" || (shownTo as readonly UserKey[]).includes(user);

      test(`${lecture} is ${visible ? "shown to" : "withheld from"} ${user}`, async ({ page }) => {
        await startPage(page, user, "summer2022");

        // Somebody is always live, so the section itself is present either way and an
        // assertion on it would not distinguish the two cases.
        const livestreams = page.locator("#livestreams");
        await expect(livestreams).toBeVisible();
        await expect(livestreams.getByText(lecture)).toHaveCount(visible ? 1 : 0);
        if (visible) {
          await expect(livestreams.getByText(courses[course].name)).toBeVisible();
        }
      });
    }
  }

  test("marks the hidden course's lecture as hidden for the people who see it", async ({
    page,
  }) => {
    // The badge is the only thing telling an administrator that what they are looking
    // at is not on anyone else's start page.
    await startPage(page, "prof2", "summer2022");

    const card = page
      .locator("#livestreams article")
      .filter({ hasText: courses.geheim.name });
    await expect(card.getByText("Hidden")).toBeVisible();
  });

  test("links the lecture hall to the campus map", async ({ page }) => {
    await startPage(page, "studi1", "summer2022");

    const badge = page.locator('#livestreams a[href^="https://nav.tum.de/room/"]').first();
    await expect(badge).toBeVisible();
  });
});

test.describe("pinned courses", () => {
  test("are listed for the user who pinned them", async ({ page }) => {
    await startPage(page, pinned.user, "summer2022");

    await expectSlugs(sidebarGroup(page, "Pinned Courses"), pinned.visible);
  });

  test("drop a course the user may no longer see", async ({ page }) => {
    // A pin outlives the access that created it, and listing one they cannot open
    // would show a name that goes nowhere.
    await startPage(page, pinned.user, "summer2022");

    await expect(page.getByText(courses[pinned.hiddenFromThem].name)).toHaveCount(0);
  });

  test("are absent for a user who has pinned nothing", async ({ page }) => {
    await startPage(page, "studi1", "summer2022");

    await expect(sidebarGroup(page, "Pinned Courses")).toHaveCount(0);
  });
});

test.describe("today's lectures", () => {
  test("today's lecture is listed under Today for the students in that course", async ({
    page,
  }) => {
    // The fixture dates it late in the evening so it stays ahead of the clock; once it
    // starts there is nothing left to list and the section is correct to be empty.
    const now = new Date();
    const started =
      now.getHours() > schedule.todayStartsAt.hours ||
      (now.getHours() === schedule.todayStartsAt.hours &&
        now.getMinutes() >= schedule.todayStartsAt.minutes);
    test.skip(started, "the fixture's lecture for today has already started");

    // studi3 is enrolled only in this course, so the section has exactly one entry.
    await startPage(page, "studi3", "summer2022");

    const today = page.locator("#live-today");
    await expect(today).toBeVisible();
    await expectSlugs(today, ["brauereiwesen"]);
  });

  test("Today is empty for a visitor with no courses of their own", async ({ page }) => {
    await startPage(page, null, "summer2022");

    await expect(page.locator("#live-today")).toHaveCount(0);
  });
});

test.describe("server notifications", () => {
  test("are shown to everyone, warnings styled apart from the rest", async ({ page }) => {
    await startPage(page, null, "summer2022");

    const banners = page.locator(".tum-live-notification");
    await expect(banners).toHaveCount(serverNotifications.length);
    for (const notification of serverNotifications) {
      const banner = banners.filter({ hasText: notification.text.slice(0, 20) });
      await expect(banner).toHaveClass(
        new RegExp(notification.warn ? "tum-live-notification-warn" : "tum-live-notification-info"),
      );
    }
  });

  test("render the administrator's markup rather than showing its tags", async ({ page }) => {
    await startPage(page, null, "summer2022");

    await expect(page.locator(".tum-live-notification b")).toHaveText("Wartungsarbeiten");
  });
});

test.describe("the recent recordings", () => {
  test("an anonymous visitor is shown the public courses' recordings", async ({ page }) => {
    // With no courses of their own, the section falls back to the public listing.
    await startPage(page, null, "summer2022");

    const recent = page.locator("#recent-vods");
    await expect(recent).toBeVisible();
    await expectSlugs(recent, ["brauereiwesen"]);
  });

  test("a student is shown their own courses' recordings", async ({ page }) => {
    await startPage(page, "studi1", "summer2022");

    const recent = page.locator("#recent-vods");
    // Enrolled in three courses; games101 has never been recorded, so it is left out.
    await expectSlugs(recent, ["brauereiwesen", "bierkunde"]);
    await expect(recent.getByText(recordings.brauereiwesen[0])).toBeVisible();
  });

  test("a pinned course counts as one of their own", async ({ page }) => {
    // studi3 is enrolled in one course and has pinned it; the section must not list it
    // twice for being in both.
    await startPage(page, pinned.user, "summer2022");

    await expectSlugs(page.locator("#recent-vods"), ["brauereiwesen"]);
    await expect(page.locator("#recent-vods article.tum-live-stream")).toHaveCount(1);
  });
});
