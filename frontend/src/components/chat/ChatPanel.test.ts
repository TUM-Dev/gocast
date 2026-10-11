import { fromJson, type JsonValue } from "@bufbuild/protobuf";
import { flushPromises, mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ChatCooldownError, NO_ACCESS, type ChatAccess, type ChatMessage, type ChatPoll } from "@/lib/chat";
import { RealtimeEventSchema } from "@/gen/server/apiv2_pb";
import type { RealtimeOptions } from "@/lib/realtime";
import { useChatStore } from "@/stores/chat";
import ChatPanel from "./ChatPanel.vue";

const {
  fetchChatMessages,
  fetchActivePoll,
  sendMessage,
  toggleReaction,
  vote,
  approveMessage,
  retractMessage,
  deleteMessage,
  resolveMessage,
  startPoll,
  closePoll,
  authState,
  connections,
} = vi.hoisted(() => ({
  fetchChatMessages: vi.fn(),
  fetchActivePoll: vi.fn(),
  sendMessage: vi.fn(),
  toggleReaction: vi.fn(),
  vote: vi.fn(),
  approveMessage: vi.fn(),
  retractMessage: vi.fn(),
  deleteMessage: vi.fn(),
  resolveMessage: vi.fn(),
  startPoll: vi.fn(),
  closePoll: vi.fn(),
  authState: { user: { id: 3 } as { id: number } | null, loaded: true, load: () => Promise.resolve(null) },
  connections: [] as { options: RealtimeOptions; close: () => void }[],
}));

vi.mock("@/lib/chat", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/chat")>()),
  fetchChatMessages,
  fetchActivePoll,
  sendMessage,
  toggleReaction,
  vote,
  approveMessage,
  retractMessage,
  deleteMessage,
  resolveMessage,
  startPoll,
  closePoll,
}));

vi.mock("@/stores/auth", () => ({
  useAuthStore: () => authState,
}));

// The socket is driven by hand: the panel only sees what the store makes of it.
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

const viewersEvent = (viewers: number) =>
  fromJson(RealtimeEventSchema, { streamId: 7, viewers: { viewers } } as JsonValue);

function message(id: number, extra: Partial<ChatMessage> = {}): ChatMessage {
  return {
    id,
    userName: `Author ${id}`,
    color: "#368bd6",
    html: `<p>message ${id}</p>`,
    text: `message ${id}`,
    admin: false,
    visible: true,
    resolved: false,
    addressedTo: [],
    createdAt: new Date(2026, 9, 11, 14, 5),
    reactions: [],
    replies: [],
    ...extra,
  };
}

const poll: ChatPoll = {
  id: 1,
  question: "Weißbier?",
  active: true,
  options: [
    { id: 1, answer: "Ja", votes: 3 },
    { id: 2, answer: "Nein", votes: 1 },
  ],
  votedOptionId: 1,
};

/** Mounts the panel and lets the connection report it has subscribed. */
async function mountPanel(
  messages: ChatMessage[],
  activePoll?: ChatPoll,
  access: ChatAccess = NO_ACCESS,
) {
  fetchChatMessages.mockResolvedValue({ messages, access });
  fetchActivePoll.mockResolvedValue(activePoll);
  const wrapper = mount(ChatPanel, { props: { streamId: 7 } });
  const options = connections.at(-1)!.options;
  options.onStatus?.("open");
  options.onResync?.();
  await flushPromises();
  return { wrapper, options, store: useChatStore(7) };
}

const student: ChatAccess = { canWrite: true, canModerate: false, anonymousAllowed: true, moderated: false };
const admin: ChatAccess = { ...student, canModerate: true, moderated: true };

beforeEach(() => {
  authState.user = { id: 3 };
  setActivePinia(createPinia());
  vi.clearAllMocks();
  connections.length = 0;
});

describe("ChatPanel", () => {
  it("renders messages in the order the store holds them, replies under their parent", async () => {
    const { wrapper } = await mountPanel([
      message(1, { replies: [message(3, { replyTo: 1 }), message(4, { replyTo: 1 })] }),
      message(2),
    ]);

    const ids = wrapper.findAll("[data-message-id]").map((el) => el.attributes("data-message-id"));
    expect(ids).toEqual(["1", "3", "4", "2"]);
    const parent = wrapper.find('[data-message-id="1"]');
    expect(parent.find('[data-message-id="3"]').exists()).toBe(true);
    expect(wrapper.find('[data-message-id="2"]').text()).toContain("message 2");
  });

  it("shows the author as the API names them, anonymous included", async () => {
    const { wrapper } = await mountPanel([message(1, { userName: "Anonymous", userId: undefined })]);
    expect(wrapper.text()).toContain("Anonymous");
  });

  it("summarises reactions and flags administrators and unapproved messages", async () => {
    const { wrapper } = await mountPanel([
      message(1, {
        admin: true,
        reactions: [{ emoji: "tada", count: 2, userIds: [1, 2], userNames: ["A", "B"] }],
      }),
      message(2, { visible: false }),
    ], undefined, { ...NO_ACCESS, canModerate: true });

    expect(wrapper.find('[data-message-id="1"]').text()).toContain("\u{1F389}");
    expect(wrapper.find('[data-message-id="1"]').text()).toContain("Admin");
    expect(wrapper.find('[data-message-id="2"]').text()).toContain("Awaiting approval");
  });

  it("shows the empty state once loaded, not before", async () => {
    fetchChatMessages.mockReturnValue(new Promise(() => {}));
    fetchActivePoll.mockReturnValue(new Promise(() => {}));
    const early = mount(ChatPanel, { props: { streamId: 8 } });
    expect(early.text()).not.toContain("No messages yet.");

    const { wrapper } = await mountPanel([]);
    expect(wrapper.text()).toContain("No messages yet.");
  });

  it("shows the running poll without voting for a viewer who cannot write", async () => {
    const { wrapper } = await mountPanel([], poll);

    expect(wrapper.text()).toContain("Weißbier?");
    expect(wrapper.text()).toContain("(your vote)");
    expect(wrapper.text()).toContain("75%");
    expect(wrapper.find("[data-poll-option] button").exists()).toBe(false);
  });

  it("counts the viewers the socket reports", async () => {
    const { wrapper, options } = await mountPanel([]);
    options.onEvent?.(viewersEvent(1));
    await flushPromises();
    expect(wrapper.get('[data-testid="viewers"]').text()).toBe("1 viewer");
    options.onEvent?.(viewersEvent(5));
    await flushPromises();
    expect(wrapper.get('[data-testid="viewers"]').text()).toBe("5 viewers");
  });

  it("says it is connecting, then reconnecting, then nothing", async () => {
    fetchChatMessages.mockResolvedValue({ messages: [], access: NO_ACCESS });
    fetchActivePoll.mockResolvedValue(undefined);
    const wrapper = mount(ChatPanel, { props: { streamId: 7 } });
    const options = connections.at(-1)!.options;

    options.onStatus?.("connecting");
    await flushPromises();
    expect(wrapper.get('[data-testid="chat-notice"]').text()).toBe("Connecting…");

    options.onStatus?.("open");
    options.onResync?.();
    await flushPromises();
    expect(wrapper.find('[data-testid="chat-notice"]').exists()).toBe(false);

    options.onStatus?.("connecting");
    await flushPromises();
    expect(wrapper.get('[data-testid="chat-notice"]').text()).toContain("Reconnecting");

    options.onStatus?.("open");
    await flushPromises();
    expect(wrapper.find('[data-testid="chat-notice"]').exists()).toBe(false);
  });

  it.each([
    [4403, "not allowed to join"],
    [4404, "does not exist"],
  ])("explains a %i refusal and stops claiming to reconnect", async (code, text) => {
    const { wrapper, options } = await mountPanel([]);

    options.onClose?.(code, "");
    options.onStatus?.("closed");
    options.onFatal?.(code, "refused");
    await flushPromises();

    expect(wrapper.get('[data-testid="chat-notice"]').text()).toContain(text);
    expect(wrapper.text()).not.toContain("Reconnecting");
  });

  it("asks an expired session to sign in while the retries go on", async () => {
    const { wrapper, options } = await mountPanel([]);

    options.onClose?.(4401, "token expired");
    options.onStatus?.("connecting");
    await flushPromises();

    const notice = wrapper.get('[data-testid="chat-notice"]');
    expect(notice.text()).toContain("session has expired");
    expect(notice.get("a").attributes("href")).toBe("/login");
  });

  it("offers a way back to the newest message after scrolling up", async () => {
    const { wrapper, store } = await mountPanel([message(1)]);
    const list = wrapper.get('[data-testid="chat-list"]');
    const el = list.element as HTMLElement;
    Object.defineProperty(el, "scrollHeight", { configurable: true, value: 1000 });
    Object.defineProperty(el, "clientHeight", { configurable: true, value: 300 });

    // Scrolled to the top: the reader is not following.
    el.scrollTop = 0;
    await list.trigger("scroll");
    store.messages.push(message(2));
    await flushPromises();
    expect(wrapper.text()).toContain("1 new · Jump to latest");

    await wrapper.get('[data-testid="jump-latest"]').trigger("click");
    expect(el.scrollTop).toBe(1000);
    expect(wrapper.find('[data-testid="jump-latest"]').exists()).toBe(false);
  });

  it("follows new messages while the reader is at the bottom", async () => {
    const { wrapper, store } = await mountPanel([message(1)]);
    const el = wrapper.get('[data-testid="chat-list"]').element as HTMLElement;
    Object.defineProperty(el, "scrollHeight", { configurable: true, value: 500 });

    store.messages.push(message(2));
    await flushPromises();

    expect(el.scrollTop).toBe(500);
    expect(wrapper.find('[data-testid="jump-latest"]').exists()).toBe(false);
  });

  it("closes the socket when it goes away", async () => {
    const { wrapper } = await mountPanel([]);
    wrapper.unmount();
    expect(connections.at(-1)!.close).toHaveBeenCalled();
  });
});

describe("ChatPanel for a viewer who can write", () => {
  const mountStudent = (messages: ChatMessage[] = [message(1)], activePoll?: ChatPoll) =>
    mountPanel(messages, activePoll, student);

  it("sends the draft with Enter, anonymously when asked, and clears it", async () => {
    sendMessage.mockResolvedValue(message(9));
    const { wrapper } = await mountStudent();
    const box = wrapper.get("textarea");

    await box.setValue("  hello  ");
    await wrapper.get('input[type="checkbox"]').setValue(true);
    await box.trigger("keydown", { key: "Enter" });
    await flushPromises();

    expect(sendMessage).toHaveBeenCalledWith(7, { text: "hello", replyTo: undefined, anonymous: true });
    expect((box.element as HTMLTextAreaElement).value).toBe("");
  });

  it("does not send on Shift+Enter or an empty draft", async () => {
    const { wrapper } = await mountStudent();
    const box = wrapper.get("textarea");

    await box.trigger("keydown", { key: "Enter" });
    await box.setValue("a");
    await box.trigger("keydown", { key: "Enter", shiftKey: true });

    expect(sendMessage).not.toHaveBeenCalled();
  });

  it("offers anonymity only where the course allows it", async () => {
    const { wrapper } = await mountPanel([], undefined, { ...student, anonymousAllowed: false });
    expect(wrapper.find('input[type="checkbox"]').exists()).toBe(false);
  });

  it("answers a message: shows who, sends the reply to it, then stops replying", async () => {
    sendMessage.mockResolvedValue(message(9, { replyTo: 1 }));
    const { wrapper } = await mountStudent();

    await wrapper
      .findAll('[data-message-id="1"] button')
      .find((b) => b.text() === "Reply")!
      .trigger("click");
    expect(wrapper.get('[data-testid="replying-to"]').text()).toContain("Author 1");

    await wrapper.get("textarea").setValue("re");
    await wrapper.get("form").trigger("submit");
    await flushPromises();

    expect(sendMessage).toHaveBeenCalledWith(7, { text: "re", replyTo: 1, anonymous: false });
    expect(wrapper.find('[data-testid="replying-to"]').exists()).toBe(false);
  });

  it("offers no reply on a reply", async () => {
    const { wrapper } = await mountStudent([message(1, { replies: [message(2, { replyTo: 1 })] })]);
    const buttons = wrapper.findAll('[data-message-id="2"] button').map((b) => b.text());
    expect(buttons).not.toContain("Reply");
  });

  it("keeps the draft and pauses sending after a cooldown", async () => {
    vi.useFakeTimers();
    try {
      sendMessage.mockRejectedValueOnce(new ChatCooldownError());
      const { wrapper } = await mountStudent();
      await wrapper.get("textarea").setValue("fast");

      await wrapper.get("form").trigger("submit");
      await flushPromises();

      expect(wrapper.get('[data-testid="composer-error"]').text()).toContain("too fast");
      expect((wrapper.get("textarea").element as HTMLTextAreaElement).value).toBe("fast");
      const send = wrapper.get('button[type="submit"]');
      expect(send.attributes("disabled")).toBeDefined();
      expect(send.text()).toBe("Wait a moment");

      await vi.advanceTimersByTimeAsync(5_000);
      expect(wrapper.get('button[type="submit"]').attributes("disabled")).toBeUndefined();
    } finally {
      vi.useRealTimers();
    }
  });

  it("shows any other refusal and keeps the draft", async () => {
    sendMessage.mockRejectedValueOnce(new Error("message too long"));
    const { wrapper } = await mountStudent();
    await wrapper.get("textarea").setValue("x");

    await wrapper.get("form").trigger("submit");
    await flushPromises();

    expect(wrapper.get('[data-testid="composer-error"]').text()).toBe("message too long");
    expect((wrapper.get("textarea").element as HTMLTextAreaElement).value).toBe("x");
  });

  it("will not send more than the server takes", async () => {
    const { wrapper } = await mountStudent();
    await wrapper.get("textarea").setValue("x".repeat(1001));
    expect(wrapper.get('button[type="submit"]').attributes("disabled")).toBeDefined();
  });

  it("reacts from the picker and by clicking a reaction, marking one's own", async () => {
    toggleReaction.mockResolvedValue([]);
    const { wrapper } = await mountStudent([
      message(1, { reactions: [{ emoji: "tada", count: 1, userIds: [3], userNames: ["Me"] }] }),
    ]);
    const item = wrapper.get('[data-message-id="1"]');

    const chip = item.get('[aria-label="Celebrate, 1"]');
    expect(chip.attributes("aria-pressed")).toBe("true");
    await chip.trigger("click");
    expect(toggleReaction).toHaveBeenCalledWith(7, 1, "tada");

    await item.findAll("button").find((b) => b.text() === "React")!.trigger("click");
    await item.get('[aria-label="Heart"]').trigger("click");
    expect(toggleReaction).toHaveBeenLastCalledWith(7, 1, "heart");
    expect(item.find('[aria-label="Pick a reaction"]').exists()).toBe(false);
  });

  it("lets the viewer vote once, then shows the vote", async () => {
    vote.mockResolvedValue({ ...poll, votedOptionId: 2 });
    const unvoted = { ...poll, votedOptionId: undefined, options: poll.options.map((o) => ({ ...o, votes: 0 })) };
    const { wrapper } = await mountStudent([], unvoted);

    await wrapper.get('[data-poll-option="2"] button').trigger("click");
    await flushPromises();

    expect(vote).toHaveBeenCalledWith(7, 2);
    expect(wrapper.find("[data-poll-option] button").exists()).toBe(false);
    expect(wrapper.get('[data-poll-option="2"]').text()).toContain("(your vote)");
  });

  it("offers no moderation to a student", async () => {
    const { wrapper } = await mountStudent([message(1, { visible: false, userId: 3 })]);
    const text = wrapper.get('[data-message-id="1"]').text();
    for (const word of ["Approve", "Retract", "Delete", "Resolve"]) expect(text).not.toContain(word);
    expect(wrapper.text()).not.toContain("Start a poll");
  });

  it("hides messages awaiting approval from everyone but their author", async () => {
    const { wrapper } = await mountStudent([
      message(1),
      message(2, { visible: false, userId: 8 }),
      message(3, { visible: false, userId: 3 }),
    ]);
    const ids = wrapper.findAll("[data-message-id]").map((el) => el.attributes("data-message-id"));
    expect(ids).toEqual(["1", "3"]);
  });

  it("disables writing once the session ended", async () => {
    const { wrapper, options } = await mountStudent();

    options.onFatal?.(4401, "expired");
    await flushPromises();

    expect(wrapper.get("textarea").attributes("disabled")).toBeDefined();
    expect(wrapper.get("textarea").attributes("placeholder")).toContain("session has expired");
    expect(wrapper.find('[data-message-id="1"] [aria-label="Message actions"]').exists()).toBe(false);
  });

  it("asks a visitor to sign in", async () => {
    authState.user = null;
    const { wrapper } = await mountPanel([message(1)], undefined, NO_ACCESS);

    expect(wrapper.get("textarea").attributes("disabled")).toBeDefined();
    expect(wrapper.get("footer a").attributes("href")).toBe("/login");
  });

  it("says when the chat is switched off for a signed-in user", async () => {
    const { wrapper } = await mountPanel([], undefined, NO_ACCESS);
    expect(wrapper.get("textarea").attributes("placeholder")).toBe("The chat is switched off.");
  });

  it("tells a student the chat is moderated", async () => {
    const { wrapper } = await mountPanel([], undefined, { ...student, moderated: true });
    expect(wrapper.text()).toContain("reviewed by the course's administrators");
  });
});

describe("ChatPanel for a course administrator", () => {
  const mountAdmin = (messages: ChatMessage[], activePoll?: ChatPoll) =>
    mountPanel(messages, activePoll, admin);

  const action = (wrapper: Awaited<ReturnType<typeof mountPanel>>["wrapper"], id: number, label: string) =>
    wrapper.findAll(`[data-message-id="${id}"] button`).find((b) => b.text() === label)!;

  it("lists messages awaiting approval, counts them and approves one", async () => {
    approveMessage.mockResolvedValue(message(2, { visible: true }));
    const { wrapper } = await mountAdmin([message(1), message(2, { visible: false })]);

    expect(wrapper.get('[data-testid="awaiting"]').text()).toBe("1 awaiting approval");
    expect(wrapper.get('[data-message-id="2"]').text()).toContain("Awaiting approval");
    expect(action(wrapper, 1, "Approve")).toBeUndefined();

    await action(wrapper, 2, "Approve").trigger("click");
    await flushPromises();

    expect(approveMessage).toHaveBeenCalledWith(7, 2);
    expect(wrapper.find('[data-testid="awaiting"]').exists()).toBe(false);
    expect(wrapper.get('[data-message-id="2"]').text()).not.toContain("Awaiting approval");
  });

  it("retracts an approved message", async () => {
    retractMessage.mockResolvedValue(message(1, { visible: false }));
    const { wrapper } = await mountAdmin([message(1)]);

    await action(wrapper, 1, "Retract").trigger("click");
    await flushPromises();

    expect(retractMessage).toHaveBeenCalledWith(7, 1);
    expect(wrapper.get('[data-message-id="1"]').text()).toContain("Awaiting approval");
  });

  it("resolves a message", async () => {
    resolveMessage.mockResolvedValue(message(1));
    const { wrapper } = await mountAdmin([message(1)]);

    await action(wrapper, 1, "Resolve").trigger("click");
    await flushPromises();

    expect(wrapper.get('[data-message-id="1"]').text()).toContain("Resolved");
    expect(action(wrapper, 1, "Resolve")).toBeUndefined();
  });

  it("asks before deleting, and deleting removes the message", async () => {
    deleteMessage.mockResolvedValue(undefined);
    const { wrapper } = await mountAdmin([message(1), message(2)]);

    await action(wrapper, 1, "Delete").trigger("click");
    expect(deleteMessage).not.toHaveBeenCalled();
    await action(wrapper, 1, "Cancel").trigger("click");
    expect(action(wrapper, 1, "Delete")).toBeDefined();

    await action(wrapper, 1, "Delete").trigger("click");
    await action(wrapper, 1, "Confirm delete").trigger("click");
    await flushPromises();

    expect(deleteMessage).toHaveBeenCalledWith(7, 1);
    expect(wrapper.findAll("[data-message-id]").map((el) => el.attributes("data-message-id"))).toEqual(["2"]);
  });

  it("shows a refused action and leaves the message alone", async () => {
    deleteMessage.mockRejectedValue(new Error("only the course's administrators can moderate its chat"));
    const { wrapper } = await mountAdmin([message(1)]);

    await action(wrapper, 1, "Delete").trigger("click");
    await action(wrapper, 1, "Confirm delete").trigger("click");
    await flushPromises();

    expect(wrapper.get('[data-testid="action-error"]').text()).toContain("only the course's administrators");
    expect(wrapper.find('[data-message-id="1"]').exists()).toBe(true);
  });

  it("starts a poll from the form, and tells of one that is running already", async () => {
    const { ApiError } = await import("@/lib/api");
    startPoll.mockRejectedValueOnce(new ApiError(409, "conflict"));
    startPoll.mockResolvedValueOnce({ ...poll, votedOptionId: undefined });
    const { wrapper } = await mountAdmin([]);

    await wrapper.findAll("button").find((b) => b.text() === "Start a poll")!.trigger("click");
    const form = wrapper.get('form[aria-label="Start a poll"]');
    await form.get('input[type="text"]').setValue("Weißbier?");
    await form.get('input[aria-label="Answer 1"]').setValue("Ja");
    await form.get('input[aria-label="Answer 2"]').setValue("Nein");

    await form.trigger("submit");
    await flushPromises();
    expect(startPoll).toHaveBeenCalledWith(7, "Weißbier?", ["Ja", "Nein"]);
    expect(wrapper.get('form[aria-label="Start a poll"] [role="alert"]').text()).toContain("running already");

    await form.trigger("submit");
    await flushPromises();
    expect(wrapper.find('form[aria-label="Start a poll"]').exists()).toBe(false);
    expect(wrapper.get("[data-poll-state=open]").text()).toContain("Weißbier?");
  });

  it("closes the running poll and shows its results to dismiss", async () => {
    closePoll.mockResolvedValue({
      ...poll,
      active: false,
      options: [
        { id: 1, answer: "Ja", votes: 5 },
        { id: 2, answer: "Nein", votes: 1 },
      ],
    });
    const { wrapper } = await mountAdmin([], poll);

    await wrapper.findAll("button").find((b) => b.text() === "Close poll")!.trigger("click");
    await flushPromises();

    const results = wrapper.get("[data-poll-state=closed]");
    expect(results.text()).toContain("(closed)");
    expect(results.text()).toContain("5 · 83%");

    await results.get("button").trigger("click");
    expect(wrapper.find("[data-poll-state=closed]").exists()).toBe(false);
  });
});

