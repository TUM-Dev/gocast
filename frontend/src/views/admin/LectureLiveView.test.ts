import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { flushPromises, mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { defineComponent, h } from "vue";

import { RealtimeEventSchema, StreamWatchSchema, WatchViewKind, type RealtimeEvent } from "@/gen/server/apiv2_pb";
import { ApiError } from "@/lib/api";
import LectureLiveView from "./LectureLiveView.vue";

const m = vi.hoisted(() => ({
  fetchWatch: vi.fn(),
  reactions: vi.fn(),
  courseLoad: vi.fn(),
  seekToLive: vi.fn(),
  push: (() => {}) as (e: RealtimeEvent) => void,
}));
vi.mock("vue-router", () => ({ useRoute: () => ({ params: { courseID: "3", streamID: "7" } }) }));
vi.mock("@/lib/watch", async (orig) => ({ ...(await orig<typeof import("@/lib/watch")>()), fetchStreamWatch: m.fetchWatch }));
vi.mock("@/lib/watch-actions", async (orig) => ({
  ...(await orig<typeof import("@/lib/watch-actions")>()),
  fetchAllowedReactions: m.reactions,
}));
vi.mock("@/stores/streamRealtime", () => ({
  useStreamRealtime: () => ({
    subscribe: (s: { onEvent: (e: RealtimeEvent) => void }) => {
      m.push = s.onEvent;
      return () => {};
    },
  }),
}));
vi.mock("@/stores/auth", () => ({
  useAuthStore: () => ({ load: () => Promise.resolve({ id: 1 }) }),
  redirectToLogin: vi.fn(),
}));
vi.mock("@/stores/course-admin", () => ({
  useCourseAdminStore: () => ({ course: { id: 3, slug: "eidi" }, error: "", load: m.courseLoad }),
}));
vi.mock("@/lib/realtime", () => ({ RealtimeConnection: class { on() {} connect() {} close() {} } }));

const Layout = defineComponent({ setup: (_, { slots }) => () => h("div", slots.default?.()) });
const Player = defineComponent({
  props: { src: String, quality: String, live: Boolean },
  setup(_, { expose }) {
    expose({ seekToLive: m.seekToLive });
    return () => h("div", { class: "player" });
  },
});
const Chat = defineComponent({ props: ["streamId"], setup: () => () => h("div", { class: "chat" }) });
const stubs = {
  AdminLayout: Layout,
  VideoPlayer: Player,
  ChatPanel: Chat,
  RouterLink: { props: ["to"], template: '<a :href="to"><slot/></a>' },
};

function watchData(live = true, over: Record<string, unknown> = {}) {
  return create(StreamWatchSchema, {
    stream: { id: 7, displayName: "Lecture 7", liveNow: live, end: timestampFromDate(new Date(Date.now() + 3_600_000)) },
    course: { id: 3, name: "EIDI", slug: "eidi" },
    views: [{ kind: WatchViewKind.COMBINED, playlistUrl: "/live.m3u8" }],
    chat: { enabled: true },
    viewers: 4,
    admin: { canEndStream: true, cameraPresets: [{ presetId: 1, name: "Board", imageUrl: "/p.jpg" }] },
    ...over,
  });
}

const push = (e: RealtimeEvent) => m.push(e);
const mountIt = () => mount(LectureLiveView, { global: { stubs } });

beforeEach(() => {
  setActivePinia(createPinia());
  m.fetchWatch.mockReset().mockResolvedValue(watchData());
  m.reactions.mockReset().mockResolvedValue(["👍", "👏"]);
  m.courseLoad.mockReset().mockResolvedValue(undefined);
  m.seekToLive.mockReset();
});

describe("LectureLiveView", () => {
  it("shows the live player at the highest quality, viewers, chat, presets", async () => {
    const w = mountIt();
    await flushPromises();
    expect(m.fetchWatch).toHaveBeenCalledWith("eidi", 7);
    expect(w.findComponent(Player).props()).toMatchObject({ src: "/live.m3u8", quality: "highest", live: true });
    expect(w.get("[data-testid=viewers]").text()).toContain("4 watching");
    expect(w.findComponent(Chat).props("streamId")).toBe(7);
    expect(w.find("[data-testid=preset-1]").exists()).toBe(true);
    await w.findAll("button").find((b) => b.text() === "Seek to Live")!.trigger("click");
    expect(m.seekToLive).toHaveBeenCalled();
  });

  it("renders reaction percentages from the stats event and live viewer updates", async () => {
    const w = mountIt();
    await flushPromises();
    expect(w.get("[data-testid='reaction-👍']").text()).toContain("0%");
    push(create(RealtimeEventSchema, { event: { case: "streamReactionStats", value: { shares: [{ reaction: "👍", count: 1, share: 0.25 }, { reaction: "👏", count: 3, share: 0.75 }], windowSeconds: 30 } as never } }));
    push(create(RealtimeEventSchema, { event: { case: "viewers", value: { viewers: 9 } as never } }));
    await flushPromises();
    expect(w.get("[data-testid='reaction-👍']").text()).toContain("25%");
    expect(w.get("[data-testid='reaction-👏']").text()).toContain("75%");
    expect(w.get("[data-testid=viewers]").text()).toContain("9 watching");
  });

  it("says plainly that a lecture that is not live has nothing to manage", async () => {
    m.fetchWatch.mockResolvedValue(watchData(false));
    const w = mountIt();
    await flushPromises();
    expect(w.get("[data-testid=not-live]").text()).toContain("not live");
    expect(w.findComponent(Player).exists()).toBe(false);
  });

  it("reloads when the stream stops being live", async () => {
    const w = mountIt();
    await flushPromises();
    m.fetchWatch.mockResolvedValue(watchData(false));
    push(create(RealtimeEventSchema, { event: { case: "live", value: { live: false } as never } }));
    await flushPromises();
    expect(m.fetchWatch).toHaveBeenCalledTimes(2);
    expect(w.find("[data-testid=not-live]").exists()).toBe(true);
  });

  it("refuses a lecture of another course", async () => {
    m.fetchWatch.mockResolvedValue(watchData(true, { course: { id: 99, name: "Other", slug: "eidi" } }));
    const w = mountIt();
    await flushPromises();
    expect(w.get("[role=alert]").text()).toContain("does not exist or you do not administer");
    expect(w.findComponent(Player).exists()).toBe(false);
  });

  it("reports a refusal", async () => {
    m.fetchWatch.mockRejectedValue(new ApiError(403, "no"));
    const w = mountIt();
    await flushPromises();
    expect(w.find("[role=alert]").exists()).toBe(true);
  });
});
