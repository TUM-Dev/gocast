import { expect, test, type APIRequestContext, type Page } from "@playwright/test";

import { apiAs, login } from "./helpers";
import { users } from "./seed";

/**
 * The chat's writes, API and sockets only: chat-moderation.spec.ts drives the page.
 *
 * Stream 10 is the live lecture of "geheim" (course 5, hidden: anyone signed in may
 * open it by its address, prof2 administers it); stream 9 is in the enrolled-only
 * bierkunde (course 4), where prof1 administers and studi3 is not enrolled; stream 4
 * is a recording in brauereiwesen whose chat stays off. Nothing else in the suite
 * touches courses 4 and 5 beyond reading them, and the specs that write to stream 4
 * rename it. The fixture has the chat off everywhere, so the spec turns it on for
 * streams 9 and 10 and puts every setting it changes back in afterAll.
 *
 * The cooldown counts a user's messages of the last two minutes in the database, so
 * afterAll also deletes every message the run wrote. A run that crashed before
 * that leaves messages behind, which only the cooldown test tolerates.
 */

const messages = (stream: number) => `/api/v2/streams/${stream}/chat/messages`;

const written = new Set<number>();

/** Posts as `who`, remembering the id for the cleanup. */
async function send(
  who: APIRequestContext,
  stream: number,
  data: { text: string; replyTo?: number; anonymous?: boolean },
) {
  const response = await who.post(messages(stream), { data });
  const body = response.ok() ? await response.json() : undefined;
  if (body?.message?.id) written.add(body.message.id);
  return { response, message: body?.message };
}

async function setStream(who: APIRequestContext, course: number, stream: number, chat: boolean) {
  const r = await who.patch(`/api/v2/courses/${course}/streams/${stream}`, { data: { chat_enabled: chat } });
  expect(r.status()).toBe(200);
}

async function setCourse(who: APIRequestContext, course: number, settings: object) {
  const r = await who.patch(`/api/v2/courses/${course}/settings`, { data: settings });
  expect(r.status()).toBe(200);
}

/** Closes the stream's running poll, if any; a 404 means none was running. */
async function closePoll(who: APIRequestContext, stream: number) {
  await who.post(`/api/v2/streams/${stream}/chat/polls/active/close`, { data: {} });
}

/**
 * Listens to a stream's socket from `page` and returns a reader of the frames so far.
 */
async function listen(page: Page, stream: number) {
  const key = `v2-${stream}`;
  await page.evaluate(
    ({ stream, key }) =>
      new Promise<void>((resolve, reject) => {
        const w = window as unknown as { __frames: Record<string, unknown[]> };
        w.__frames ??= {};
        const frames: unknown[] = (w.__frames[key] = []);
        const ws = new WebSocket(`ws://${location.host}/api/v2/realtime?stream=${stream}`);
        ws.onmessage = (e) => {
          try {
            frames.push(JSON.parse(String(e.data)));
          } catch {
            // not JSON: not ours
          }
          if (frames.length === 1) resolve(); // subscribed: the viewer count came first
        };
        ws.onopen = () => ws.send("{}");
        ws.onclose = (e) => reject(new Error(`closed ${e.code}`));
      }),
    { stream, key },
  );
  return () =>
    page.evaluate((key) => (window as unknown as { __frames: Record<string, unknown[]> }).__frames[key], key);
}

type Frame = Record<string, any>; // eslint-disable-line @typescript-eslint/no-explicit-any

const eventsOf = async (read: () => Promise<unknown[]>, kind: string): Promise<Frame[]> =>
  ((await read()) as Frame[]).filter((f) => f[kind]);

let prof1: APIRequestContext;
let prof2: APIRequestContext;

test.beforeAll(async ({ playwright }) => {
  prof1 = await apiAs(playwright, users.prof1);
  prof2 = await apiAs(playwright, users.prof2);
  await setStream(prof1, 4, 9, true);
  await setStream(prof2, 5, 10, true);
});

test.afterAll(async () => {
  // Cleanup that must not fail the run, and must run whatever failed before it.
  await closePoll(prof2, 10);
  for (const id of written) await prof2.delete(`${messages(10)}/${id}`);
  await setCourse(prof2, 5, { moderated_chat_enabled: false, anonymous_chat_enabled: true });
  await setStream(prof2, 5, 10, false);
  await setStream(prof1, 4, 9, false);
});

test.describe("sendChatMessage", () => {
  test("refuses an anonymous caller", async ({ playwright }) => {
    const anonymous = await apiAs(playwright);
    expect((await anonymous.post(messages(10), { data: { text: "hi" } })).status()).toBe(401);
  });

  test("refuses a user the course does not admit", async ({ playwright }) => {
    const studi3 = await apiAs(playwright, users.studi3); // not enrolled in bierkunde
    expect((await studi3.post(messages(9), { data: { text: "hi" } })).status()).toBe(403);
  });

  test("refuses a stream whose chat is off, and one that does not exist", async ({ playwright }) => {
    const studi1 = await apiAs(playwright, users.studi1);
    expect((await studi1.post(messages(4), { data: { text: "hi" } })).status()).toBe(403);
    expect((await studi1.post(messages(999999), { data: { text: "hi" } })).status()).toBe(404);
  });

  test("stores a message, renders it as a read does and pushes it to the socket", async ({
    browser,
    playwright,
  }) => {
    const watcher = await browser.newPage();
    await login(watcher, users.admin, "/");
    const v2 = await listen(watcher, 10);

    const studi1 = await apiAs(playwright, users.studi1);
    const text = `write-${Date.now()} <b>x</b> https://example.org`;
    const { response, message } = await send(studi1, 10, { text });

    expect(response.status()).toBe(200);
    expect(message).toMatchObject({ userId: 4, userName: "Stephanie Studi", messageText: text, visible: true });
    expect(message.messageHtml).toContain("&lt;b&gt;x&lt;/b&gt;");
    expect(message.messageHtml).toContain('<a href="https://example.org"');

    await expect
      .poll(async () => (await eventsOf(v2, "message")).map((f) => f.message.message.id))
      .toContain(message.id);
    const [pushed] = (await eventsOf(v2, "message")).filter((f) => f.message.message.id === message.id);
    expect(pushed).toMatchObject({ streamId: 10, audience: "AUDIENCE_ALL" });
    expect(pushed.message.message.messageHtml).toBe(message.messageHtml);

    // And a read returns what was pushed.
    const read = await (await studi1.get(messages(10))).json();
    expect(read.messages.find((m: Frame) => m.id === message.id)?.messageHtml).toBe(message.messageHtml);
    await watcher.close();
  });

  test("refuses empty and over-long text", async ({ playwright }) => {
    const studi1 = await apiAs(playwright, users.studi1);
    expect((await send(studi1, 10, { text: "   " })).response.status()).toBe(400);
    expect((await send(studi1, 10, { text: "x".repeat(1001) })).response.status()).toBe(400);
  });

  test("replies nest, and a reply to another stream's message is refused", async ({ playwright }) => {
    const studi1 = await apiAs(playwright, users.studi1);
    const parent = (await send(studi1, 10, { text: `parent-${Date.now()}` })).message;

    const reply = await send(studi1, 10, { text: "reply", replyTo: parent.id });
    expect(reply.response.status()).toBe(200);
    expect(reply.message.replyTo).toBe(parent.id);
    const read = await (await studi1.get(messages(10))).json();
    expect(read.messages.find((m: Frame) => m.id === parent.id).replies.map((r: Frame) => r.id)).toEqual([
      reply.message.id,
    ]);

    // studi1 is enrolled in bierkunde (stream 9), whose chat is on.
    const foreign = await send(studi1, 9, { text: "elsewhere", replyTo: parent.id });
    expect(foreign.response.status()).toBe(400);
    // A reply cannot be replied to, and a message that is not there is a 404.
    expect((await send(studi1, 10, { text: "deeper", replyTo: reply.message.id })).response.status()).toBe(400);
    expect((await send(studi1, 10, { text: "ghost", replyTo: 999999 })).response.status()).toBe(404);
  });

  test("answers 429 to a user who writes too fast, and never to an administrator", async ({ playwright }) => {
    const studi2 = await apiAs(playwright, users.studi2);

    // Five messages in two minutes are allowed. A previous run may have used some of
    // them up, so only the sixth send at the latest has to be refused.
    let refused = 0;
    for (let i = 0; i < 6 && !refused; i++) {
      const { response } = await send(studi2, 10, { text: `fast-${i}-${Date.now()}` });
      if (response.status() === 429) refused = response.status();
      else expect(response.status()).toBe(200);
    }
    expect(refused).toBe(429);

    for (let i = 0; i < 7; i++) {
      expect((await send(prof2, 10, { text: `admin-${i}-${Date.now()}` })).response.status()).toBe(200);
    }
  });

  test("keeps an anonymous author's id out of what is pushed", async ({ browser, playwright }) => {
    const watcher = await browser.newPage();
    await login(watcher, users.admin, "/");
    const v2 = await listen(watcher, 10);

    const studi3 = await apiAs(playwright, users.studi3);
    const { response, message } = await send(studi3, 10, { text: `anon-${Date.now()}`, anonymous: true });

    expect(response.status()).toBe(200);
    expect(message).toMatchObject({ userName: "Anonymous", userId: 6 }); // the author is told
    await expect
      .poll(async () => (await eventsOf(v2, "message")).map((f) => f.message.message.id))
      .toContain(message.id);
    const [pushed] = (await eventsOf(v2, "message")).filter((f) => f.message.message.id === message.id);
    expect(pushed.message.message).toMatchObject({ userName: "Anonymous" });
    expect(pushed.message.message.userId ?? 0).toBe(0);
    await watcher.close();
  });

  test("holds a moderated course's messages for its administrators and their author", async ({
    browser,
    playwright,
  }) => {
    await setCourse(prof2, 5, { moderated_chat_enabled: true });
    const admin = await browser.newPage();
    await login(admin, users.prof2, "/");
    const author = await browser.newPage();
    await login(author, users.studi1, "/");
    const other = await browser.newPage();
    await login(other, users.studi3, "/");
    const [adminSees, authorSees, otherSees] = [
      await listen(admin, 10),
      await listen(author, 10),
      await listen(other, 10),
    ];

    const studi1 = await apiAs(playwright, users.studi1);
    const held = (await send(studi1, 10, { text: `held-${Date.now()}` })).message;
    expect(held.visible).toBe(false);
    // An administrator's message is not held, and doubles as the marker that the
    // held one, sent before it, would have reached the third viewer by now.
    const open = (await send(prof2, 10, { text: `open-${Date.now()}` })).message;

    const ids = async (read: () => Promise<unknown[]>) =>
      (await eventsOf(read, "message")).map((f) => f.message.message.id);
    await expect.poll(() => ids(otherSees)).toContain(open.id);
    expect(await ids(otherSees)).not.toContain(held.id);
    await expect.poll(() => ids(adminSees)).toContain(held.id);
    await expect.poll(() => ids(authorSees)).toContain(held.id);
    const [pushed] = (await eventsOf(adminSees, "message")).filter((f) => f.message.message.id === held.id);
    expect(pushed).toMatchObject({ audience: "AUDIENCE_COURSE_ADMINS", audienceUserId: 4 });
    expect(pushed.message.message.visible ?? false).toBe(false);

    // Reads agree: the author and the administrator list it, a bystander does not.
    const listed = async (who: APIRequestContext) =>
      ((await (await who.get(messages(10))).json()).messages as Frame[]).map((m) => m.id);
    expect(await listed(studi1)).toContain(held.id);
    expect(await listed(prof2)).toContain(held.id);
    expect(await listed(await apiAs(playwright, users.studi3))).not.toContain(held.id);

    // Nobody but an administrator may react to it.
    expect(
      (await (await apiAs(playwright, users.studi3)).post(`${messages(10)}/${held.id}/reactions`, { data: { emoji: "tada" } }))
        .status(),
    ).toBe(404);
    await setCourse(prof2, 5, { moderated_chat_enabled: false });
    await Promise.all([admin.close(), author.close(), other.close()]);
  });

  test("refuses an anonymous message where the course does not allow them", async ({ playwright }) => {
    await setCourse(prof2, 5, { anonymous_chat_enabled: false });
    const studi3 = await apiAs(playwright, users.studi3);

    expect((await send(studi3, 10, { text: "psst", anonymous: true })).response.status()).toBe(400);
    expect((await send(studi3, 10, { text: `named-${Date.now()}` })).response.status()).toBe(200);
    await setCourse(prof2, 5, { anonymous_chat_enabled: true });
  });
});

test.describe("toggleChatReaction", () => {
  test("adds a reaction, takes it back and pushes both", async ({ browser, playwright }) => {
    const studi1 = await apiAs(playwright, users.studi1);
    // An administrator's, who has no cooldown to spend.
    const target = (await send(prof2, 10, { text: `react-${Date.now()}` })).message;
    const watcher = await browser.newPage();
    await login(watcher, users.studi3, "/");
    const v2 = await listen(watcher, 10);
    const path = `${messages(10)}/${target.id}/reactions`;

    const on = await studi1.post(path, { data: { emoji: "tada" } });
    expect(on.status()).toBe(200);
    expect((await on.json()).reactions).toEqual([
      expect.objectContaining({ emoji: "tada", count: 1, userIds: [4] }),
    ]);
    const off = await studi1.post(path, { data: { emoji: "tada" } });
    expect(off.status()).toBe(200);
    expect((await off.json()).reactions ?? []).toEqual([]);

    await expect.poll(async () => (await eventsOf(v2, "reactions")).length).toBe(2);
    const [first, second] = await eventsOf(v2, "reactions");
    expect(first!.reactions).toMatchObject({ messageId: target.id });
    expect(first!.reactions.reactions).toHaveLength(1);
    expect(second!.reactions.reactions ?? []).toHaveLength(0);
    await watcher.close();
  });

  test("refuses an emoji that is not offered, an anonymous caller and a foreign message", async ({
    playwright,
  }) => {
    const studi1 = await apiAs(playwright, users.studi1);
    const target = (await send(prof2, 10, { text: `react2-${Date.now()}` })).message;

    expect((await studi1.post(`${messages(10)}/${target.id}/reactions`, { data: { emoji: "skull" } })).status()).toBe(400);
    const anonymous = await apiAs(playwright);
    expect((await anonymous.post(`${messages(10)}/${target.id}/reactions`, { data: { emoji: "tada" } })).status()).toBe(401);
    // The message is on stream 10, not 9.
    expect((await studi1.post(`${messages(9)}/${target.id}/reactions`, { data: { emoji: "tada" } })).status()).toBe(404);
  });
});

test.describe("votePoll", () => {
  test("counts a vote once, tells only the administrators the tally, and refuses a closed poll", async ({
    browser,
    playwright,
  }) => {
    const admin = await browser.newPage();
    await login(admin, users.prof2, "/");
    const bystander = await browser.newPage();
    await login(bystander, users.studi3, "/");
    const adminSees = await listen(admin, 10);
    const bystanderSees = await listen(bystander, 10);

    const prof2 = await apiAs(playwright, users.prof2);
    await closePoll(prof2, 10);
    expect(
      (
        await prof2.post("/api/v2/streams/10/chat/polls", {
          data: { question: `Poll ${Date.now()}?`, answers: ["Ja", "Nein"] },
        })
      ).status(),
    ).toBe(200);
    const studi1 = await apiAs(playwright, users.studi1);
    const poll = (await (await studi1.get("/api/v2/streams/10/chat/polls/active")).json()).poll;
    expect(poll.options).toHaveLength(2);
    const [yes, no] = poll.options as { id: number }[];

    const voted = await studi1.post("/api/v2/streams/10/chat/polls/votes", { data: { pollOptionId: yes!.id } });
    expect(voted.status()).toBe(200);
    expect((await voted.json()).poll).toMatchObject({ id: poll.id, votedOptionId: yes!.id });

    await expect.poll(async () => (await eventsOf(adminSees, "pollVote")).length).toBe(1);
    expect((await eventsOf(adminSees, "pollVote"))[0]!.pollVote).toMatchObject({ pollOptionId: yes!.id, votes: 1 });
    expect(await eventsOf(bystanderSees, "pollVote")).toEqual([]);

    // Final: neither the same nor the other option counts again. An option of nowhere is a 404.
    expect((await studi1.post("/api/v2/streams/10/chat/polls/votes", { data: { pollOptionId: yes!.id } })).status()).toBe(409);
    expect((await studi1.post("/api/v2/streams/10/chat/polls/votes", { data: { pollOptionId: no!.id } })).status()).toBe(409);
    expect((await studi1.post("/api/v2/streams/10/chat/polls/votes", { data: { pollOptionId: 999999 } })).status()).toBe(404);
    expect((await (await apiAs(playwright)).post("/api/v2/streams/10/chat/polls/votes", { data: { pollOptionId: yes!.id } })).status()).toBe(401);

    await closePoll(prof2, 10);
    const studi3 = await apiAs(playwright, users.studi3);
    expect((await studi3.post("/api/v2/streams/10/chat/polls/votes", { data: { pollOptionId: no!.id } })).status()).toBe(404);
    await Promise.all([admin.close(), bystander.close()]);
  });
});
