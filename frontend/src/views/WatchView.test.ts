import { create } from "@bufbuild/protobuf";
import { flushPromises, mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { createMemoryHistory, createRouter } from "vue-router";

import WatchView from "./WatchView.vue";
import { StreamWatchSchema, WatchLayout as L, WatchViewKind as K } from "@/gen/server/apiv2_pb";
import { ApiError } from "@/lib/api";
import { useAuthStore } from "@/stores/auth";

const { fetchStreamWatch, holder } = vi.hoisted(() => ({
  fetchStreamWatch: vi.fn(),
  holder: { chat: undefined as unknown as Record<string, unknown> & { live?: boolean } },
}));

vi.mock("@/lib/watch", async (orig) => ({ ...(await orig<typeof import("@/lib/watch")>()), fetchStreamWatch }));
vi.mock("@/lib/config", () => ({ fetchConfig: () => Promise.resolve({ title: "TUM-Live" }) }));
vi.mock("@/stores/chat", async () => {
  const { reactive } = await import("vue");
  holder.chat = reactive({ start: () => {}, stop: () => {}, live: undefined, title: undefined, descriptionHtml: undefined, viewers: 0 });
  return { useChatStore: () => holder.chat };
});
vi.mock("@/components/chat/ChatPanel.vue", () => ({ default: { template: "<div data-testid='chat' />" } }));
vi.mock("@/components/watch/WatchPlayer.vue", () => ({
  default: { props: ["view", "live", "startTime", "clip"], template: "<div data-testid='player' :data-src='view.playlistUrl' :data-live='live' :data-start='startTime' />" },
}));

function watchData(init: Record<string, unknown> = {}) {
  return create(StreamWatchSchema, {
    stream: { id: 7, displayName: "Intro", recording: true, duration: 1000, descriptionHtml: "<p>Hello</p>" },
    course: { id: 1, name: "Course", slug: "c", year: 2026, term: "W" },
    views: [{ kind: K.COMBINED, playlistUrl: "c.m3u8" }, { kind: K.PRESENTATION, playlistUrl: "p.m3u8" }],
    defaultLayout: L.COMBINED,
    chat: { enabled: true },
    ...init,
  } as never);
}

async function open(path = "/w/c/7") {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: "/w/:slug/:streamID", component: WatchView },
      { path: "/w/:slug/:streamID/:version", component: WatchView },
      { path: "/course/:y/:t/:s", component: { template: "<div/>" } },
    ],
  });
  await router.push(path);
  const wrapper = mount(WatchView, { global: { plugins: [router] } });
  await flushPromises();
  return wrapper;
}

describe("WatchView", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    fetchStreamWatch.mockReset();
    holder.chat.live = undefined;
    useAuthStore().load = vi.fn().mockResolvedValue(null);
    useAuthStore().loaded = true;
  });

  it("plays the recording with the layout's playlist", async () => {
    fetchStreamWatch.mockResolvedValue(watchData());
    const w = await open("/w/c/7/PRES");
    expect(w.get("[data-testid=player]").attributes("data-src")).toBe("p.m3u8");
    expect(w.get("[data-testid=watch-title]").text()).toBe("Intro");
    expect(w.text()).toContain("Hello");
    expect(fetchStreamWatch).toHaveBeenCalledWith("c", 7);
  });

  it("falls back to the default layout and resumes from progress", async () => {
    fetchStreamWatch.mockResolvedValue(watchData({ progress: { progress: 0.5 } }));
    const w = await open("/w/c/7");
    expect(w.get("[data-testid=player]").attributes("data-src")).toBe("c.m3u8");
    expect(w.get("[data-testid=player]").attributes("data-start")).toBe("500");
  });

  it("shows the waiting room before the start", async () => {
    fetchStreamWatch.mockResolvedValue(
      watchData({ stream: { id: 7, displayName: "Intro", isComingUp: true, start: { seconds: BigInt(Math.floor(Date.now() / 1000) + 600) } }, views: [] }),
    );
    const w = await open();
    expect(w.find("[data-testid=waiting-room]").exists()).toBe(true);
    expect(w.find("[data-testid=player]").exists()).toBe(false);
  });

  it("shows live with the badge, and refetches when the live state flips", async () => {
    fetchStreamWatch.mockResolvedValue(watchData({ stream: { id: 7, displayName: "Intro", liveNow: true }, viewers: 3 }));
    const w = await open();
    expect(w.find("[data-testid=live-badge]").exists()).toBe(true);
    expect(w.get("[data-testid=player]").attributes("data-live")).toBe("true");
    expect(w.get("[data-testid=viewers]").text()).toBe("3 watching");

    fetchStreamWatch.mockResolvedValue(watchData());
    holder.chat.live = false;
    await flushPromises();
    expect(fetchStreamWatch).toHaveBeenCalledTimes(2);
    expect(w.get("[data-testid=player]").attributes("data-live")).toBe("false");
  });

  it("shows ended without a recording", async () => {
    fetchStreamWatch.mockResolvedValue(watchData({ stream: { id: 7, displayName: "Intro", ended: true }, views: [] }));
    expect((await open()).find("[data-testid=ended]").exists()).toBe(true);
  });

  it.each([
    [403, "may not watch"],
    [404, "does not exist"],
    [400, "does not exist"],
  ])("handles a %i", async (status, text) => {
    fetchStreamWatch.mockRejectedValue(new ApiError(status, "x"));
    useAuthStore().load = vi.fn().mockResolvedValue({ id: 1 });
    const w = await open();
    expect(w.text()).toContain(text);
  });
});
