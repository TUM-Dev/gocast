import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { expect, test, type Page } from "@playwright/test";

/**
 * The split view, `/w/brauereiwesen/1/SPLIT` (presentation and camera side by side,
 * one control bar across both). Read-only, so safe in parallel.
 *
 * The fixture's media is on a host the browser tests cannot reach, so the sync specs
 * answer the playlists themselves, per page (`page.route`, which no other spec sees),
 * with the two ten-second streams in fixtures/hls. All three of the stream's views have
 * the same playlist URL in the dump, so the first playlist requested gets the red
 * stream and the second the green one: which player gets which does not matter here, the
 * two only have to stay together.
 */

const split = "/w/brauereiwesen/1/SPLIT";
const HLS_DIR = path.join(path.dirname(fileURLToPath(import.meta.url)), "fixtures", "hls");
const SEGMENT_HOST = "https://hls.e2e.invalid";
const CORS = { "access-control-allow-origin": "*" };

async function serveFixtureStreams(page: Page): Promise<void> {
  let playlists = 0;
  await page.route(/\/playlist\.m3u8/, (route) => {
    const stream = playlists++ % 2 === 0 ? "pres" : "cam";
    const body = fs
      .readFileSync(path.join(HLS_DIR, stream, "index.m3u8"), "utf8")
      .replace(/^(s\d+\.ts)$/gm, `${SEGMENT_HOST}/${stream}/$1`);
    return route.fulfill({ status: 200, contentType: "application/vnd.apple.mpegurl", headers: CORS, body });
  });
  await page.route(`${SEGMENT_HOST}/**`, (route) => {
    const file = path.join(HLS_DIR, new URL(route.request().url()).pathname);
    return route.fulfill({ status: 200, contentType: "video/mp2t", headers: CORS, body: fs.readFileSync(file) });
  });
}

const overflow = (page: Page) =>
  page.evaluate(() => {
    const el = document.scrollingElement!;
    return el.scrollWidth - el.clientWidth;
  });

interface Playback {
  time: number;
  paused: boolean;
  ended: boolean;
  muted: boolean;
}
const video = (page: Page, side: "presentation" | "camera") =>
  page.getByTestId(`split-${side}`).locator("video");
const playback = (page: Page, side: "presentation" | "camera") =>
  video(page, side).evaluate((v: HTMLVideoElement): Playback => ({ time: v.currentTime, paused: v.paused, ended: v.ended, muted: v.muted }));

test.describe("the split view's layout", () => {
  for (const width of [1024, 1280, 1440, 1920]) {
    for (const sidebar of ["closed", "open"]) {
      test(`does not overflow the page at ${width}px with the sidebar ${sidebar}`, async ({ page }) => {
        await page.setViewportSize({ width, height: 900 });
        await page.goto(split);
        await expect(page.getByTestId("split-view")).toBeVisible();
        if (sidebar === "open") {
          await page.getByTestId("open-sidebar").click();
          await expect(page.getByTestId("region-sidebar")).toBeVisible();
        }
        expect(await overflow(page)).toBeLessThanOrEqual(0);

        // The two sides and the divider add up to the row, not to more.
        const box = (id: string) => page.getByTestId(id).boundingBox();
        const view = (await box("split-view"))!;
        const camera = (await box("split-camera"))!;
        expect(camera.x + camera.width).toBeLessThanOrEqual(view.x + view.width + 1);
      });
    }
  }

  test("stacks without overflow on a phone", async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(split);
    await expect(page.getByTestId("split-view")).toBeVisible();

    expect(await overflow(page)).toBeLessThanOrEqual(0);
    const pres = (await page.getByTestId("split-presentation").boundingBox())!;
    const cam = (await page.getByTestId("split-camera").boundingBox())!;
    expect(cam.y).toBeGreaterThanOrEqual(pres.y + pres.height - 1);
    expect(cam.x + cam.width).toBeLessThanOrEqual(391);
    await expect(page.getByTestId("split-gutter")).toBeHidden();
  });

  test("the camera has no controls of its own", async ({ page }) => {
    await page.goto(split);
    await expect(page.getByTestId("split-camera").locator(".video-js")).toBeAttached();
    await expect(page.getByTestId("split-camera").locator(".vjs-control-bar")).toBeHidden();
    await expect(page.getByTestId("split-camera").locator(".vjs-big-play-button")).toBeHidden();
  });
});

test.describe("the split view's playback", () => {
  test.beforeEach(async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await serveFixtureStreams(page);
    await page.goto(split);
    await expect(video(page, "presentation")).toBeAttached();
    await expect.poll(async () => (await video(page, "camera").evaluate((v: HTMLVideoElement) => v.readyState)) >= 1).toBe(true);
    await expect.poll(async () => (await video(page, "presentation").evaluate((v: HTMLVideoElement) => v.readyState)) >= 1).toBe(true);
    await page.getByTestId("split-presentation").locator(".vjs-big-play-button").click();
    await expect.poll(async () => (await playback(page, "presentation")).time).toBeGreaterThan(0.5);
  });

  /** Both videos on `target` seconds (give or take), both still playing. */
  async function expectTogether(page: Page, target: number) {
    await expect
      .poll(
        async () => {
          const [a, b] = [await playback(page, "presentation"), await playback(page, "camera")];
          return Math.abs(a.time - b.time) < 0.5 && Math.abs(a.time - target) < 1.5 && !a.paused && !b.paused && !a.ended && !b.ended;
        },
        { timeout: 10_000 },
      )
      .toBe(true);
  }

  test("has one control bar, across both players, and a muted camera", async ({ page }) => {
    await page.mouse.move(300, 300);
    const bars = page.locator(".vjs-control-bar:visible");
    await expect(bars).toHaveCount(1);
    const bar = (await bars.boundingBox())!;
    const row = (await page.getByTestId("split-view").boundingBox())!;
    expect(Math.abs(bar.width - row.width)).toBeLessThan(4);
    expect(Math.abs(bar.x - row.x)).toBeLessThan(4);
    expect((await playback(page, "camera")).muted).toBe(true);
    expect((await playback(page, "presentation")).muted).toBe(false);
  });

  test("seeks both videos from the shared progress bar", async ({ page }) => {
    await page.mouse.move(300, 300);
    const bar = (await page.getByTestId("split-presentation").locator(".vjs-progress-holder").boundingBox())!;
    // Past the row's width the bar is the pair's, not the presentation's.
    const row = (await page.getByTestId("split-view").boundingBox())!;
    expect(bar.width).toBeGreaterThan(row.width * 0.5);
    await page.mouse.move(bar.x + bar.width * 0.5, bar.y + 1);
    await page.mouse.click(bar.x + bar.width * 0.5, bar.y + 1);
    await expectTogether(page, 5);
  });

  test("seeks both videos from a hotkey", async ({ page }) => {
    await page.mouse.move(300, 300);
    const before = (await playback(page, "presentation")).time;
    await page.keyboard.press("ArrowRight");
    await expect.poll(async () => (await playback(page, "presentation")).time).toBeGreaterThan(before + 2);
    const target = (await playback(page, "presentation")).time;
    await expectTogether(page, target);
  });

  test("keeps the camera muted when the volume or mute changes on the shared bar", async ({ page }) => {
    await page.mouse.move(300, 300);
    const mute = page.locator(".vjs-mute-control:visible");
    await mute.click();
    await expect.poll(async () => (await playback(page, "presentation")).muted).toBe(true);
    await mute.click();
    await expect.poll(async () => (await playback(page, "presentation")).muted).toBe(false);
    expect((await playback(page, "camera")).muted).toBe(true);
    await video(page, "presentation").evaluate((v: HTMLVideoElement) => (v.volume = 0.3));
    await expect.poll(async () => (await playback(page, "camera")).muted).toBe(true);
  });
});
