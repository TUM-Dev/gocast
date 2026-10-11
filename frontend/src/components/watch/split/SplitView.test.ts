import { mount } from "@vue/test-utils";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { defineComponent, h } from "vue";

import { WatchViewKind } from "@/gen/server/apiv2_pb";
import { RATIO_KEY } from "./ratio";
import SplitView from "./SplitView.vue";

const patch = vi.hoisted(() => vi.fn().mockResolvedValue({}));
vi.mock("@/lib/api", () => ({ apiPatchMessage: patch }));
const sync = vi.hoisted(() => vi.fn(() => vi.fn()));
vi.mock("./sync", () => ({ syncPlayers: sync }));

const Stub = defineComponent({
  props: ["src"],
  setup(_, { expose }) {
    const on = vi.fn(() => () => {});
    expose({ on, seek: vi.fn(), currentTime: 12, duration: 100, toRecordingTime: (t: number) => t + 5 });
    return () => h("div", { class: "player" });
  },
});

const views = [
  { kind: WatchViewKind.PRESENTATION, playlistUrl: "/pres.m3u8" },
  { kind: WatchViewKind.CAMERA, playlistUrl: "/cam.m3u8" },
] as never;

const mountIt = () => mount(SplitView, { props: { views }, global: { stubs: { VideoPlayer: Stub } } });

describe("SplitView", () => {
  beforeEach(() => {
    localStorage.clear();
    sync.mockClear();
  });

  it("plays presentation and camera and syncs them", () => {
    const w = mountIt();
    const players = w.findAllComponents(Stub);
    expect(players.map((p) => p.props("src"))).toEqual(["/pres.m3u8", "/cam.m3u8"]);
    expect(sync).toHaveBeenCalledOnce();
  });

  it("exposes the primary", () => {
    const w = mountIt();
    const vm = w.vm as unknown as { currentTime: number; toRecordingTime(t: number): number };
    expect(vm.currentTime).toBe(12);
    expect(vm.toRecordingTime(1)).toBe(6);
  });

  it("applies a preset and remembers it", async () => {
    const w = mountIt();
    await w.find("button[aria-haspopup=menu]").trigger("click");
    const items = w.findAll("[role=menuitemradio]");
    expect(items).toHaveLength(5);
    await items[3]!.trigger("click");
    expect(localStorage.getItem(RATIO_KEY)).toBe("75");
    expect(w.find("[data-testid=split-camera]").attributes("style")).toContain("--r: 0.75");
    expect(w.find("[data-testid=split-presentation]").attributes("style")).toContain("--r: 0.25");
    expect(w.find("[data-testid=split-menu]").exists()).toBe(false);
  });

  it("resizes with the keyboard", async () => {
    const w = mountIt();
    const gutter = w.find("[role=separator]");
    await gutter.trigger("keydown", { key: "ArrowLeft" });
    expect(localStorage.getItem(RATIO_KEY)).toBe("30");
    await gutter.trigger("keydown", { key: "End" });
    expect(localStorage.getItem(RATIO_KEY)).toBe("100");
  });

  it("starts at the remembered ratio and hides a side at 0 without unmounting it", () => {
    localStorage.setItem(RATIO_KEY, "100");
    const w = mountIt();
    expect(w.find("[data-testid=split-presentation]").attributes("aria-hidden")).toBe("true");
    expect(w.findAllComponents(Stub)).toHaveLength(2);
  });
});

describe("SplitView progress and ending", () => {
  const handlers = new Map<string, () => void>();
  // Opening must not report, only playback that moved: the tests advance this.
  let position = 50;
  const progress = patch;
  const Live = defineComponent({
    props: ["src"],
    setup(props, { expose }) {
      const on = (event: string, fn: () => void) => {
        if (props.src === "/pres.m3u8") handlers.set(event, fn);
        return () => handlers.delete(event);
      };
      expose({ on, seek: vi.fn(), currentTime: 50, getCurrentTime: () => position, getDuration: () => 100, duration: 100, toRecordingTime: (t: number) => t + 100 });
      return () => h("div");
    },
  });
  const mountReporting = (extra: Record<string, unknown> = {}) =>
    mount(SplitView, {
      props: { views, streamId: 9, reportProgress: true, recordingDuration: 400, ...extra },
      global: { stubs: { VideoPlayer: Live } },
    });

  beforeEach(() => {
    vi.useFakeTimers();
    handlers.clear();
    progress.mockClear();
    position = 50;
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("reports the presentation's position as a fraction of the recording, on pause", async () => {
    mountReporting();
    position = 20;
    handlers.get("play")!();
    position = 50;
    handlers.get("pause")!();
    await vi.advanceTimersByTimeAsync(0);
    expect(progress).toHaveBeenCalledTimes(1);
    // (50 s into the clip + 100 s of cut) of 400 s.
    const [, , path, body] = progress.mock.calls[0];
    expect(path).toBe("/progress/9");
    expect(body).toMatchObject({ streamId: 9, progress: 0.375, watched: false });
  });

  it("does not report a pause without playback having moved", async () => {
    mountReporting();
    handlers.get("play")!();
    handlers.get("pause")!();
    await vi.advanceTimersByTimeAsync(0);
    expect(progress).not.toHaveBeenCalled();
  });

  it("emits ended and stops reporting when the primary ends", async () => {
    const w = mountReporting();
    handlers.get("play")!();
    handlers.get("ended")!();
    expect(w.emitted("ended")).toHaveLength(1);
    await vi.advanceTimersByTimeAsync(60_000);
    expect(progress).not.toHaveBeenCalled();
  });

  it("reports nothing when told not to, but still emits ended", async () => {
    const w = mountReporting({ reportProgress: false });
    expect(handlers.has("pause")).toBe(false);
    handlers.get("ended")!();
    await vi.advanceTimersByTimeAsync(60_000);
    expect(progress).not.toHaveBeenCalled();
    expect(w.emitted("ended")).toHaveLength(1);
  });
});
