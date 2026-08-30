import { expect, test, type Page } from "@playwright/test";

import { login } from "./helpers";
import {
  administers,
  canOpen,
  courseUrl,
  courses,
  expected,
  live,
  pinned,
  privateLecture,
  recordings,
  schedule,
  semesters,
  serverNotifications,
  unlistedLecture,
  users,
  watched,
  type CourseKey,
  type UserKey,
} from "./seed";

/**
 * What each kind of caller may see, asserted against the rendered page rather than the
 * API alone. A listing that the server filters correctly and the page then renders
 * from the wrong array is exactly as wrong, and neither the Go tests nor the component
 * tests would notice.
 *
 * The fixture is tum-live-starter.sql; e2e/seed.ts restates it. These read only, so
 * they neither depend on nor disturb the tests that write settings.
 */

/** A sidebar group, addressed by its heading. */
function sidebarGroup(page: Page, heading: string) {
  return page
    .locator("#side-navigation article")
    .filter({ has: page.locator("header", { hasText: heading }) });
}

/** The course slugs a set of course links points at, in the order they are rendered. */
async function linkedSlugs(scope: { locator: Page["locator"] }): Promise<string[]> {
  const links = scope.locator('a[href^="/course/"]');
  const hrefs = await links.evaluateAll((nodes) =>
    nodes.map((node) => (node as HTMLAnchorElement).getAttribute("href") ?? ""),
  );
  return hrefs.map((href) => href.split("/").pop() ?? "");
}

async function expectSlugs(scope: { locator: Page["locator"] }, want: readonly string[]) {
  expect(new Set(await linkedSlugs(scope))).toEqual(new Set(want));
}

/** Opens the start page for a semester, signed in as the given user or as nobody. */
async function startPage(page: Page, user: UserKey | null, semester: keyof typeof semesters) {
  const path = `/${semesters[semester].query}`;
  if (user) await login(page, users[user], path);
  else await page.goto(path);
  // The listings arrive after the shell, so wait for the one group always present.
  await expect(sidebarGroup(page, "Public Courses")).toBeVisible();
}

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

  test("show the course page's pin control as already pinned", async ({ page }) => {
    await login(page, users[pinned.user], courseUrl("brauereiwesen"));

    await expect(page.getByRole("button", { name: "Pinned" })).toBeVisible();
  });

  test("show it unpinned for someone else", async ({ page }) => {
    await login(page, users.studi1, courseUrl("brauereiwesen"));

    await expect(page.getByRole("button", { name: "Pin", exact: true })).toBeVisible();
  });
});

test.describe("lectures still to come", () => {
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
