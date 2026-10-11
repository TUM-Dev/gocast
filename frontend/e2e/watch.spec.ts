import { expect, test } from "@playwright/test";

import { login } from "./helpers";
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
 */

test.describe("the watch page", () => {
  test("plays a recording for an anonymous visitor of a public course", async ({ page }) => {
    await page.goto("/w/brauereiwesen/1");

    await expect(page.getByTestId("watch-title")).toHaveText("VL 1: Was ist Bier?");
    await expect(page.getByTestId("watch-player")).toBeVisible();
    // The side panel is there for a recording; its chat is off in the fixture, so the
    // tabs a signed-out visitor gets are the sections.
    await expect(page.getByTestId("region-sidebar")).toBeVisible();
    // The page sits in the app: header and footer around it.
    await expect(page.getByRole("banner")).toBeVisible();
  });

  test("switches the layout from the view menu", async ({ page }) => {
    await login(page, users.studi1, "/w/brauereiwesen/1");
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

  test("an unknown version is the not-found page, not a blank one", async ({ page }) => {
    const response = await page.goto("/w/brauereiwesen/1/NOPE");

    expect(response?.status()).toBe(404);
    await expect(page.getByText("This page does not exist.")).toBeVisible();
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
