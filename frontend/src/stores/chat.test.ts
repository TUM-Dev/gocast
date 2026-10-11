import { fromJson, type JsonValue } from "@bufbuild/protobuf";
import { createPinia, setActivePinia } from "pinia";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { RealtimeEventSchema } from "@/gen/server/apiv2_pb";
import type { ChatMessage, ChatPoll } from "@/lib/chat";
import type { RealtimeOptions } from "@/lib/realtime";
import { useChatStore } from "./chat";

const { fetchChatMessages, fetchActivePoll, sendMessage, toggleReaction, vote, connections } =
  vi.hoisted(() => ({
    fetchChatMessages: vi.fn(),
    fetchActivePoll: vi.fn(),
    sendMessage: vi.fn(),
    toggleReaction: vi.fn(),
    vote: vi.fn(),
    connections: [] as { options: RealtimeOptions; connect: () => void; close: () => void }[],
  }));

vi.mock("@/lib/chat", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/chat")>()),
  fetchChatMessages,
  fetchActivePoll,
  sendMessage,
  toggleReaction,
  vote,
}));

// The connection's own behaviour is realtime.test.ts's; here it is only the source of
// events and resyncs, driven by hand.
vi.mock("@/lib/realtime", () => ({
  CLOSE_UNAUTHORIZED: 4401,
  RealtimeConnection: class {
    connect = vi.fn();
    close = vi.fn();
    constructor(readonly options: RealtimeOptions) {
      connections.push(this);
    }
  },
}));

const event = (json: object) =>
  fromJson(RealtimeEventSchema, { streamId: 7, ...json } as JsonValue);

function message(id: number, extra: Partial<ChatMessage> = {}): ChatMessage {
  return {
    id,
    userName: "Studi",
    color: "#368bd6",
    html: `message ${id}`,
    text: `message ${id}`,
    admin: false,
    visible: true,
    resolved: false,
    addressedTo: [],
    createdAt: new Date(0),
    reactions: [],
    replies: [],
    ...extra,
  };
}

const poll: ChatPoll = {
  id: 11,
  question: "Weißbier?",
  active: true,
  options: [
    { id: 21, answer: "Ja", votes: 0 },
    { id: 22, answer: "Nein", votes: 0 },
  ],
};

/** A started store whose connection has subscribed and whose snapshot has landed. */
async function started(
  snapshot: ChatMessage[] = [message(1, { replies: [message(2, { replyTo: 1 })] })],
) {
  fetchChatMessages.mockResolvedValueOnce(snapshot);
  fetchActivePoll.mockResolvedValueOnce(structuredClone(poll));
  const store = useChatStore(7);
  store.start();
  const conn = connections.at(-1)!;
  conn.options.onResync!();
  await vi.waitFor(() => expect(store.loaded).toBe(true));
  return { store, conn };
}

beforeEach(() => {
  setActivePinia(createPinia());
  vi.clearAllMocks();
  connections.length = 0;
});

describe("the chat store", () => {
  it("connects once and loads the snapshot when the connection subscribes", async () => {
    const { store, conn } = await started();
    store.start();

    expect(connections).toHaveLength(1);
    expect(conn.options.streamId).toBe(7);
    expect(conn.connect).toHaveBeenCalledTimes(1);
    expect(fetchChatMessages).toHaveBeenCalledWith(7);
    expect(store.messages.map((m) => m.id)).toEqual([1]);
    expect(store.activePoll?.question).toBe("Weißbier?");
  });

  it("inserts new messages and replies, and upserts an approved one", async () => {
    const { store } = await started();

    store.apply(event({ message: { message: { id: 3, messageHtml: "hi", visible: false } } }));
    store.apply(event({ message: { message: { id: 4, replyTo: 1, messageHtml: "re" } } }));
    store.apply(event({ message: { message: { id: 3, messageHtml: "hi", visible: true } } }));

    expect(store.messages.map((m) => m.id)).toEqual([1, 3]);
    expect(store.messages[1]!.visible).toBe(true);
    expect(store.messages[0]!.replies.map((r) => r.id)).toEqual([2, 4]);
  });

  it("keeps a message's replies when its approval re-sends it without them", async () => {
    const { store } = await started();

    store.apply(event({ message: { message: { id: 1, messageHtml: "approved", visible: true } } }));

    expect(store.messages[0]!.html).toBe("approved");
    expect(store.messages[0]!.replies.map((r) => r.id)).toEqual([2]);
  });

  it("deletes, resolves, retracts and re-reacts", async () => {
    const { store } = await started([
      message(1, { reactions: [{ emoji: "+1", count: 1, userIds: [3], userNames: ["A"] }] }),
      message(5, { replies: [message(6, { replyTo: 5 })] }),
    ]);

    store.apply(event({ messageResolved: { messageId: 1 } }));
    store.apply(
      event({
        reactions: { messageId: 6, reactions: [{ emoji: "tada", count: 1, userIds: [4] }] },
      }),
    );
    expect(store.messages[0]!.resolved).toBe(true);
    expect(store.messages[1]!.replies[0]!.reactions[0]!.emoji).toBe("tada");

    store.apply(event({ messageRetracted: { messageId: 1 } }));
    expect(store.messages[0]).toMatchObject({ visible: false, reactions: [] });

    store.apply(event({ messageDeleted: { messageId: 6 } }));
    expect(store.messages[1]!.replies).toEqual([]);
    store.apply(event({ messageDeleted: { messageId: 5 } }));
    expect(store.messages.map((m) => m.id)).toEqual([1]);
  });

  it("follows a poll from votes to its close", async () => {
    const { store } = await started();

    store.apply(event({ pollVote: { pollOptionId: 21, votes: 3 } }));
    expect(store.activePoll!.options[0]!.votes).toBe(3);

    store.apply(
      event({
        pollClosed: {
          poll: { id: 11, question: "Weißbier?", options: [{ id: 21, answer: "Ja", votes: 5 }] },
        },
      }),
    );
    expect(store.activePoll).toBeUndefined();
    expect(store.closedPoll!.options[0]!.votes).toBe(5);

    store.apply(event({ pollStarted: { poll: { id: 12, question: "Helles?", active: true } } }));
    expect(store.activePoll!.id).toBe(12);
    expect(store.closedPoll).toBeUndefined();
  });

  it("tracks viewers, live state, title and description", async () => {
    const { store } = await started();

    store.apply(event({ viewers: { viewers: 42 } }));
    store.apply(event({ live: { live: true } }));
    store.apply(event({ title: { title: "Hopfen" } }));
    store.apply(event({ description: { html: "<p>Malz</p>" } }));

    expect(store.viewers).toBe(42);
    expect(store.live).toBe(true);
    expect(store.title).toBe("Hopfen");
    expect(store.descriptionHtml).toBe("<p>Malz</p>");
  });

  it("applies events that arrive during a resync on top of the new snapshot", async () => {
    const { store, conn } = await started();

    let land!: (m: ChatMessage[]) => void;
    fetchChatMessages.mockReturnValueOnce(new Promise((resolve) => (land = resolve)));
    fetchActivePoll.mockResolvedValueOnce(undefined);
    conn.options.onResync!();

    // Arrives while the snapshot is in flight; must survive the snapshot replacing
    // the messages.
    conn.options.onEvent!(event({ message: { message: { id: 9, messageHtml: "late" } } }));
    expect(store.messages.map((m) => m.id)).toEqual([1]);

    land([message(1), message(8)]);
    await vi.waitFor(() => expect(store.messages.map((m) => m.id)).toEqual([1, 8, 9]));
    expect(store.activePoll).toBeUndefined();
  });

  it("reports a refused stream and stops", async () => {
    fetchChatMessages.mockRejectedValueOnce(new Error("not allowed"));
    fetchActivePoll.mockResolvedValueOnce(undefined);
    const store = useChatStore(7);
    store.start();

    connections[0]!.options.onFatal!(4403, "User is not eligible");
    await vi.waitFor(() => expect(store.error).toBe("not allowed"));

    store.stop();
    expect(connections[0]!.close).toHaveBeenCalled();
  });

  it("says so when the session ended, and keeps saying so after the snapshot", async () => {
    fetchChatMessages.mockResolvedValueOnce([message(1)]);
    fetchActivePoll.mockResolvedValueOnce(undefined);
    const store = useChatStore(7);
    store.start();

    connections[0]!.options.onFatal!(4401, "token is expired");

    await vi.waitFor(() => expect(store.loaded).toBe(true));
    expect(store.sessionExpired).toBe(true);
    expect(store.error).toBe("your session has expired");
    expect(store.messages.map((m) => m.id)).toEqual([1]); // still readable
  });

  it("does not call a refused stream an expired session", async () => {
    fetchChatMessages.mockRejectedValueOnce(new Error("not allowed"));
    fetchActivePoll.mockResolvedValueOnce(undefined);
    const store = useChatStore(7);
    store.start();

    connections[0]!.options.onFatal!(4403, "User is not eligible");

    await vi.waitFor(() => expect(store.error).toBe("not allowed"));
    expect(store.sessionExpired).toBe(false);
  });

  it("is one store per stream", () => {
    expect(useChatStore(7)).toBe(useChatStore(7));
    expect(useChatStore(7)).not.toBe(useChatStore(8));
  });
});

describe("the chat store's connection state", () => {
  it("remembers why the server hung up until a connection holds", async () => {
    const { store, conn } = await started();

    conn.options.onClose!(4401, "token expired");
    expect(store.lastCloseCode).toBe(4401);

    conn.options.onStatus!("open");
    expect(store.lastCloseCode).toBeUndefined();
  });

  it("keeps the code of a refusal, and still loads what the read allows", async () => {
    fetchChatMessages.mockResolvedValue([]);
    fetchActivePoll.mockResolvedValue(undefined);
    const store = useChatStore(7);
    store.start();

    connections.at(-1)!.options.onFatal!(4404, "stream not found");

    expect(store.fatalCode).toBe(4404);
    expect(store.error).toBe("stream not found");
  });
});

describe("writing", () => {
  it("shows a sent message at once and dedupes the pushed copy by id", async () => {
    const { store } = await started();
    sendMessage.mockResolvedValueOnce(message(5, { text: "mine", userId: 3, userName: "Anonymous" }));

    const sent = await store.sendMessage({ text: "mine" });

    expect(sendMessage).toHaveBeenCalledWith(7, { text: "mine" });
    expect(sent.id).toBe(5);
    expect(store.messages.map((m) => m.id)).toEqual([1, 5]);

    // The hub pushes the same message back, without the anonymous author's id.
    store.apply(event({ message: { message: { id: 5, userName: "Anonymous", messageHtml: "mine" } } }));
    expect(store.messages.map((m) => m.id)).toEqual([1, 5]);
    expect(store.messages[1]!.userId).toBe(3);
  });

  it("does not duplicate a message the socket delivered first", async () => {
    const { store } = await started();
    store.apply(event({ message: { message: { id: 5, messageHtml: "mine" } } }));
    sendMessage.mockResolvedValueOnce(message(5, { text: "mine" }));

    await store.sendMessage({ text: "mine" });

    expect(store.messages.map((m) => m.id)).toEqual([1, 5]);
  });

  it("nests a sent reply under its parent", async () => {
    const { store } = await started();
    sendMessage.mockResolvedValueOnce(message(6, { replyTo: 1 }));

    await store.sendMessage({ text: "re", replyTo: 1 });

    expect(store.messages[0]!.replies.map((r) => r.id)).toEqual([2, 6]);
  });

  it("passes a failed send on and changes nothing", async () => {
    const { store } = await started();
    sendMessage.mockRejectedValueOnce(new Error("too fast"));

    await expect(store.sendMessage({ text: "x" })).rejects.toThrow("too fast");

    expect(store.messages.map((m) => m.id)).toEqual([1]);
    expect(store.error).toBeUndefined();
  });

  it("replaces a message's reactions with the answer", async () => {
    const { store } = await started();
    toggleReaction.mockResolvedValueOnce([{ emoji: "tada", count: 1, userIds: [3], userNames: ["A"] }]);

    await store.toggleReaction(1, "tada");

    expect(toggleReaction).toHaveBeenCalledWith(7, 1, "tada");
    expect(store.messages[0]!.reactions.map((r) => r.emoji)).toEqual(["tada"]);

    toggleReaction.mockResolvedValueOnce([]);
    await store.toggleReaction(1, "tada");
    expect(store.messages[0]!.reactions).toEqual([]);
  });

  it("marks the caller's vote on the running poll", async () => {
    const { store } = await started();
    vote.mockResolvedValueOnce({ ...structuredClone(poll), votedOptionId: 21 });

    await store.vote(21);

    expect(vote).toHaveBeenCalledWith(7, 21);
    expect(store.activePoll?.votedOptionId).toBe(21);
  });

  it("ignores a vote's answer for a poll that has closed meanwhile", async () => {
    const { store } = await started();
    vote.mockImplementationOnce(async () => {
      store.apply(event({ pollClosed: { poll: { id: 11, question: "Weißbier?", active: false } } }));
      return { ...structuredClone(poll), votedOptionId: 21 };
    });

    await store.vote(21);

    expect(store.activePoll).toBeUndefined();
  });
});
