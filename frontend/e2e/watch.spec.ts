import { expect, test } from "@playwright/test";

import { apiAs, login } from "./helpers";
import { users } from "./seed";

/**
 * The watch page, the embed and the live management page, `/w/:slug/:streamID` and
 * `/admin/courses/:courseID/lectures/:streamID/live`. Read-only, so safe in parallel.
 *
 * The fixture's recordings point at a stream host the browser tests cannot reach, so
 * none of this asserts on playback itself; the player is there, and its page is
 * around it. Streams: 1 is a public recording in brauereiwesen, 11 a private one in
 * the same course, 7 the live lecture of the signed-in-only games101 (prof2
 * administers it), 9 a recording in the enrolled-only bierkunde, and 17 the lecture of
 * bierkunde that starts within the half hour.
 *
 * Signed-in visits to stream 1 use studi3, never studi1: studi1's progress on that
 * stream is seeded for visibility-course-page.spec.ts to assert on (see `watched`), and
 * a spec that opens a recording as studi1 is one more way to disturb it.
 */

test.describe("the watch page", () => {
  test("plays a recording for an anonymous visitor of a public course", async ({ page }) => {
    await page.goto("/w/brauereiwesen/1");

    await expect(page.getByTestId("watch-title")).toHaveText("VL 1: Was ist Bier?");
    await expect(page.getByTestId("watch-player")).toBeVisible();
    // The chat is off in the fixture, and the old page showed no column then, so the
    // panels start collapsed; the button opens them.
    await expect(page.getByTestId("region-sidebar")).toBeHidden();
    await page.getByTestId("open-sidebar").click();
    await expect(page.getByTestId("region-sidebar")).toBeVisible();
    await expect(page.getByRole("tab", { name: "Sections" })).toBeVisible();
    // The page sits in the app: header and footer around it.
    await expect(page.getByRole("banner")).toBeVisible();
  });

  test("switches the layout from the view menu", async ({ page }) => {
    await login(page, users.studi3, "/w/brauereiwesen/1");
    await expect(page.getByTestId("watch-player")).toBeVisible();

    await page.getByRole("button", { name: "Change view" }).click();
    await page.getByRole("menuitemradio", { name: "Presentation only" }).click();
    await expect(page).toHaveURL(/\/w\/brauereiwesen\/1\/PRES$/);
    await expect(page.getByTestId("watch-player")).toBeVisible();

    await page.getByRole("button", { name: "Change view" }).click();
    await page.getByRole("menuitemradio", { name: "Split view" }).click();
    await expect(page).toHaveURL(/\/w\/brauereiwesen\/1\/SPLIT$/);
    await expect(page.getByTestId("split-view")).toBeVisible();
  });

  test("an unknown version is the not-found page with a 404 status", async ({ page }) => {
    const response = await page.goto("/w/brauereiwesen/1/NOPE");

    expect(response?.status()).toBe(404);
    await expect(page.getByText("This page does not exist.")).toBeVisible();
  });

  test("a known version is found whatever its case", async ({ page }) => {
    const response = await page.goto("/w/brauereiwesen/1/pres");

    expect(response?.status()).toBe(200);
    await expect(page.getByTestId("watch-player")).toBeVisible();
  });

  test("names the page as its canonical link, without the query", async ({ page }) => {
    await page.goto("/w/brauereiwesen/1/PRES?t=30&rate=2");

    await expect(page.getByTestId("watch-player")).toBeVisible();
    // The host is the deployment's configured address, so only the path is asserted.
    await expect(page.locator('link[rel="canonical"]')).toHaveAttribute("href", /\/w\/brauereiwesen\/1\/PRES$/);
  });

  test("opens the add-bookmark form on the B key for a signed-in viewer", async ({ page }) => {
    await login(page, users.studi3, "/w/brauereiwesen/1");
    await expect(page.getByTestId("watch-player")).toBeVisible();
    await page.getByTestId("watch-player").hover();

    await page.keyboard.press("b");
    // The sidebar is closed on this stream until the key opens it on the Bookmarks tab.
    await expect(page.getByRole("tab", { name: "Bookmarks", selected: true })).toBeVisible();
    await expect(page.getByLabel("Description")).toBeFocused();
    // Nothing is saved: the form is cancelled.
    await page.getByRole("button", { name: "Cancel" }).click();
  });

  test("shows the waiting room for a lecture that has not started", async ({ page }) => {
    await login(page, users.studi1, "/w/bierkunde/17");

    await expect(page.getByTestId("waiting-room")).toBeVisible();
    await expect(page.getByTestId("countdown")).toBeVisible();
    await expect(page.getByTestId("watch-title")).toHaveText("VL 2: Verkostung");
    // Nothing to act on yet.
    await expect(page.getByTestId("watch-action-bar")).toHaveCount(0);
  });

  test("shows the live lecture to a signed-in student", async ({ page }) => {
    await login(page, users.studi1, "/w/games101/7");

    await expect(page.getByTestId("watch-title")).toHaveText("VL 1: Livestream");
    await expect(page.getByTestId("live-badge")).toBeVisible();
    await expect(page.getByTestId("watch-admin")).toHaveCount(0);
  });

  test("gives the lecturer the controls under the player", async ({ page }) => {
    await login(page, users.prof2, "/w/games101/7");

    await expect(page.getByTestId("watch-admin")).toBeVisible();
    await expect(page.getByRole("link", { name: "Live management" })).toBeVisible();
  });
});

/**
 * With the chat on the sidebar is there from the start, as the old page's chat column
 * was. Stream 2 is a recording of brauereiwesen that no other spec reads; prof1
 * administers the course. The chat goes back off afterwards.
 */
test.describe("the watch page with the chat on", () => {
  test.describe.configure({ mode: "serial" });
  const stream = "/api/v2/courses/1/streams/2";

  test.beforeAll(async ({ playwright }) => {
    const prof1 = await apiAs(playwright, users.prof1);
    expect((await prof1.patch(stream, { data: { chat_enabled: true } })).status()).toBe(200);
  });
  test.afterAll(async ({ playwright }) => {
    const prof1 = await apiAs(playwright, users.prof1);
    await prof1.patch(stream, { data: { chat_enabled: false } });
  });

  test("shows the chat beside the player without asking", async ({ page }) => {
    await page.goto("/w/brauereiwesen/2");

    await expect(page.getByTestId("region-sidebar")).toBeVisible();
    await expect(page.getByRole("tab", { name: "Chat" })).toHaveAttribute("aria-selected", "true");
    await expect(page.getByTestId("open-sidebar")).toHaveCount(0);
    const player = await page.getByTestId("region-player").boundingBox();
    const sidebar = await page.getByTestId("region-sidebar").boundingBox();
    expect(sidebar!.x).toBeGreaterThanOrEqual(player!.x + player!.width - 1);
  });

  test("puts the chat below the player on a phone", async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/w/brauereiwesen/2");

    await expect(page.getByTestId("region-sidebar")).toBeVisible();
    const player = await page.getByTestId("region-player").boundingBox();
    const sidebar = await page.getByTestId("region-sidebar").boundingBox();
    expect(sidebar!.y).toBeGreaterThanOrEqual(player!.y + player!.height - 1);
  });

  test("keeps the tab the viewer chose earlier", async ({ page }) => {
    await page.addInitScript(() => localStorage.setItem("watch.sidebarTab", "sections"));
    await page.goto("/w/brauereiwesen/2");

    await expect(page.getByTestId("region-sidebar")).toBeVisible();
    await expect(page.getByRole("tab", { name: "Sections" })).toHaveAttribute("aria-selected", "true");
  });
});

test.describe("who may open a stream", () => {
  test("sends a visitor who has not signed in to the login page", async ({ page }) => {
    await page.goto("/w/games101/7");

    await expect(page).toHaveURL(/\/login/);
  });

  test("refuses a student the course does not admit", async ({ page }) => {
    await login(page, users.studi3, "/w/bierkunde/9");

    await expect(page.getByText("You don't have permissions to access this stream")).toBeVisible();
    await expect(page.getByTestId("watch-player")).toHaveCount(0);
  });

  test("hides a private lecture from students and anonymous visitors", async ({ page }) => {
    const anonymous = await page.goto("/w/brauereiwesen/11");
    const refused = page.url().includes("/login") || anonymous?.status() === 403;
    expect(refused, `ended at ${page.url()} with ${anonymous?.status()}`).toBe(true);

    await login(page, users.studi1, "/settings");
    const student = await page.goto("/w/brauereiwesen/11");
    expect(student?.status()).toBe(403);
    await expect(page.getByTestId("watch-player")).toHaveCount(0);
  });

  test("shows a private lecture to the people who administer the course", async ({ page }) => {
    await login(page, users.prof1, "/w/brauereiwesen/11");

    await expect(page.getByTestId("watch-title")).toBeVisible();
  });
});

test.describe("the embed", () => {
  test("renders the player bare on the same URL, without signing in", async ({ page }) => {
    await page.goto("/w/brauereiwesen/1?video_only=1");

    await expect(page.getByTestId("watch-embed")).toBeVisible();
    // No application around it, no title block, no panels.
    await expect(page.getByRole("banner")).toHaveCount(0);
    await expect(page.getByRole("contentinfo")).toHaveCount(0);
    await expect(page.getByTestId("watch-title")).toHaveCount(0);
    await expect(page.getByTestId("region-sidebar")).toHaveCount(0);
  });

  test("takes the version from the URL as the page does", async ({ page }) => {
    await page.goto("/w/brauereiwesen/1/PRES?video_only=1");

    await expect(page.getByTestId("watch-embed")).toBeVisible();
  });

  test("tells when a lecture that has not begun will start", async ({ page }) => {
    await login(page, users.studi1, "/w/bierkunde/17?video_only=1");

    await expect(page.getByTestId("embed-starts-in")).toBeVisible();
  });
});

test.describe("the live management page", () => {
  const live = "/admin/courses/2/lectures/7/live";

  test("opens for the lecturer of the course", async ({ page }) => {
    await login(page, users.prof2, live);

    await expect(page.getByRole("heading", { name: "VL 1: Livestream" })).toBeVisible();
    await expect(page.getByTestId("viewers")).toBeVisible();
  });

  test("is refused to a student", async ({ page }) => {
    await login(page, users.studi1, "/settings");
    const response = await page.goto(live);

    expect(response?.status()).toBe(403);
    await expect(page.getByTestId("viewers")).toHaveCount(0);
  });

  test("is refused to a lecturer of another course", async ({ page }) => {
    await login(page, users.prof1, "/settings");
    const response = await page.goto(live);

    expect(response?.status()).toBe(403);
  });

  test("the old management URL redirects to it", async ({ page }) => {
    await login(page, users.prof2);
    const hop = await page.request.get("/admin/management/2/7", { maxRedirects: 0 });

    expect(hop.status()).toBe(301);
    expect(hop.headers()["location"]).toBe(live);
  });

  test("the old management URL does not reveal anything to a student", async ({ page }) => {
    await login(page, users.studi1, "/settings");
    const hop = await page.request.get("/admin/management/2/7", { maxRedirects: 0 });

    expect(hop.status()).not.toBe(301);
  });
});

/**
 * Stream 1 has German and English subtitles in the fixture (tum-live-starter.sql), so
 * its sidebar offers the Transcript and Search tabs. Meilisearch is not part of the dev
 * or CI stack, so search is only asserted to say it is unavailable, never on hits.
 */
test.describe("the watch page of a recording with subtitles", () => {
  test("offers the subtitles in the player and serves them", async ({ page }) => {
    await page.goto("/w/brauereiwesen/1");
    await expect(page.getByTestId("watch-player")).toBeVisible();
    // video.js builds the captions button only for a player that has text tracks.
    await expect(page.locator(".vjs-subs-caps-button")).toBeAttached();

    const res = await page.request.get("/api/v2/streams/brauereiwesen/1/subtitles/en");
    expect(res.status()).toBe(200);
    expect(await res.text()).toContain("Welcome to the lecture on brewing.");
  });

  test("lists the transcript and keeps the other tabs reachable afterwards", async ({ page }) => {
    await login(page, users.studi3, "/w/brauereiwesen/1");
    await page.getByTestId("open-sidebar").click();
    await expect(page.getByRole("tab", { name: "Search" })).toBeVisible();

    await page.getByRole("tab", { name: "Transcript" }).click();
    await expect(page.getByRole("button", { name: /Welcome to the lecture on brewing\./ }).or(
      page.getByRole("button", { name: /Willkommen zur Vorlesung/ }),
    )).toBeVisible();

    // The tab strip once got squeezed out of the sidebar by a tall transcript; it must
    // still be on screen and every tab must still switch panels.
    await expect(page.getByRole("tab", { name: "Bookmarks" })).toBeInViewport();
    await page.getByRole("tab", { name: "Bookmarks" }).click();
    await expect(page.getByRole("tab", { name: "Bookmarks", selected: true })).toBeVisible();
    await expect(page.getByRole("tabpanel", { name: "Bookmarks" })).toBeVisible();
    await expect(page.getByRole("tabpanel", { name: "Transcript" })).toBeHidden();
    await page.getByRole("tab", { name: "Sections" }).click();
    await expect(page.getByRole("tabpanel", { name: "Sections" })).toBeVisible();
  });

  test("says search is unavailable when there is no search service", async ({ page }) => {
    await page.goto("/w/brauereiwesen/1");
    await page.getByTestId("open-sidebar").click();
    await page.getByRole("tab", { name: "Search" }).click();

    await page.getByPlaceholder("Search in lecture").fill("Hopfen");
    await expect(page.getByRole("status").filter({ hasText: /not available/i })).toBeVisible();
  });

  test("a recording without subtitles has neither tab", async ({ page }) => {
    // Stream 11 is a private recording in the same course; it has no subtitle rows.
    await login(page, users.prof1, "/w/brauereiwesen/11");
    await page.getByTestId("open-sidebar").click();
    await expect(page.getByRole("tab", { name: "Sections" })).toBeVisible();
    await expect(page.getByRole("tab", { name: "Transcript" })).toHaveCount(0);
    await expect(page.getByRole("tab", { name: "Search" })).toHaveCount(0);
  });

  test("keeps the action bar menus inside a phone screen", async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await login(page, users.studi3, "/w/brauereiwesen/1");
    await expect(page.getByTestId("watch-action-bar")).toBeVisible();

    for (const name of ["Change view", "Attachments and downloads"]) {
      const button = page.getByRole("button", { name });
      if ((await button.count()) === 0) continue;
      await button.click();
      const box = await page.getByRole("menu", { name }).boundingBox();
      expect(box, name).not.toBeNull();
      expect(box!.x, `${name} left edge`).toBeGreaterThanOrEqual(0);
      expect(box!.x + box!.width, `${name} right edge`).toBeLessThanOrEqual(390);
      await page.keyboard.press("Escape");
    }
    await expect(page.getByRole("button", { name: "Change view" })).toBeVisible();
  });
});
