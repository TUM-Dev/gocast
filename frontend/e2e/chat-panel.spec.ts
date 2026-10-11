import { expect, test, type Page } from "@playwright/test";

import { apiAs, login } from "./helpers";
import { users } from "./seed";

/**
 * The read-only chat on its own page, `/w/:slug/:streamID/chat`.
 *
 * Messages are written the way the old UI writes them, over the v1 websocket, since
 * nothing in the SPA sends yet. Stream 8 is a recording in the public godev course,
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

/** Posts through the v1 chat socket from `page`, which has to be signed in already. */
async function postV1(page: Page, messages: { msg: string; replyTo?: number }[]): Promise<void> {
  await page.evaluate(
    (messages) =>
      new Promise<void>((resolve, reject) => {
        const scheme = location.protocol === "https:" ? "wss" : "ws";
        const ws = new WebSocket(`${scheme}://${location.host}/api/pub-sub/ws`);
        ws.onopen = () => {
          ws.send(JSON.stringify({ type: "subscribe", channel: "chat/8" }));
          for (const m of messages) {
            ws.send(
              JSON.stringify({
                type: "message",
                channel: "chat/8",
                payload: { type: "message", ...m },
              }),
            );
          }
          // The socket has no acknowledgement; give the server a moment to write them.
          setTimeout(() => {
            ws.close();
            resolve();
          }, 1000);
        };
        ws.onerror = () => reject(new Error("v1 chat socket failed"));
      }),
    messages,
  );
}

test.describe("the chat panel", () => {
  test("lists the messages written before it opened, and is read-only", async ({ page, browser }) => {
    const token = `panel-${Date.now()}`;
    const poster = await browser.newPage();
    await login(poster, users.prof1);
    await postV1(poster, [{ msg: `${token} first` }, { msg: `${token} second` }]);
    await poster.close();

    await login(page, users.studi1, path);

    await expect(page.getByRole("heading", { name: "Live chat" })).toBeVisible();
    await expect(page.getByText(`${token} first`)).toBeVisible();
    await expect(page.getByText(`${token} second`)).toBeVisible();
    // In the order they were written, and the author as the server names them.
    const text = await page.getByTestId("chat-list").innerText();
    expect(text.indexOf(`${token} first`)).toBeLessThan(text.indexOf(`${token} second`));
    await expect(page.getByTestId("chat-list").getByText("Peter").first()).toBeVisible();
    await expect(page.getByLabel("Message")).toBeDisabled();
    // The panel is the whole page: no application header around it.
    await expect(page.getByRole("banner")).toHaveCount(0);
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

  test("does not take over the popup's address", async ({ page }) => {
    await login(page, users.studi1, "/w/godev/8/chat/popup");

    // Still the server-rendered popup, which has no "Live chat" heading of the SPA's.
    await expect(page.getByTestId("chat-list")).toHaveCount(0);
  });
});
