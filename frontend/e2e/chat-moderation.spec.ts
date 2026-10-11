import { expect, test, type APIRequestContext, type Browser, type Page } from "@playwright/test";

import { baseURL } from "../playwright.config";
import { apiAs, login } from "./helpers";
import { users } from "./seed";

/**
 * Moderating and polling in the chat panel, `/w/:slug/:streamID/chat`: an
 * administrator and two students, each in a browser of their own, talking through the
 * real server and its realtime socket.
 *
 * Nothing seeded is touched. The spec creates a course of its own in the test year
 * 1234, with a lecture of its own, so the settings it needs (a moderated chat with
 * anonymous messages) and the messages and polls it leaves cannot meet another spec;
 * soft-deleting the course afterwards is all the cleanup there is. prof1 creates the
 * course and so administers it; studi1 and studi2 see it because it is open to every
 * signed-in user. The cooldown counts a student's messages of the last two minutes, so
 * each student writes a few at most.
 */

const slug = "e2e-chat-moderation";
let courseId = 0;
let streamId = 0;
let path = "";

test.describe.configure({ mode: "serial" });

async function create(who: APIRequestContext): Promise<void> {
  const course = await who.post("/api/v2/courses", {
    data: { name: "E2E Chat Moderation", slug, year: 1234, term: "S", language: "" },
  });
  expect(course.status()).toBe(200);
  courseId = (await course.json()).courseId;

  const settings = await who.patch(`/api/v2/courses/${courseId}/settings`, {
    data: {
      visibility: "loggedin",
      chat_enabled: true,
      moderated_chat_enabled: true,
      anonymous_chat_enabled: true,
    },
  });
  expect(settings.status()).toBe(200);

  const lecture = await who.post(`/api/v2/courses/${courseId}/streams`, {
    data: { title: "E2E Chat", start: "1234-05-02T08:15:00Z", durationMinutes: 90, chatEnabled: true },
  });
  expect(lecture.status()).toBe(200);
  streamId = (await lecture.json()).lectures[0].id;
  path = `/w/${slug}/${streamId}/chat`;
}

test.beforeAll(async ({ playwright }) => {
  await create(await apiAs(playwright, users.prof1));
});

test.afterAll(async ({ playwright }) => {
  if (courseId) await (await apiAs(playwright, users.prof1)).delete(`/api/v2/courses/${courseId}`);
});

/** A signed-in page of its own, with its browser context. */
async function open(browser: Browser, user: (typeof users)[keyof typeof users]): Promise<Page> {
  const context = await browser.newContext({ baseURL });
  const page = await context.newPage();
  await login(page, user, path);
  await expect(box(page)).toBeVisible();
  return page;
}

/** The message box. Its form is named "Write a message", so the label alone is ambiguous. */
const box = (page: Page) => page.getByRole("textbox", { name: "Message", exact: true });

const message = (page: Page, text: string) => page.locator("[data-message-id]", { hasText: text });

async function write(page: Page, text: string): Promise<void> {
  await box(page).fill(text);
  await page.getByRole("button", { name: "Send", exact: true }).click();
}

test("a held message waits for approval, then appears for the other students", async ({ browser }) => {
  const [admin, author, other] = [
    await open(browser, users.prof1),
    await open(browser, users.studi1),
    await open(browser, users.studi2),
  ];
  const text = `held ${Date.now()}`;
  try {
    await write(author, text);

    // The author and the administrator have it; for the author it is held back.
    await expect(message(author, text)).toContainText("Awaiting approval");
    await expect(message(admin, text)).toContainText("Awaiting approval");
    await expect(admin.getByTestId("awaiting")).toHaveText("1 awaiting approval");
    // The author may not moderate it, and the other student does not see it at all.
    await expect(message(author, text).getByRole("button", { name: "Approve" })).toHaveCount(0);
    await expect(message(other, text)).toHaveCount(0);

    await message(admin, text).getByRole("button", { name: "Approve" }).click();

    await expect(message(other, text)).toBeVisible();
    await expect(message(other, text)).not.toContainText("Awaiting approval");
    await expect(message(author, text)).not.toContainText("Awaiting approval");
    await expect(admin.getByTestId("awaiting")).toHaveCount(0);

    // Retracting hides it from the student again, in the open page.
    await message(admin, text).getByRole("button", { name: "Retract" }).click();
    await expect(message(other, text)).toHaveCount(0);
    await expect(message(admin, text)).toContainText("Awaiting approval");
  } finally {
    await Promise.all([admin, author, other].map((p) => p.context().close()));
  }
});

test("a poll runs from start to vote to results", async ({ browser }) => {
  const admin = await open(browser, users.prof1);
  const student = await open(browser, users.studi1);
  const bystander = await open(browser, users.studi2);
  const question = `Beer ${Date.now()}?`;
  try {
    await admin.getByRole("button", { name: "Start a poll" }).click();
    const form = admin.getByRole("form", { name: "Start a poll" });
    await form.getByLabel("Question").fill(question);
    await form.getByLabel("Answer 1").fill("Helles");
    await form.getByLabel("Answer 2").fill("Weissbier");
    await form.getByRole("button", { name: "Start poll" }).click();

    // Everyone sees it at once; the students may vote, nobody sees a tally yet.
    const poll = (p: Page) => p.getByRole("region", { name: "Poll" });
    await expect(poll(student)).toContainText(question);
    await expect(poll(bystander)).toContainText(question);
    await expect(poll(student).locator("button", { hasText: "Helles" })).toBeVisible();
    await expect(poll(student)).not.toContainText("%");

    await poll(student).getByRole("button", { name: /Helles/ }).click();
    await expect(poll(student)).toContainText("(your vote)");
    await expect(poll(student).getByRole("button", { name: /Helles|Weissbier/ })).toHaveCount(0);
    // The administrator sees the tally live.
    await expect(poll(admin)).toContainText("1 · 100%");
    await expect(poll(bystander)).not.toContainText("%");

    // A second poll cannot start beside the first: there is no form for it.
    await expect(admin.getByRole("button", { name: "Start a poll" })).toHaveCount(0);

    await poll(admin).getByRole("button", { name: "Close poll" }).click();

    // Closing shows everyone the results.
    for (const page of [student, bystander, admin]) {
      const closed = page.locator("[data-poll-state=closed]");
      await expect(closed).toContainText("(closed)");
      await expect(closed).toContainText("Helles");
      await expect(closed).toContainText("1 · 100%");
    }
    await expect(student.locator("[data-poll-state=open]")).toHaveCount(0);
  } finally {
    await Promise.all([admin, student, bystander].map((p) => p.context().close()));
  }
});

test("an administrator resolves and deletes a message for everyone", async ({ browser }) => {
  const admin = await open(browser, users.prof1);
  const student = await open(browser, users.studi2);
  const text = `question ${Date.now()}`;
  try {
    // An administrator's own messages are not held.
    await write(admin, text);
    await expect(message(student, text)).toBeVisible();

    await message(admin, text).getByRole("button", { name: "Resolve" }).click();
    await expect(message(student, text)).toContainText("Resolved");
    await expect(message(admin, text).getByRole("button", { name: "Resolve" })).toHaveCount(0);

    // Deleting asks first.
    await message(admin, text).getByRole("button", { name: "Delete" }).click();
    await expect(message(student, text)).toBeVisible();
    await message(admin, text).getByRole("button", { name: "Confirm delete" }).click();

    await expect(message(student, text)).toHaveCount(0);
    await expect(message(admin, text)).toHaveCount(0);

    // And it stays gone.
    await student.reload();
    await expect(box(student)).toBeVisible();
    await expect(message(student, text)).toHaveCount(0);
  } finally {
    await Promise.all([admin, student].map((p) => p.context().close()));
  }
});

test("students cannot moderate through the API", async ({ playwright }) => {
  const studi1 = await apiAs(playwright, users.studi1);
  const base = `/api/v2/streams/${streamId}/chat`;

  expect((await studi1.post(`${base}/messages/1/approve`, { data: {} })).status()).toBe(403);
  expect((await studi1.delete(`${base}/messages/1`)).status()).toBe(403);
  expect((await studi1.post(`${base}/polls`, { data: { question: "q", answers: ["a", "b"] } })).status()).toBe(403);
  expect((await studi1.post(`${base}/polls/active/close`, { data: {} })).status()).toBe(403);
});

test("an administrator of another course cannot moderate this one", async ({ playwright }) => {
  // prof2 administers courses 1 and 2, not the one this spec created.
  const prof2 = await apiAs(playwright, users.prof2);

  const response = await prof2.post(`/api/v2/streams/${streamId}/chat/polls`, {
    data: { question: "q", answers: ["a", "b"] },
  });

  expect(response.status()).toBe(403);
});
