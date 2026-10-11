import { flushPromises, mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { ChatMessage, ChatPoll } from "@/lib/chat";
import type { RealtimeOptions } from "@/lib/realtime";
import { useChatStore } from "@/stores/chat";
import ChatPanel from "./ChatPanel.vue";

const { fetchChatMessages, fetchActivePoll, connections } = vi.hoisted(() => ({
  fetchChatMessages: vi.fn(),
  fetchActivePoll: vi.fn(),
  connections: [] as { options: RealtimeOptions; close: () => void }[],
}));

vi.mock("@/lib/chat", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/chat")>()),
  fetchChatMessages,
  fetchActivePoll,
}));

// The socket is driven by hand: the panel only sees what the store makes of it.
vi.mock("@/lib/realtime", () => ({
  RealtimeConnection: class {
    connect = vi.fn();
    close = vi.fn();
    constructor(readonly options: RealtimeOptions) {
      connections.push(this);
    }
  },
}));

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
async function mountPanel(messages: ChatMessage[], activePoll?: ChatPoll) {
  fetchChatMessages.mockResolvedValue(messages);
  fetchActivePoll.mockResolvedValue(activePoll);
  const wrapper = mount(ChatPanel, { props: { streamId: 7 } });
  const options = connections.at(-1)!.options;
  options.onStatus?.("open");
  options.onResync?.();
  await flushPromises();
  return { wrapper, options, store: useChatStore(7) };
}

beforeEach(() => {
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
    ]);

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

  it("shows the running poll read-only, with the vote and the shares", async () => {
    const { wrapper } = await mountPanel([], poll);

    expect(wrapper.text()).toContain("Weißbier?");
    expect(wrapper.text()).toContain("(your vote)");
    expect(wrapper.text()).toContain("75%");
    expect(wrapper.find("[data-poll-option] button").exists()).toBe(false);
  });

  it("has no way to send", async () => {
    const { wrapper } = await mountPanel([]);
    expect(wrapper.find("input").attributes("disabled")).toBeDefined();
    expect(wrapper.find("button").exists()).toBe(false);
  });

  it("counts the viewers the socket reports", async () => {
    const { wrapper, store } = await mountPanel([]);
    store.viewers = 1;
    await flushPromises();
    expect(wrapper.get('[data-testid="viewers"]').text()).toBe("1 viewer");
    store.viewers = 5;
    await flushPromises();
    expect(wrapper.get('[data-testid="viewers"]').text()).toBe("5 viewers");
  });

  it("says it is connecting, then reconnecting, then nothing", async () => {
    fetchChatMessages.mockResolvedValue([]);
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

    await wrapper.get("button").trigger("click");
    expect(el.scrollTop).toBe(1000);
    expect(wrapper.find("button").exists()).toBe(false);
  });

  it("follows new messages while the reader is at the bottom", async () => {
    const { wrapper, store } = await mountPanel([message(1)]);
    const el = wrapper.get('[data-testid="chat-list"]').element as HTMLElement;
    Object.defineProperty(el, "scrollHeight", { configurable: true, value: 500 });

    store.messages.push(message(2));
    await flushPromises();

    expect(el.scrollTop).toBe(500);
    expect(wrapper.find("button").exists()).toBe(false);
  });

  it("closes the socket when it goes away", async () => {
    const { wrapper } = await mountPanel([]);
    wrapper.unmount();
    expect(connections.at(-1)!.close).toHaveBeenCalled();
  });
});
