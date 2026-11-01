import { create } from "@bufbuild/protobuf";
import { flushPromises, mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { defineComponent, h } from "vue";

import { StreamWatchSchema, WatchViewKind } from "@/gen/server/apiv2_pb";
import { ApiError } from "@/lib/api";
import { LectureCutMode, type Waveform } from "@/lib/lecture-cut";
import LectureCutView from "./LectureCutView.vue";

const m = vi.hoisted(() => ({
  fetchWatch: vi.fn(),
  fetchWaveform: vi.fn(),
  requestWaveform: vi.fn(),
  pollWaveform: vi.fn(),
  cutLecture: vi.fn(),
  progress: vi.fn(),
  courseLoad: vi.fn(),
  seek: vi.fn(),
  paused: vi.fn(),
}));
vi.mock("vue-router", () => ({ useRoute: () => ({ params: { courseID: "3", streamID: "7" } }) }));
vi.mock("@/lib/watch", async (orig) => ({ ...(await orig<typeof import("@/lib/watch")>()), fetchStreamWatch: m.fetchWatch }));
vi.mock("@/lib/lecture-content", async (orig) => ({
  ...(await orig<typeof import("@/lib/lecture-content")>()),
  fetchTranscodingProgress: m.progress,
}));
vi.mock("@/lib/lecture-cut", async (orig) => ({
  ...(await orig<typeof import("@/lib/lecture-cut")>()),
  fetchWaveform: m.fetchWaveform,
  requestWaveform: m.requestWaveform,
  pollWaveform: m.pollWaveform,
  cutLecture: m.cutLecture,
}));
vi.mock("@/stores/auth", () => ({
  useAuthStore: () => ({ load: () => Promise.resolve({ id: 1 }) }),
  redirectToLogin: vi.fn(),
}));
vi.mock("@/stores/course-admin", () => ({
  useCourseAdminStore: () => ({ course: { id: 3, name: "EIDI", slug: "eidi" }, error: "", load: m.courseLoad }),
}));

const Layout = defineComponent({ setup: (_, { slots }) => () => h("div", slots.default?.()) });
const Player = defineComponent({
  props: { src: String },
  emits: ["timeupdate"],
  setup(_, { expose }) {
    expose({ seek: m.seek, isPaused: m.paused });
    return () => h("div", { class: "player" });
  },
});
const stubs = {
  AdminLayout: Layout,
  VideoPlayer: Player,
  RouterLink: { props: ["to"], template: '<a :href="to"><slot/></a>' },
};

const waveform = (over: Partial<Waveform> = {}): Waveform => ({
  peaks: new Uint8Array(100).fill(120),
  bucketSeconds: 1,
  versionDurations: { COMB: 100, CAM: 98 },
  versionsDiffer: false,
  silences: [{ $typeName: "protobuf.WatchSilence", start: 20, end: 60 }],
  ...over,
});

function watchData(over: Record<string, unknown> = {}) {
  return create(StreamWatchSchema, {
    stream: { id: 7, displayName: "Lecture 7", liveNow: false },
    course: { id: 3, name: "EIDI", slug: "eidi" },
    views: [
      { kind: WatchViewKind.CAMERA, playlistUrl: "/cam.m3u8" },
      { kind: WatchViewKind.COMBINED, playlistUrl: "/comb.m3u8" },
    ],
    ...over,
  });
}

const mountIt = () => mount(LectureCutView, { global: { stubs } });
const byTestId = (w: ReturnType<typeof mountIt>, id: string) => w.get(`[data-testid=${id}]`);

beforeEach(() => {
  setActivePinia(createPinia());
  m.fetchWatch.mockReset().mockResolvedValue(watchData());
  m.fetchWaveform.mockReset().mockResolvedValue(waveform());
  m.requestWaveform.mockReset().mockResolvedValue(undefined);
  m.pollWaveform.mockReset().mockReturnValue(() => {});
  m.cutLecture.mockReset().mockResolvedValue(7);
  m.progress.mockReset().mockResolvedValue([]);
  m.courseLoad.mockReset().mockResolvedValue(undefined);
  m.seek.mockReset();
  m.paused.mockReset().mockReturnValue(false);
});

describe("LectureCutView", () => {
  it("plays the combined view and proposes the silences without applying them", async () => {
    const w = mountIt();
    await flushPromises();

    expect(m.fetchWatch).toHaveBeenCalledWith("eidi", 7);
    expect(w.findComponent(Player).props("src")).toBe("/comb.m3u8");
    expect(byTestId(w, "cut-suggestions").text()).toContain("1 suggested cut");
    // 20-60 s, with 1.5 s left at either edge
    expect(byTestId(w, "cut-row-suggested").text()).toContain("0:21.5 – 0:58.5");
    expect(byTestId(w, "cut-summary").text()).toContain("Nothing is cut yet");
    expect(byTestId(w, "cut-submit").attributes("disabled")).toBeDefined();
  });

  it("applies a suggestion once it is accepted", async () => {
    const w = mountIt();
    await flushPromises();
    await byTestId(w, "cut-accept-all").trigger("click");

    expect(w.find("[data-testid=cut-suggestions]").exists()).toBe(false);
    expect(byTestId(w, "cut-summary").text()).toContain("Removing 0:37 of 1:40");
    expect(byTestId(w, "cut-summary").text()).toContain("1:03 long");
    expect(byTestId(w, "cut-submit").attributes("disabled")).toBeUndefined();
  });

  it("asks before replacing the recording, then sends the accepted cuts", async () => {
    const w = mountIt();
    await flushPromises();
    await byTestId(w, "cut-accept-all").trigger("click");

    await byTestId(w, "cut-submit").trigger("click");
    expect(m.cutLecture).not.toHaveBeenCalled();
    await byTestId(w, "cut-confirm").trigger("click");
    await flushPromises();

    expect(m.cutLecture).toHaveBeenCalledWith(3, 7, LectureCutMode.OVERWRITE, [{ start: 21.5, end: 58.5 }]);
    expect(w.find("[data-testid=cut-progress]").exists()).toBe(true);
  });

  it("makes a new lecture without asking", async () => {
    m.cutLecture.mockResolvedValue(8);
    const w = mountIt();
    await flushPromises();
    await byTestId(w, "cut-accept-all").trigger("click");
    await w.findAll("input[type=radio]")[1].setValue(true);
    await byTestId(w, "cut-submit").trigger("click");
    await flushPromises();

    expect(m.cutLecture).toHaveBeenCalledWith(3, 7, LectureCutMode.DUPLICATE, [{ start: 21.5, end: 58.5 }]);
    expect(m.progress).toHaveBeenCalledWith(3, 8);
  });

  it("does not send the suggestions that were not accepted", async () => {
    const w = mountIt();
    await flushPromises();
    // Mark one cut by hand, leaving the suggestion alone.
    w.findComponent(Player).vm.$emit("timeupdate", 5);
    await byTestId(w, "cut-start").trigger("click");
    w.findComponent(Player).vm.$emit("timeupdate", 10);
    await byTestId(w, "cut-end").trigger("click");
    await byTestId(w, "cut-submit").trigger("click");
    await byTestId(w, "cut-confirm").trigger("click");
    await flushPromises();

    expect(m.cutLecture).toHaveBeenCalledWith(3, 7, LectureCutMode.OVERWRITE, [{ start: 5, end: 10 }]);
  });

  it("shows why a cut was refused", async () => {
    m.cutLecture.mockRejectedValue(new ApiError(409, "busy"));
    const w = mountIt();
    await flushPromises();
    await byTestId(w, "cut-accept-all").trigger("click");
    await w.findAll("input[type=radio]")[1].setValue(true);
    await byTestId(w, "cut-submit").trigger("click");
    await flushPromises();

    expect(w.get("[role=alert]").text()).toContain("being converted");
    expect(w.find("[data-testid=cut-progress]").exists()).toBe(false);
  });

  it("refuses versions that differ by more than 30 seconds", async () => {
    m.fetchWaveform.mockResolvedValue(waveform({ versionsDiffer: true, versionDurations: { COMB: 100, CAM: 60 } }));
    const w = mountIt();
    await flushPromises();
    await byTestId(w, "cut-accept-all").trigger("click");

    expect(byTestId(w, "cut-versions-differ").text()).toContain("differ in length by more than 30 seconds");
    expect(byTestId(w, "cut-submit").attributes("disabled")).toBeDefined();
  });

  it("builds the timeline when there is none, and waits for it", async () => {
    m.fetchWaveform.mockResolvedValue(null);
    const w = mountIt();
    await flushPromises();

    expect(m.requestWaveform).toHaveBeenCalledWith(3, 7);
    expect(w.text()).toContain("Building the audio timeline");
    expect(w.find("[data-testid=cut-timeline]").exists()).toBe(false);

    m.pollWaveform.mock.calls[0][0].onReady(waveform());
    await flushPromises();
    expect(w.find("[data-testid=cut-timeline]").exists()).toBe(true);
  });

  it("says when the timeline never comes", async () => {
    m.fetchWaveform.mockResolvedValue(null);
    const w = mountIt();
    await flushPromises();
    m.pollWaveform.mock.calls[0][0].onError(new Error("timed out"));
    await flushPromises();
    expect(w.get("[role=alert]").text()).toContain("taking too long");
  });

  it("waits for a conversion that is under way", async () => {
    m.progress.mockResolvedValue([{ version: "COMB", progress: 40, failed: false, error: "" }]);
    const w = mountIt();
    await flushPromises();

    expect(w.text()).toContain("being converted");
    expect(w.find("[data-testid=cut-timeline]").exists()).toBe(false);
    expect(m.fetchWaveform).not.toHaveBeenCalled();
  });

  it("opens the editor after a failed conversion", async () => {
    m.progress.mockResolvedValue([{ version: "COMB", progress: 0, failed: true, error: "boom" }]);
    const w = mountIt();
    await flushPromises();
    expect(w.find("[data-testid=cut-timeline]").exists()).toBe(true);
  });

  it("has nothing to cut in a lecture without a recording", async () => {
    m.fetchWatch.mockResolvedValue(watchData({ views: [] }));
    const w = mountIt();
    await flushPromises();
    expect(w.get("[role=alert]").text()).toContain("no finished recording");
    expect(w.find("[data-testid=cut-timeline]").exists()).toBe(false);
  });

  it("skips the cuts while playing, but not while paused", async () => {
    const w = mountIt();
    await flushPromises();
    await byTestId(w, "cut-accept-all").trigger("click");

    w.findComponent(Player).vm.$emit("timeupdate", 30);
    expect(m.seek).toHaveBeenCalledWith(58.5);

    m.seek.mockReset();
    m.paused.mockReturnValue(true);
    w.findComponent(Player).vm.$emit("timeupdate", 30);
    expect(m.seek).not.toHaveBeenCalled();
  });
});
