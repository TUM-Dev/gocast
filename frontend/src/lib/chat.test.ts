import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { clearToken } from "./api";
import { fetchActivePoll, fetchChatMessages, fetchChatUsers, fetchPolls } from "./chat";

let fetchMock: ReturnType<typeof vi.fn>;

/** Answers the token request (or refuses it, for a visitor), then the given body. */
function respondWith(body: unknown, { signedIn = true } = {}): void {
  fetchMock.mockImplementation((url: string) => {
    if (String(url).endsWith("/auth/token")) {
      return Promise.resolve(
        signedIn
          ? new Response(JSON.stringify({ access_token: "t", expires_in: 900 }), { status: 200 })
          : new Response("{}", { status: 401 }),
      );
    }
    return Promise.resolve(new Response(JSON.stringify(body), { status: 200 }));
  });
}

const lastCall = () => fetchMock.mock.calls.at(-1) as [string, RequestInit];
const authHeader = () => (lastCall()[1].headers as Record<string, string>).Authorization;

beforeEach(() => {
  clearToken();
  fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("fetchChatMessages", () => {
  it("reads messages with replies, turning the wire's zeros into undefined", async () => {
    respondWith({
      messages: [
        {
          id: 1,
          userName: "Anonymous",
          color: "#368bd6",
          messageHtml: "a &lt; b",
          messageText: "a < b",
          visible: true,
          createdAt: "2026-10-05T10:00:00Z",
          reactions: [{ emoji: "tada", count: 2, userIds: [3, 4], userNames: ["A", "B"] }],
          replies: [
            { id: 2, userId: 9, userName: "Dozent", admin: true, replyTo: 1, visible: true },
          ],
        },
      ],
    });

    const [msg] = await fetchChatMessages(7);

    expect(new URL(lastCall()[0], "http://x").pathname).toBe("/api/v2/streams/7/chat/messages");
    expect(authHeader()).toBe("Bearer t");
    expect(msg).toMatchObject({
      id: 1,
      userId: undefined,
      replyTo: undefined,
      html: "a &lt; b",
      text: "a < b",
      visible: true,
      resolved: false,
      addressedTo: [],
      reactions: [{ emoji: "tada", count: 2, userIds: [3, 4], userNames: ["A", "B"] }],
    });
    expect(msg!.createdAt.toISOString()).toBe("2026-10-05T10:00:00.000Z");
    expect(msg!.replies[0]).toMatchObject({ id: 2, userId: 9, admin: true, replyTo: 1 });
  });

  it("asks without a token when nobody is signed in", async () => {
    respondWith({ messages: [] }, { signedIn: false });

    expect(await fetchChatMessages(7)).toEqual([]);
    expect(authHeader()).toBeUndefined();
  });
});

describe("fetchActivePoll", () => {
  it("is undefined when no poll is running", async () => {
    respondWith({});

    expect(await fetchActivePoll(7)).toBeUndefined();
    expect(new URL(lastCall()[0], "http://x").pathname).toBe("/api/v2/streams/7/chat/polls/active");
  });

  it("reads the poll and the caller's vote", async () => {
    respondWith({
      poll: {
        id: 11,
        question: "Weißbier?",
        active: true,
        options: [{ id: 21, answer: "Ja" }],
        votedOptionId: 21,
      },
    });

    expect(await fetchActivePoll(7)).toEqual({
      id: 11,
      question: "Weißbier?",
      active: true,
      options: [{ id: 21, answer: "Ja", votes: 0 }],
      votedOptionId: 21,
    });
  });
});

describe("fetchPolls", () => {
  it("reads the closed polls", async () => {
    respondWith({
      polls: [{ id: 11, question: "Q", options: [{ id: 21, answer: "Ja", votes: 4 }] }],
    });

    const polls = await fetchPolls(7);

    expect(new URL(lastCall()[0], "http://x").pathname).toBe("/api/v2/streams/7/chat/polls");
    expect(polls[0]).toMatchObject({ id: 11, active: false, votedOptionId: undefined });
    expect(polls[0]!.options[0]!.votes).toBe(4);
  });
});

describe("fetchChatUsers", () => {
  it("reads ids and names", async () => {
    respondWith({ users: [{ id: 3, name: "Studi" }] });

    expect(await fetchChatUsers(7)).toEqual([{ id: 3, name: "Studi" }]);
    expect(new URL(lastCall()[0], "http://x").pathname).toBe("/api/v2/streams/7/chat/users");
  });
});
