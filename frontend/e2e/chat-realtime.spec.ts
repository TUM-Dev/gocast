import { expect, test, type Page } from "@playwright/test";

import { apiAs, bearerToken, login } from "./helpers";
import { users } from "./seed";

/**
 * The chat's reads and the realtime socket, API and socket only: no page uses them
 * yet.
 *
 * Stream 1 is a recording in the public Brauereiwesen; stream 7 is the live lecture
 * of games101, which is for signed-in users; stream 9 is in the enrolled-only
 * bierkunde, where studi1 is enrolled and studi3 is not; stream 11 is a private
 * lecture in Brauereiwesen.
 */

test.describe("reading a stream's chat", () => {
  test("answers a signed-in student on a course for signed-in users", async ({ playwright }) => {
    const studi1 = await apiAs(playwright, users.studi1);
    const response = await studi1.get("/api/v2/streams/7/chat/messages");
    expect(response.status()).toBe(200);
    expect(await response.json()).toEqual(expect.any(Object));
  });

  test("answers an anonymous viewer on a public course", async ({ playwright }) => {
    const anonymous = await apiAs(playwright);
    expect((await anonymous.get("/api/v2/streams/1/chat/messages")).status()).toBe(200);
    expect((await anonymous.get("/api/v2/streams/1/chat/users")).status()).toBe(200);
    // No poll is running: an empty answer, not a 404. The gateway writes the unset
    // field out as null.
    const poll = await anonymous.get("/api/v2/streams/1/chat/polls/active");
    expect(poll.status()).toBe(200);
    expect((await poll.json()).poll ?? null).toBeNull();
  });

  test("refuses whoever may not watch the stream", async ({ playwright }) => {
    const anonymous = await apiAs(playwright);
    expect((await anonymous.get("/api/v2/streams/7/chat/messages")).status()).toBe(403);
    expect((await anonymous.get("/api/v2/streams/11/chat/messages")).status()).toBe(403);

    const studi3 = await apiAs(playwright, users.studi3);
    expect((await studi3.get("/api/v2/streams/9/chat/messages")).status()).toBe(403);

    const studi1 = await apiAs(playwright, users.studi1);
    expect((await studi1.get("/api/v2/streams/9/chat/messages")).status()).toBe(200);
  });

  test("lists past polls to the course's administrators only", async ({ playwright }) => {
    const studi1 = await apiAs(playwright, users.studi1);
    expect((await studi1.get("/api/v2/streams/7/chat/polls")).status()).toBe(403);

    const prof2 = await apiAs(playwright, users.prof2);
    expect((await prof2.get("/api/v2/streams/7/chat/polls")).status()).toBe(200);
  });
});

interface SocketOutcome {
  /** The first frame, parsed, if one arrived. */
  frame?: Record<string, unknown>;
  /** The close code, if the server closed the socket. */
  closed?: number;
}

/**
 * Opens the socket from the page, so the browser sends the page's cookies the way
 * the SPA's would, and reports the first frame or the close, whichever comes first.
 */
async function openSocket(page: Page, stream: string, hello?: object): Promise<SocketOutcome> {
  return page.evaluate(
    ({ stream, hello }) =>
      new Promise<SocketOutcome>((resolve) => {
        const scheme = location.protocol === "https:" ? "wss" : "ws";
        const ws = new WebSocket(`${scheme}://${location.host}/api/v2/realtime?stream=${stream}`);
        ws.onopen = () => {
          if (hello) ws.send(JSON.stringify(hello));
        };
        ws.onmessage = (e) => {
          resolve({ frame: JSON.parse(String(e.data)) });
          ws.close();
        };
        ws.onclose = (e) => resolve({ closed: e.code });
        setTimeout(() => resolve({}), 8000);
      }),
    { stream, hello },
  );
}

test.describe("the realtime socket", () => {
  test("opens on the session cookie and starts with the viewer count", async ({ page }) => {
    await login(page, users.studi1);

    const outcome = await openSocket(page, "7");

    expect(outcome.frame).toMatchObject({ streamId: 7, audience: "AUDIENCE_ALL" });
    expect(outcome.frame).toHaveProperty("viewers");
  });

  test("takes a bearer token in the first frame instead", async ({ page, playwright }) => {
    const token = await bearerToken(await apiAs(playwright, users.studi1));
    expect(token).not.toBeNull();
    await page.goto("/"); // signed out: nothing but the token can open stream 7

    expect((await openSocket(page, "7", { token })).frame).toHaveProperty("viewers");
    expect((await openSocket(page, "7", {})).closed).toBe(4403);
  });

  test("lets an anonymous viewer watch a public course", async ({ page }) => {
    await page.goto("/");
    expect((await openSocket(page, "1", {})).frame).toHaveProperty("viewers");
  });

  test("closes with a code saying why", async ({ page }) => {
    await login(page, users.studi1);

    expect((await openSocket(page, "abc")).closed).toBe(4400);
    expect((await openSocket(page, "999999")).closed).toBe(4404);
    expect((await openSocket(page, "11")).closed).toBe(4403);
  });

  test("pushes a rename to everyone watching", async ({ page, playwright }) => {
    await login(page, users.studi1);
    const prof2 = await apiAs(playwright, users.prof2);
    const lecture = (
      await (await prof2.get("/api/v2/courses/2/lectures/admin")).json()
    ).lectures.find((l: { id: number }) => l.id === 7);

    // Listen first, then rename the lecture to the name it already has: the event
    // goes out all the same, and nothing another spec reads has changed.
    const title = page.evaluate(
      () =>
        new Promise<string>((resolve, reject) => {
          const ws = new WebSocket(`ws://${location.host}/api/v2/realtime?stream=7`);
          ws.onmessage = (e) => {
            const event = JSON.parse(String(e.data));
            if (event.title) resolve(event.title.title);
          };
          ws.onclose = (e) => reject(new Error(`closed ${e.code}`));
          setTimeout(() => reject(new Error("no title event")), 8000);
        }),
    );
    // The socket subscribes on connect; give it a moment before the rename.
    await page.waitForTimeout(500);
    const response = await prof2.patch("/api/v2/courses/2/streams/7", {
      data: { name: lecture.name },
    });
    expect(response.status()).toBe(200);

    expect(await title).toBe(lecture.name);
  });
});
