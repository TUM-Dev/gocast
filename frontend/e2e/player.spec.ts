import { expect, test, type Page } from "@playwright/test";

import { login } from "./helpers";
import { users } from "./seed";

/**
 * The watch page's player chrome and the dialogs that open over it (#2060). Read-only,
 * so safe in parallel.
 *
 * The fixture's media is on a host the browser tests cannot reach, so everything here
 * asserts on the DOM video.js renders without any media. Captions are covered by the
 * subtitle spec that comes with the subtitle fixtures (#2061), not here.
 */

const recording = "/w/brauereiwesen/1";

async function openPlayer(page: Page, to = recording): Promise<void> {
  await page.goto(to);
  await expect(page.getByTestId("watch-player")).toBeVisible();
  await expect(page.locator(".video-js .vjs-control-bar")).toBeAttached();
}

/**
 * What `--player-accent` resolves to for the current theme, as a computed colour. Asked
 * of the browser through a probe inside the player rather than hard-coded, so the test
 * pins "the skin uses the accent player.css defines" and not one shade of it.
 */
async function accent(page: Page): Promise<string> {
  return page.locator(".video-js").evaluate((player) => {
    const probe = document.createElement("span");
    probe.style.backgroundColor = "var(--player-accent)";
    player.appendChild(probe);
    const colour = getComputedStyle(probe).backgroundColor;
    probe.remove();
    return colour;
  });
}

const background = (page: Page, selector: string) =>
  page.locator(selector).first().evaluate((el) => getComputedStyle(el).backgroundColor);

test.describe("the player's controls", () => {
  test("offers no quality selector", async ({ page }) => {
    await openPlayer(page);

    await expect(page.getByTestId("highest-quality")).toHaveCount(0);
    await expect(page.getByText(/highest quality/i)).toHaveCount(0);
    await expect(page.locator(".video-js [class*='quality' i]")).toHaveCount(0);
  });
});

test.describe("the player's skin", () => {
  for (const scheme of ["light", "dark"] as const) {
    test(`uses the brand accent in the ${scheme} theme`, async ({ page }) => {
      await page.emulateMedia({ colorScheme: scheme });
      await openPlayer(page);
      await expect(page.locator("html")).toHaveClass(scheme === "dark" ? /\bdark\b/ : /^(?!.*\bdark\b)/);

      const expected = await accent(page);
      // Not video.js' own white/grey, and not unset.
      expect(expected).not.toBe("");
      expect(expected).not.toBe("rgba(0, 0, 0, 0)");

      await expect.poll(() => background(page, ".video-js .vjs-play-progress")).toBe(expected);
      await expect.poll(() => background(page, ".video-js .vjs-volume-level")).toBe(expected);
      await expect.poll(() => background(page, ".video-js .vjs-big-play-button")).toBe(expected);
    });
  }

  test("the accent differs between the themes", async ({ page }) => {
    await page.emulateMedia({ colorScheme: "light" });
    await openPlayer(page);
    const light = await accent(page);

    await page.emulateMedia({ colorScheme: "dark" });
    await expect(page.locator("html")).toHaveClass(/\bdark\b/);
    await expect.poll(() => accent(page)).not.toBe(light);
  });
});

test.describe("dialogs over the player", () => {
  /** Whether the element at the player's centre is inside `dialog`. */
  async function coversPlayer(page: Page, dialog: string): Promise<boolean> {
    const player = await page.locator(".video-js").first().boundingBox();
    expect(player).not.toBeNull();
    return page.evaluate(
      ({ x, y, dialog }) => !!document.elementFromPoint(x, y)?.closest(dialog),
      { x: player!.x + player!.width / 2, y: player!.y + player!.height / 2, dialog },
    );
  }

  test("the keyboard shortcuts dialog sits above the player, under <body>", async ({ page }) => {
    await openPlayer(page);
    await page.getByRole("button", { name: "Keyboard shortcuts" }).click();

    const dialog = page.getByRole("dialog", { name: "Keyboard shortcuts" });
    await expect(dialog).toBeVisible();
    // The dialog is its own container, so it is itself the child of <body>.
    await expect(page.locator('body > [role="dialog"][aria-label="Keyboard shortcuts"]')).toHaveCount(1);
    await expect.poll(() => coversPlayer(page, '[aria-label="Keyboard shortcuts"][role="dialog"]')).toBe(true);
  });

  test("the share dialog sits above the player, under <body>", async ({ page }) => {
    await openPlayer(page);
    await page.getByRole("button", { name: "Share", exact: true }).click();

    const dialog = page.getByRole("dialog", { name: "Share" });
    await expect(dialog).toBeVisible();
    await expect(page.locator("body > [data-testid='action-dialog']")).toHaveCount(1);
    await expect.poll(() => coversPlayer(page, "[data-testid='action-dialog']")).toBe(true);
  });

  // Stream 7 is the live lecture of games101; prof2 administers it and may end it.
  // The dialog is only opened and cancelled: nothing is ended.
  test("the end-stream dialog sits above the player, under <body>", async ({ page }) => {
    await login(page, users.prof2, "/w/games101/7");
    await expect(page.getByTestId("watch-player")).toBeVisible();
    await expect(page.locator(".video-js .vjs-control-bar")).toBeAttached();
    await page.getByRole("button", { name: "End stream" }).click();

    const dialog = page.getByRole("dialog", { name: "Keep the recording after ending the stream?" });
    await expect(dialog).toBeVisible();
    await expect(page.locator("body > div").filter({ has: dialog })).toHaveCount(1);
    await expect
      .poll(() => coversPlayer(page, '[aria-labelledby="end-stream-title"]'))
      .toBe(true);
    await page.getByRole("button", { name: "Cancel" }).click();
    await expect(dialog).toBeHidden();
  });
});
