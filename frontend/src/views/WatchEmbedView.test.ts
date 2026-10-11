import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { flushPromises, mount } from "@vue/test-utils";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { defineComponent, h, reactive } from "vue";

import { StreamWatchSchema, WatchViewKind } from "@/gen/server/apiv2_pb";
import { ApiError } from "@/lib/api";
import WatchEmbedView from "./WatchEmbedView.vue";

const state = vi.hoisted(() => ({
  route: { params: {} as Record<string, string>, query: {} as Record<string, string> },
  fetch: vi.fn(),
  handlers: {} as Record<string, () => void>,
}));
vi.mock("vue-router", () => ({ useRoute: () => state.route }));
vi.mock("@/lib/watch-admin", () => ({ fetchStreamWatch: state.fetch }));
vi.mock("@/lib/realtime", () => ({
  RealtimeConnection: class {
    on() {}
    connect() {}
    close() {}
  },
}));

const Player = defineComponent({
  props: ["src", "live", "clip", "startTime"],
  setup(_, { expose }) {
    expose({
      on: (e: string, fn: () => void) => {
        state.handlers[e] = fn;
        return () => {};
      },
    });
    return () => h("div", { class: "player" });
  },
});

function watchData(over: Record<string, unknown> = {}) {
  return create(StreamWatchSchema, {
    stream: { id: 5, displayName: "Lecture 1", liveNow: false, recording: true, start: timestampFromDate(new Date(Date.now() + 5 * 60_000)) },
    course: { id: 3, name: "EIDI", slug: "eidi", year: 2026, term: "W" },
    views: [
      { kind: WatchViewKind.COMBINED, playlistUrl: "/comb.m3u8" },
      { kind: WatchViewKind.PRESENTATION, playlistUrl: "/pres.m3u8" },
    ],
    ...over,
  });
}

const mountIt = (props = {}) => mount(WatchEmbedView, { props, global: { stubs: { VideoPlayer: Player } } });

beforeEach(() => {
  Object.assign(state.route, reactive({ params: { slug: "eidi", streamID: "5" }, query: {} }));
  state.fetch.mockReset().mockResolvedValue(watchData());
});

describe("WatchEmbedView", () => {
  it("fetches anonymously-capable data and plays the combined view by default", async () => {
    const w = mountIt();
    await flushPromises();
    expect(state.fetch).toHaveBeenCalledWith("eidi", 5);
    expect(w.findComponent(Player).props("src")).toBe("/comb.m3u8");
  });

  it("plays the view named in the URL, and the cut", async () => {
    state.route.params.version = "PRES";
    state.fetch.mockResolvedValue(watchData({ startOffset: 30, endOffset: 90 }));
    const w = mountIt();
    await flushPromises();
    expect(w.findComponent(Player).props("src")).toBe("/pres.m3u8");
    expect(w.findComponent(Player).props("clip")).toEqual({ start: 30, end: 90 });
  });

  it("uses the parent's data without fetching", async () => {
    const w = mountIt({ watch: watchData() });
    await flushPromises();
    expect(state.fetch).not.toHaveBeenCalled();
    expect(w.findComponent(Player).exists()).toBe(true);
  });

  it("shows the title bar only after playback began, while paused", async () => {
    const w = mountIt();
    await flushPromises();
    expect(w.get("[data-testid=embed-title]").attributes("style")).toContain("display: none");
    state.handlers.play!();
    await w.vm.$nextTick();
    await w.trigger("mousemove");
    expect(w.get("[data-testid=embed-title]").attributes("style") ?? "").not.toContain("display: none");
    expect(w.get("[data-testid=embed-title]").text()).toContain("Lecture 1");
    expect(w.get("[data-testid=embed-title] a").attributes("href")).toBe("/w/eidi/5");
  });

  it("shows when a lecture that has not begun will start, instead of a player", async () => {
    state.fetch.mockResolvedValue(watchData({ stream: { id: 5, displayName: "Soon", recording: false, liveNow: false, start: timestampFromDate(new Date(Date.now() + 5 * 60_000 + 5000)) } }));
    const w = mountIt();
    await flushPromises();
    expect(w.findComponent(Player).exists()).toBe(false);
    expect(w.get("[data-testid=embed-starts-in]").text()).toContain("Start in about 5 minutes");
  });

  it("says so when the video is not available", async () => {
    state.fetch.mockRejectedValue(new ApiError(403, "no"));
    const w = mountIt();
    await flushPromises();
    expect(w.get("[role=alert]").text()).toBe("This video is not available.");
  });
});
