import { mount } from "@vue/test-utils";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { defineComponent, h } from "vue";

import { WatchViewKind } from "@/gen/server/apiv2_pb";
import { RATIO_KEY } from "./ratio";
import SplitView from "./SplitView.vue";

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
    expect(w.find("[data-testid=split-camera]").attributes("style")).toContain("75%");
    expect(w.find("[data-testid=split-presentation]").attributes("style")).toContain("25%");
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
