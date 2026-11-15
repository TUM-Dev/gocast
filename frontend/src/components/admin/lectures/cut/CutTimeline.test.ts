import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";

import type { Cut } from "@/lib/cuts";
import CutTimeline from "./CutTimeline.vue";

const cut = (id: number, start: number, end: number, suggested = false): Cut => ({ id, start, end, suggested });

function mountTimeline(props: Partial<InstanceType<typeof CutTimeline>["$props"]> = {}) {
  const wrapper = mount(CutTimeline, {
    props: {
      peaks: new Uint8Array(100).fill(100),
      bucketSeconds: 1,
      duration: 100,
      cuts: [],
      currentTime: 0,
      pendingStart: null,
      zoom: 1,
      selectedId: null,
      ...props,
    },
  });
  // happy-dom lays nothing out; give the track the width the pointer maths needs.
  const track = wrapper.get("[data-testid=cut-timeline]");
  track.element.getBoundingClientRect = () => ({ left: 0, top: 0, width: 1000, height: 100, right: 1000, bottom: 100, x: 0, y: 0, toJSON: () => ({}) });
  return { wrapper, track };
}

const pointer = (track: ReturnType<typeof mountTimeline>["track"], type: string, clientX: number, target?: Element) =>
  (target ? { element: target } : track).element.dispatchEvent(
    new MouseEvent(type, { clientX, bubbles: true, button: 0 }),
  );

describe("CutTimeline", () => {
  it("seeks on a click", () => {
    const { wrapper, track } = mountTimeline();
    pointer(track, "pointerdown", 250);
    pointer(track, "pointerup", 250);
    expect(wrapper.emitted("seek")).toEqual([[25]]);
    expect(wrapper.emitted("select")).toEqual([[null]]);
    expect(wrapper.emitted("create")).toBeUndefined();
  });

  it("creates a cut by dragging, in either direction", () => {
    const { wrapper, track } = mountTimeline();
    pointer(track, "pointerdown", 600);
    pointer(track, "pointermove", 300);
    pointer(track, "pointerup", 300);
    expect(wrapper.emitted("create")).toEqual([[30, 60]]);
    expect(wrapper.emitted("seek")).toBeUndefined();
  });

  it("selects the cut that is clicked", () => {
    const { wrapper, track } = mountTimeline({ cuts: [cut(4, 20, 40)] });
    pointer(track, "pointerdown", 300);
    pointer(track, "pointerup", 300);
    expect(wrapper.emitted("select")).toEqual([[4]]);
  });

  it("resizes from a handle", () => {
    const { wrapper, track } = mountTimeline({ cuts: [cut(4, 20, 40)] });
    const handle = wrapper.get("[data-handle=end]").element;
    pointer(track, "pointerdown", 400, handle);
    pointer(track, "pointermove", 500);
    pointer(track, "pointerup", 500);
    expect(wrapper.emitted("resize")).toEqual([[4, "end", 50]]);
    expect(wrapper.emitted("create")).toBeUndefined();
  });

  it("marks suggestions apart from cuts", () => {
    const { wrapper } = mountTimeline({ cuts: [cut(1, 10, 20), cut(2, 50, 60, true)] });
    expect(wrapper.findAll("[data-testid=cut-region]")).toHaveLength(1);
    expect(wrapper.findAll("[data-testid=cut-suggested]")).toHaveLength(1);
  });

  it("places the playhead and the pending start", () => {
    const { wrapper } = mountTimeline({ currentTime: 25, pendingStart: 10 });
    expect(wrapper.get("[data-testid=cut-playhead]").attributes("style")).toContain("left: 25%");
    expect(wrapper.get("[data-testid=cut-pending]").attributes("style")).toContain("left: 10%");
  });

  it("steps with the arrow keys and deletes the selected cut", async () => {
    const { wrapper, track } = mountTimeline({ currentTime: 50, selectedId: 3, cuts: [cut(3, 10, 20)] });
    await track.trigger("keydown", { key: "ArrowRight" });
    await track.trigger("keydown", { key: "ArrowLeft", shiftKey: true });
    await track.trigger("keydown", { key: "Delete" });
    expect(wrapper.emitted("seek")).toEqual([[51], [40]]);
    expect(wrapper.emitted("remove")).toEqual([[3]]);
  });

  it("ignores Delete without a selection", async () => {
    const { wrapper, track } = mountTimeline();
    await track.trigger("keydown", { key: "Delete" });
    expect(wrapper.emitted("remove")).toBeUndefined();
  });

  it("does nothing without a recording length", () => {
    const { wrapper, track } = mountTimeline({ duration: 0 });
    pointer(track, "pointerdown", 100);
    pointer(track, "pointerup", 100);
    expect(wrapper.emitted("seek")).toBeUndefined();
  });

  it("widens the track when zoomed", () => {
    const { track } = mountTimeline({ zoom: 4 });
    expect(track.attributes("style")).toContain("width: 400%");
  });
});
