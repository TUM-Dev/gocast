import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { clearToken } from "./api";
import {
  approveMessage,
  closePoll,
  deleteMessage,
  resolveMessage,
  retractMessage,
  startPoll,
  ChatCooldownError,
  fetchActivePoll,
  fetchChatMessages,
  NO_ACCESS,
  fetchChatUsers,
  fetchPolls,
  sendMessage,
  toggleReaction,
  vote,
} from "./chat";

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
      access: { canWrite: true, anonymousAllowed: true },
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

    const { messages, access } = await fetchChatMessages(7);
    const [msg] = messages;

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
    expect(access).toEqual({
      canWrite: true,
      canModerate: false,
      anonymousAllowed: true,
      moderated: false,
    });
  });

  it("asks without a token when nobody is signed in", async () => {
    respondWith({ messages: [] }, { signedIn: false });

    expect(await fetchChatMessages(7)).toEqual({ messages: [], access: NO_ACCESS });
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

/** The body of the last request, parsed. */
const lastBody = () => JSON.parse(String(lastCall()[1].body));

describe("sendMessage", () => {
  it("posts the message and returns it as stored", async () => {
    respondWith({
      message: {
        id: 40,
        userId: 3,
        userName: "Anonymous",
        messageHtml: "hi &lt;b&gt;",
        messageText: "hi <b>",
        visible: true,
        replyTo: 30,
        createdAt: "2026-10-11T10:00:00Z",
      },
    });

    const msg = await sendMessage(7, { text: "hi <b>", replyTo: 30, anonymous: true, addressedTo: [9] });

    const [url, init] = lastCall();
    expect(new URL(url, "http://x").pathname).toBe("/api/v2/streams/7/chat/messages");
    expect(init.method).toBe("POST");
    expect(authHeader()).toBe("Bearer t");
    expect(lastBody()).toMatchObject({
      streamId: 7,
      text: "hi <b>",
      replyTo: 30,
      anonymous: true,
      addressedTo: [9],
    });
    expect(msg).toMatchObject({ id: 40, userId: 3, html: "hi &lt;b&gt;", text: "hi <b>", replyTo: 30 });
  });

  it("leaves reply_to out when the message is not a reply", async () => {
    respondWith({ message: { id: 41 } });

    await sendMessage(7, { text: "hi" });

    expect(lastBody()).not.toHaveProperty("replyTo");
  });

  it("turns the cooldown's 429 into a ChatCooldownError", async () => {
    fetchMock.mockImplementation((url: string) =>
      Promise.resolve(
        String(url).endsWith("/auth/token")
          ? new Response(JSON.stringify({ access_token: "t" }), { status: 200 })
          : new Response(JSON.stringify({ code: 8, message: "too fast" }), { status: 429 }),
      ),
    );

    await expect(sendMessage(7, { text: "hi" })).rejects.toBeInstanceOf(ChatCooldownError);
  });

  it("leaves other failures as they are", async () => {
    fetchMock.mockImplementation((url: string) =>
      Promise.resolve(
        String(url).endsWith("/auth/token")
          ? new Response(JSON.stringify({ access_token: "t" }), { status: 200 })
          : new Response(JSON.stringify({ code: 3, message: "message too long" }), { status: 400 }),
      ),
    );

    const err = await sendMessage(7, { text: "hi" }).catch((e: unknown) => e);

    expect(err).not.toBeInstanceOf(ChatCooldownError);
    expect(err).toMatchObject({ status: 400, message: "message too long" });
  });
});

describe("toggleReaction", () => {
  it("posts the emoji to the message and returns its reactions", async () => {
    respondWith({ reactions: [{ emoji: "tada", count: 1, userIds: [3], userNames: ["A"] }] });

    const reactions = await toggleReaction(7, 30, "tada");

    const [url, init] = lastCall();
    expect(new URL(url, "http://x").pathname).toBe("/api/v2/streams/7/chat/messages/30/reactions");
    expect(init.method).toBe("POST");
    expect(lastBody()).toMatchObject({ streamId: 7, messageId: 30, emoji: "tada" });
    expect(reactions).toEqual([{ emoji: "tada", count: 1, userIds: [3], userNames: ["A"] }]);
  });

  it("returns no reactions once the last one is taken back", async () => {
    respondWith({});

    expect(await toggleReaction(7, 30, "tada")).toEqual([]);
  });
});

describe("vote", () => {
  it("posts the option and returns the poll with the caller's vote", async () => {
    respondWith({
      poll: {
        id: 60,
        question: "Q?",
        active: true,
        options: [{ id: 601, answer: "yes" }, { id: 602, answer: "no" }],
        votedOptionId: 601,
      },
    });

    const poll = await vote(7, 601);

    const [url, init] = lastCall();
    expect(new URL(url, "http://x").pathname).toBe("/api/v2/streams/7/chat/polls/votes");
    expect(init.method).toBe("POST");
    expect(lastBody()).toMatchObject({ streamId: 7, pollOptionId: 601 });
    expect(poll).toMatchObject({ id: 60, votedOptionId: 601 });
    expect(poll.options.map((o) => o.id)).toEqual([601, 602]);
  });
});

describe("moderation", () => {
  const path = () => new URL(lastCall()[0], "http://x").pathname;

  it.each([
    ["approve", approveMessage],
    ["retract", retractMessage],
    ["resolve", resolveMessage],
  ] as const)("%s posts to the message and returns it", async (action, call) => {
    respondWith({ message: { id: 30, visible: action === "approve", resolved: action === "resolve" } });

    const msg = await call(7, 30);

    expect(path()).toBe(`/api/v2/streams/7/chat/messages/30/${action}`);
    expect(lastCall()[1].method).toBe("POST");
    expect(lastBody()).toMatchObject({ streamId: 7, messageId: 30 });
    expect(msg.id).toBe(30);
  });

  it("deletes with DELETE", async () => {
    respondWith({});

    await deleteMessage(7, 30);

    expect(path()).toBe("/api/v2/streams/7/chat/messages/30");
    expect(lastCall()[1].method).toBe("DELETE");
  });
});

describe("polls", () => {
  const path = () => new URL(lastCall()[0], "http://x").pathname;
  const reply = {
    poll: { id: 60, question: "Q?", active: true, options: [{ id: 601, answer: "a" }, { id: 602, answer: "b" }] },
  };

  it("starts a poll with its question and answers", async () => {
    respondWith(reply);

    const poll = await startPoll(7, "Q?", ["a", "b"]);

    expect(path()).toBe("/api/v2/streams/7/chat/polls");
    expect(lastBody()).toMatchObject({ streamId: 7, question: "Q?", answers: ["a", "b"] });
    expect(poll).toMatchObject({ id: 60, active: true });
  });

  it("closes the running poll and returns the results", async () => {
    respondWith({ poll: { ...reply.poll, active: false, options: [{ id: 601, answer: "a", votes: 3 }] } });

    const poll = await closePoll(7);

    expect(path()).toBe("/api/v2/streams/7/chat/polls/active/close");
    expect(poll.active).toBe(false);
    expect(poll.options[0]!.votes).toBe(3);
  });
});
