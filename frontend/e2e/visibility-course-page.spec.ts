import { expect, test } from "@playwright/test";

import { login } from "./helpers";
import {
  administers,
  canOpen,
  courseUrl,
  courses,
  live,
  pinned,
  privateLecture,
  recordings,
  schedule,
  unlistedLecture,
  users,
  watched,
  type CourseKey,
  type UserKey,
} from "./seed";

/**
 * What each kind of caller may see on a course's own page: whether it opens for them
 * at all, which lectures it lists, whether a private one is among them, and who is
 * offered the admin link. Asserted against the rendered page rather than the API
 * alone, for the reason visibility-start-page.spec.ts gives; that file is the other
 * half of the matrix, the listings on the start page.
 *
 * The fixture is tum-live-starter.sql; e2e/seed.ts restates it. These read only, so
 * they neither depend on nor disturb the tests that write.
 */

test.describe("a course's lectures", () => {
  test("a logged-in-only course refuses an anonymous visitor", async ({ page }) => {
    await page.goto(courseUrl("games101"));

    // The page asks for the course, is refused, and sends them to sign in rather than
    // rendering an empty course.
    await expect(page).toHaveURL(/\/login/);
  });

  test("a public course opens for an anonymous visitor", async ({ page }) => {
    await page.goto(courseUrl("brauereiwesen"));

    await expect(page.locator(".tum-live-course-view .name")).toHaveText(
      courses.brauereiwesen.name,
    );
  });

  test("lists the recordings, newest first, and nothing else", async ({ page }) => {
    await page.goto(courseUrl("brauereiwesen"));

    const titles = page.locator(".tum-live-stream .title");
    await expect(titles).toHaveText(recordings.brauereiwesen);
    // Scheduled for a date that has passed and never recorded: it belongs in no
    // section, and the old page showed it in none either.
    await expect(page.getByText(unlistedLecture)).toHaveCount(0);
  });

  test("shows a recorded lecture's running time", async ({ page }) => {
    // The duration column is null in the dump, so this is the scheduled length —
    // 12:00:00 to 12:09:56.
    await page.goto(courseUrl("brauereiwesen"));

    await expect(page.getByText("00:09:56").first()).toBeVisible();
  });

  test("a course with no recordings lists none", async ({ page }) => {
    await login(page, users.studi1, courseUrl("games101"));

    await expect(page.locator(".tum-live-course-view .name")).toHaveText(courses.games101.name);
    await expect(page.getByText("VODs")).toHaveCount(0);
    // Its one lecture is live, so it is shown as that.
    await expect(page.getByText(live[0].lecture)).toBeVisible();
  });
});

test.describe("opening a course by its own URL", () => {
  // Not the same question as which listing it appears in. `hidden` is where the two
  // come apart: unlisted everywhere, but openable by anyone with the link.
  for (const [slug, who] of Object.entries(canOpen) as [CourseKey, "everyone" | UserKey[]][]) {
    test(`anonymous ${who === "everyone" ? "may open" : "may not open"} ${slug}`, async ({
      page,
    }) => {
      await page.goto(courseUrl(slug));

      if (who === "everyone") {
        await expect(page.locator(".tum-live-course-view .name")).toHaveText(courses[slug].name);
      } else {
        await expect(page).toHaveURL(/\/login/);
      }
    });

    if (who === "everyone") continue;

    for (const user of Object.keys(users) as UserKey[]) {
      const allowed = who.includes(user);
      test(`${user} ${allowed ? "may open" : "may not open"} ${slug}`, async ({ page }) => {
        await login(page, users[user], courseUrl(slug));

        if (allowed) {
          await expect(page.locator(".tum-live-course-view .name")).toHaveText(courses[slug].name);
        } else {
          // Signed in and still refused, so this is the course's rule rather than the
          // session's: the page sends them back to the start page.
          await expect(page).toHaveURL(/\/(\?|$)/);
          await expect(page.locator(".tum-live-course-view")).toHaveCount(0);
        }
      });
    }
  }
});

test.describe("a private lecture", () => {
  for (const user of Object.keys(users) as UserKey[]) {
    const visible = user === "admin" || administers.brauereiwesen.includes(user);

    test(`is ${visible ? "shown to" : "withheld from"} ${user}`, async ({ page }) => {
      await login(page, users[user], courseUrl("brauereiwesen"));
      await expect(page.locator(".tum-live-course-view .name")).toBeVisible();

      await expect(page.getByText(privateLecture)).toHaveCount(visible ? 1 : 0);
    });
  }

  test("is marked as withheld for the administrator who sees it", async ({ page }) => {
    await login(page, users.prof1, courseUrl("brauereiwesen"));

    const card = page.locator("article.tum-live-stream").filter({ hasText: privateLecture });
    await expect(card.locator("i.fa-eye-slash")).toBeVisible();
  });

  test("is not shown to an anonymous visitor either", async ({ page }) => {
    await page.goto(courseUrl("brauereiwesen"));

    await expect(page.getByText(privateLecture)).toHaveCount(0);
  });
});

test.describe("watch progress", () => {
  test("marks what the user has watched and hides it on request", async ({ page }) => {
    await login(page, users[watched.user], courseUrl(watched.course));

    const finished = page.locator("article.tum-live-stream").filter({ hasText: watched.finished });
    await expect(finished.locator("i.fa-eye")).toBeVisible();

    await page.getByRole("button", { name: "Hide watched" }).click();

    await expect(page.getByText(watched.finished)).toHaveCount(0);
    // The half-watched one stays: the filter is about finished, not started.
    await expect(page.getByText(watched.partial)).toBeVisible();
  });

  test("belongs to the user, not the course", async ({ page }) => {
    // studi2 is in the same course and has watched nothing.
    await login(page, users.studi2, courseUrl(watched.course));

    await expect(page.locator("i.fa-eye")).toHaveCount(0);
  });
});

test.describe("the pin control on a course page", () => {
  test("is shown as already pinned to the user who pinned the course", async ({ page }) => {
    await login(page, users[pinned.user], courseUrl("brauereiwesen"));

    await expect(page.getByRole("button", { name: "Pinned" })).toBeVisible();
  });

  test("is shown unpinned to someone else", async ({ page }) => {
    await login(page, users.studi1, courseUrl("brauereiwesen"));

    await expect(page.getByRole("button", { name: "Pin", exact: true })).toBeVisible();
  });
});

test.describe("lectures still to come", () => {
  test("the course page schedules the rest, three at a time", async ({ page }) => {
    await page.goto(courseUrl("brauereiwesen"));

    // Scoped to the section: the sidebar's semester picker has a "Show all" too.
    const section = page
      .locator("section")
      .filter({ has: page.getByRole("heading", { name: "Scheduled" }) });
    const scheduled = section.locator("article.rounded-lg");

    // How many, not which: `today` counts as scheduled until the evening it starts,
    // so the first three shift during the day.
    await expect(scheduled).toHaveCount(schedule.plannedShown);

    await section.getByRole("button", { name: "Show all" }).click();

    // Not an exact count for the same reason: at least the four that are always
    // scheduled, and every one of them named.
    await expect(scheduled).not.toHaveCount(schedule.plannedShown);
    for (const lecture of schedule.planned) {
      await expect(page.getByText(lecture)).toBeVisible();
    }
  });

  test("a lecture about to start offers the waiting room", async ({ page }) => {
    await login(page, users.studi1, courseUrl(schedule.comingUp.course));

    await expect(page.getByRole("link", { name: "Join waiting room" })).toBeVisible();
  });
});

test.describe("the admin link on a course", () => {
  const cases: { user: UserKey; course: CourseKey; expectVisible: boolean }[] = [
    // The wildcard permission reaches every course.
    { user: "admin", course: "brauereiwesen", expectVisible: true },
    { user: "admin", course: "games101", expectVisible: true },
    // The enrolled-only and hidden courses are listed separately: not every user here
    // can open them at all.
    ...(["brauereiwesen", "games101"] as CourseKey[]).flatMap((course) =>
      (["prof1", "prof2", "studi1"] as UserKey[]).map((user) => ({
        user,
        course,
        expectVisible: administers[course].includes(user),
      })),
    ),
    { user: "prof1" as UserKey, course: "bierkunde" as CourseKey, expectVisible: true },
    { user: "studi1" as UserKey, course: "bierkunde" as CourseKey, expectVisible: false },
    { user: "prof2" as UserKey, course: "geheim" as CourseKey, expectVisible: true },
    { user: "studi2" as UserKey, course: "geheim" as CourseKey, expectVisible: false },
  ];

  for (const { user, course, expectVisible } of cases) {
    test(`${expectVisible ? "is offered to" : "is withheld from"} ${user} on ${course}`, async ({
      page,
    }) => {
      await login(page, users[user], courseUrl(course));

      const admin = page.getByRole("link", { name: "Admin" });
      await expect(page.locator(".tum-live-course-view .name")).toBeVisible();
      await expect(admin).toHaveCount(expectVisible ? 1 : 0);
    });
  }
});
