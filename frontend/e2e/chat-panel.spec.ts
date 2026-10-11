import { expect, test, type APIRequestContext } from "@playwright/test";

import { apiAs, login } from "./helpers";
import { users } from "./seed";

/**
 * The chat on its own page, `/w/:slug/:streamID/chat`.
 *
 * Messages are written through the API; chat-moderation.spec.ts types them into the
 * panel. `/w/:slug/:streamID/chat/popup`, the address the old player's popup button
 * opened, shows the same panel. Stream 8 is a recording in the public godev course,
 * which no other spec uses and whose chat is off in the fixture: the spec turns it on
 * for its run and back off at the end. prof1 administers the course, and
 * administrators are not subject to the message cooldown, so repeated runs do not
 * lock each other out. Every message carries a per-run token and the assertions look
 * for that token only: the chat grows with every run.
 */

const path = "/w/godev/8/chat";
const stream = "/api/v2/courses/3/streams/8";

test.beforeAll(async ({ playwright }) => {
  const prof1 = await apiAs(playwright, users.prof1);
  expect((await prof1.patch(stream, { data: { chat_enabled: true } })).status()).toBe(200);
});

test.afterAll(async ({ playwright }) => {
  const prof1 = await apiAs(playwright, users.prof1);
  await prof1.patch(stream, { data: { chat_enabled: false } });
});

/** Writes as an administrator, who is not subject to the message cooldown. */
async function post(who: APIRequestContext, messages: { text: string }[]): Promise<number[]> {
  const ids: number[] = [];
  for (const data of messages) {
    const response = await who.post("/api/v2/streams/8/chat/messages", { data });
    expect(response.status()).toBe(200);
    ids.push((await response.json()).message.id);
  }
  return ids;
}

test.describe("the chat panel", () => {
  test("lists the messages written before it opened", async ({ page, playwright }) => {
    const token = `panel-${Date.now()}`;
    const prof1 = await apiAs(playwright, users.prof1);
    const ids = await post(prof1, [{ text: `${token} first` }, { text: `${token} second` }]);

    await login(page, users.studi1, path);

    await expect(page.getByRole("heading", { name: "Live chat" })).toBeVisible();
    await expect(page.getByText(`${token} first`)).toBeVisible();
    await expect(page.getByText(`${token} second`)).toBeVisible();
    // In the order they were written, and the author as the server names them.
    const text = await page.getByTestId("chat-list").innerText();
    expect(text.indexOf(`${token} first`)).toBeLessThan(text.indexOf(`${token} second`));
    await expect(page.getByTestId("chat-list").getByText("Peter").first()).toBeVisible();
    // Stream 8's chat is on, so a signed-in student may write (chat-moderation.spec.ts does).
    await expect(page.getByRole("textbox", { name: "Message", exact: true })).toBeEnabled();
    // The panel is the whole page: no application header around it.
    await expect(page.getByRole("banner")).toHaveCount(0);

    for (const id of ids) await prof1.delete(`/api/v2/streams/8/chat/messages/${id}`);
  });

  test("counts the people watching live", async ({ page, browser }) => {
    await login(page, users.studi1, path);
    const viewers = page.getByTestId("viewers");
    await expect(viewers).toHaveText(/^\d+ viewers?$/);
    const before = Number((await viewers.innerText()).split(" ")[0]);

    // Another viewer joins: the first one hears about it without reloading.
    const other = await browser.newPage();
    await login(other, users.studi2, path);
    await expect(other.getByTestId("viewers")).toHaveText(/^\d+ viewers?$/);
    await expect(viewers).toHaveText(`${before + 1} viewers`);

    await other.close();
    await expect(viewers).toHaveText(`${before} ${before === 1 ? "viewer" : "viewers"}`);
  });

  test("sends a visitor who has not signed in to the login page", async ({ page }) => {
    // The server guards the stream before it serves the shell, as it does the watch page.
    await page.goto("/w/games101/7/chat");

    await expect(page).toHaveURL(/\/login/);
  });

  test("refuses a student the course does not admit before the panel loads", async ({ page }) => {
    // Stream 9 is in the enrolled-only bierkunde; studi3 is not enrolled. The same
    // guard the watch page has answers first, so the panel's own "not allowed" notice
    // (a socket closed with 4403 mid-session) is covered by its unit tests instead.
    await login(page, users.studi3, "/w/bierkunde/9/chat");

    await expect(page.getByText("You don't have permissions to access this stream")).toBeVisible();
    await expect(page.getByTestId("chat-list")).toHaveCount(0);
  });

  test("the popup address shows the same panel", async ({ page }) => {
    await login(page, users.studi1, "/w/godev/8/chat/popup");

    await expect(page.getByRole("heading", { name: "Live chat" })).toBeVisible();
    await expect(page.getByTestId("chat-list")).toBeVisible();
    await expect(page.getByRole("banner")).toHaveCount(0);
  });
});
