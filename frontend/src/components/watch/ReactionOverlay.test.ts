import { mount } from "@vue/test-utils";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { announceOwnReaction, MAX_FLOATS } from "@/lib/reaction-floats";
import type { RealtimeSubscriber } from "@/stores/streamRealtime";

import ReactionOverlay from "./ReactionOverlay.vue";

const hub = vi.hoisted(() => ({
  subscriber: undefined as undefined | RealtimeSubscriber,
  unsubscribe: vi.fn(),
}));

vi.mock("@/stores/streamRealtime", () => ({
  useStreamRealtime: () => ({
    subscribe: (s: RealtimeSubscriber) => {
      hub.subscriber = s;
      return hub.unsubscribe;
    },
  }),
}));

const reaction = (emoji: string) =>
  ({ event: { case: "streamReaction", value: { reaction: emoji } } }) as unknown as Parameters<
    NonNullable<RealtimeSubscriber["onEvent"]>
  >[0];

beforeEach(() => {
  vi.useFakeTimers();
  hub.unsubscribe.mockReset();
});
afterEach(() => vi.useRealTimers());

const floats = (w: ReturnType<typeof mount>) => w.findAll("[data-testid=reaction-float]").map((f) => f.text());

describe("ReactionOverlay", () => {
  it("floats the reactions the stream's socket delivers, and nothing else", async () => {
    const w = mount(ReactionOverlay, { props: { streamId: 3 } });
    hub.subscriber!.onEvent!(reaction("🎉"));
    hub.subscriber!.onEvent!({ event: { case: "viewers", value: { viewers: 4 } } } as never);
    await w.vm.$nextTick();
    expect(floats(w)).toEqual(["🎉"]);
    w.unmount();
  });

  it("is decoration: hidden from assistive technology and transparent to the mouse", () => {
    const w = mount(ReactionOverlay, { props: { streamId: 3 } });
    const root = w.get("[data-testid=reaction-overlay]");
    expect(root.attributes("aria-hidden")).toBe("true");
    expect(root.classes()).toContain("pointer-events-none");
    w.unmount();
  });

  it("shows the viewer's own reaction immediately, once", async () => {
    const w = mount(ReactionOverlay, { props: { streamId: 3 } });
    announceOwnReaction(3, "👍");
    await w.vm.$nextTick();
    expect(floats(w)).toEqual(["👍"]);
    hub.subscriber!.onEvent!(reaction("👍")); // the server's copy
    await w.vm.$nextTick();
    expect(floats(w)).toEqual(["👍"]);
    w.unmount();
  });

  it("ignores another stream's own reactions", async () => {
    const w = mount(ReactionOverlay, { props: { streamId: 3 } });
    announceOwnReaction(4, "👍");
    await w.vm.$nextTick();
    expect(floats(w)).toEqual([]);
    w.unmount();
  });

  it("caps concurrent floats", async () => {
    const w = mount(ReactionOverlay, { props: { streamId: 3 } });
    for (let i = 0; i < MAX_FLOATS * 2; i++) hub.subscriber!.onEvent!(reaction("🎉"));
    await w.vm.$nextTick();
    expect(floats(w)).toHaveLength(MAX_FLOATS);
    w.unmount();
  });

  it("stops listening when unmounted", () => {
    mount(ReactionOverlay, { props: { streamId: 3 } }).unmount();
    expect(hub.unsubscribe).toHaveBeenCalledTimes(1);
  });
});
