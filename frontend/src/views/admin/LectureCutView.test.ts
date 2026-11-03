import { create } from "@bufbuild/protobuf";
import { flushPromises, mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { defineComponent, h } from "vue";

import { StreamWatchSchema, WatchViewKind } from "@/gen/server/apiv2_pb";
import { ApiError } from "@/lib/api";
import { LectureCutMode, type Waveform } from "@/lib/lecture-cut";
import TranscodingProgress from "@/components/admin/lectures/content/TranscodingProgress.vue";
import type { OpenCut } from "@/lib/lecture-cut";
import LectureCutView from "./LectureCutView.vue";

const m = vi.hoisted(() => ({
  fetchWatch: vi.fn(),
  fetchWaveform: vi.fn(),
  requestWaveform: vi.fn(),
  pollWaveform: vi.fn(),
  cutLecture: vi.fn(),
  fetchOpenCut: vi.fn(),
  retryCut: vi.fn(),
  abandonCut: vi.fn(),
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
  fetchOpenCut: m.fetchOpenCut,
  retryCut: m.retryCut,
  abandonCut: m.abandonCut,
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

const running = (over: Partial<OpenCut> = {}): OpenCut => ({
  targetStreamId: 7,
  duplicate: false,
  progresses: [{ version: "COMB", progress: 10, failed: false, error: "" }],
  ...over,
});
const failedCut = (over: Partial<OpenCut> = {}): OpenCut =>
  running({ progresses: [{ version: "CAM", progress: 0, failed: true, error: "The video could not be converted." }], ...over });

const mountIt = () => mount(LectureCutView, { global: { stubs } });
const byTestId = (w: ReturnType<typeof mountIt>, id: string) => w.get(`[data-testid=${id}]`);

beforeEach(() => {
  setActivePinia(createPinia());
  m.fetchWatch.mockReset().mockResolvedValue(watchData());
  m.fetchWaveform.mockReset().mockResolvedValue(waveform());
  m.requestWaveform.mockReset().mockResolvedValue(undefined);
  m.pollWaveform.mockReset().mockReturnValue(() => {});
  m.cutLecture.mockReset().mockResolvedValue(7);
  m.fetchOpenCut.mockReset().mockResolvedValue(null);
  m.retryCut.mockReset().mockResolvedValue(undefined);
  m.abandonCut.mockReset().mockResolvedValue([]);
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

  it("moves the edges of a cut from the number fields", async () => {
    const w = mountIt();
    await flushPromises();
    await byTestId(w, "cut-accept-all").trigger("click");

    const start = byTestId(w, "cut-start-input");
    const end = byTestId(w, "cut-end-input");
    expect((start.element as HTMLInputElement).value).toBe("21.5");
    await start.setValue("30");
    await end.setValue("50");
    expect(byTestId(w, "cut-row").text()).toContain("0:30.0 – 0:50.0");

    // past the end of the recording: clamped, and the field shows what the cut is
    await end.setValue("500");
    expect((end.element as HTMLInputElement).value).toBe("100.0");
  });

  it("asks before replacing the recording, then sends the accepted cuts", async () => {
    const w = mountIt();
    await flushPromises();
    await byTestId(w, "cut-accept-all").trigger("click");

    await byTestId(w, "cut-submit").trigger("click");
    expect(m.cutLecture).not.toHaveBeenCalled();
    m.fetchOpenCut.mockResolvedValue(running());
    await byTestId(w, "cut-confirm").trigger("click");
    await flushPromises();

    expect(m.cutLecture).toHaveBeenCalledWith(3, 7, LectureCutMode.OVERWRITE, [{ start: 21.5, end: 58.5 }]);
    // What the server says is under way is what the page shows, in place of the editor.
    expect(byTestId(w, "cut-progress").text()).toContain("Cutting the video");
    expect(w.find("[data-testid=cut-timeline]").exists()).toBe(false);
  });

  it("makes a new lecture without asking", async () => {
    m.cutLecture.mockResolvedValue(8);
    const w = mountIt();
    await flushPromises();
    await byTestId(w, "cut-accept-all").trigger("click");
    await w.findAll("input[type=radio]")[1].setValue(true);
    m.fetchOpenCut.mockResolvedValue(running({ targetStreamId: 8, duplicate: true }));
    await byTestId(w, "cut-submit").trigger("click");
    await flushPromises();

    expect(m.cutLecture).toHaveBeenCalledWith(3, 7, LectureCutMode.DUPLICATE, [{ start: 21.5, end: 58.5 }]);
    expect(w.findComponent(TranscodingProgress).props("lectureId")).toBe(8);
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
    m.cutLecture.mockRejectedValue(new ApiError(409, "the lecture is being converted"));
    const w = mountIt();
    await flushPromises();
    await byTestId(w, "cut-accept-all").trigger("click");
    await w.findAll("input[type=radio]")[1].setValue(true);
    await byTestId(w, "cut-submit").trigger("click");
    await flushPromises();

    expect(w.get("[role=alert]").text()).toContain("The lecture is being converted");
    expect(w.find("[data-testid=cut-progress]").exists()).toBe(false);
  });

  it("refuses versions that differ by more than 60 seconds", async () => {
    m.fetchWaveform.mockResolvedValue(waveform({ versionsDiffer: true, versionDurations: { COMB: 100, CAM: 60 } }));
    const w = mountIt();
    await flushPromises();
    await byTestId(w, "cut-accept-all").trigger("click");

    expect(byTestId(w, "cut-versions-differ").text()).toContain("differ in length by more than 60 seconds");
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

describe("LectureCutView, a cut that is not finished", () => {
  it("shows how far it is instead of the editor", async () => {
    m.fetchOpenCut.mockResolvedValue(running());
    const w = mountIt();
    await flushPromises();

    expect(byTestId(w, "cut-progress").text()).toContain("Cutting the video");
    expect(w.find("[data-testid=cut-timeline]").exists()).toBe(false);
    expect(m.fetchWaveform).not.toHaveBeenCalled();
    expect(w.find("[data-testid=cut-retry]").exists()).toBe(false);
  });

  it("offers to cut the versions that failed again", async () => {
    m.fetchOpenCut.mockResolvedValueOnce(failedCut()).mockResolvedValue(running());
    const w = mountIt();
    await flushPromises();
    expect(byTestId(w, "cut-failed").text()).toContain("only the ones that failed");
    expect(byTestId(w, "cut-progress").text()).toContain("The cut did not finish");

    await byTestId(w, "cut-retry").trigger("click");
    await flushPromises();

    expect(m.retryCut).toHaveBeenCalledWith(3, 7);
    // and then it shows them being cut
    expect(byTestId(w, "cut-progress").text()).toContain("Cutting the video");
  });

  it("shows why a retry was refused and stays on the cut", async () => {
    m.fetchOpenCut.mockResolvedValue(failedCut());
    m.retryCut.mockRejectedValue(new ApiError(503, "no runner is available"));
    const w = mountIt();
    await flushPromises();
    await byTestId(w, "cut-retry").trigger("click");
    await flushPromises();
    expect(w.get("[role=alert]").text()).toContain("No runner is available");
    expect(w.find("[data-testid=cut-failed]").exists()).toBe(true);
  });

  it("gives up a cut after asking, and says what was left uncut", async () => {
    m.fetchOpenCut.mockResolvedValueOnce(failedCut()).mockResolvedValue(null);
    m.abandonCut.mockResolvedValue(["CAM"]);
    const w = mountIt();
    await flushPromises();

    await byTestId(w, "cut-abandon").trigger("click");
    expect(m.abandonCut).not.toHaveBeenCalled();
    expect(w.text()).toContain("can't be put back");
    await byTestId(w, "cut-abandon-confirm").trigger("click");
    await flushPromises();

    expect(m.abandonCut).toHaveBeenCalledWith(3, 7);
    expect(byTestId(w, "cut-notice").text()).toContain("The CAM version was not cut");
    // back at the editor
    expect(w.find("[data-testid=cut-timeline]").exists()).toBe(true);
  });

  it("says that giving up a duplicate removes the new lecture", async () => {
    m.fetchOpenCut.mockResolvedValue(failedCut({ targetStreamId: 8, duplicate: true }));
    const w = mountIt();
    await flushPromises();
    await byTestId(w, "cut-abandon").trigger("click");
    expect(w.text()).toContain("removes the new lecture");
  });

  it("can be kept after all", async () => {
    m.fetchOpenCut.mockResolvedValue(failedCut());
    const w = mountIt();
    await flushPromises();
    await byTestId(w, "cut-abandon").trigger("click");
    await w.findAll("button").find((b) => b.text() === "Keep it")!.trigger("click");
    expect(w.find("[data-testid=cut-abandon-confirm]").exists()).toBe(false);
    expect(m.abandonCut).not.toHaveBeenCalled();
  });

  it("is done once the server has finished it", async () => {
    // all versions are done and the cut is being finished, then it is closed
    m.fetchOpenCut.mockResolvedValueOnce(running({ progresses: [], targetStreamId: 8, duplicate: true })).mockResolvedValue(null);
    const w = mountIt();
    await flushPromises();

    expect(byTestId(w, "cut-progress").text()).toContain("The video is cut.");
    expect(w.find("a[href='/w/eidi/8']").exists()).toBe(true);
  });

  it("settles when the conversion it showed is over", async () => {
    m.fetchOpenCut.mockResolvedValueOnce(running()).mockResolvedValue(null);
    const w = mountIt();
    await flushPromises();
    expect(byTestId(w, "cut-progress").text()).toContain("Cutting the video");

    w.findComponent(TranscodingProgress).vm.$emit("done");
    await flushPromises();
    expect(byTestId(w, "cut-progress").text()).toContain("The video is cut.");
  });

  it("shows what the server has when only some versions could be started", async () => {
    m.cutLecture.mockRejectedValue(new ApiError(503, "only some versions could be started: retry the cut to start the rest"));
    const w = mountIt();
    await flushPromises();
    await byTestId(w, "cut-accept-all").trigger("click");
    await w.findAll("input[type=radio]")[1].setValue(true);
    m.fetchOpenCut.mockResolvedValue(failedCut({ targetStreamId: 8, duplicate: true }));
    await byTestId(w, "cut-submit").trigger("click");
    await flushPromises();

    expect(w.find("[data-testid=cut-failed]").exists()).toBe(true);
    expect(w.get("[role=alert]").text()).toContain("No runner is available");
  });
});
