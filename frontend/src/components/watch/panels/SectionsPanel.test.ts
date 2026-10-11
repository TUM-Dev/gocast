import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";

import SectionsPanel from "./SectionsPanel.vue";
import { fakePlayer, routeFetch } from "./testing";

afterEach(() => vi.unstubAllGlobals());

const body = {
  sections: [
    { description: "Intro", fileId: 11 },
    { description: "Hops", startMinutes: 2, fileId: 12 },
    { description: "Malt", startMinutes: 5, fileId: 13 },
  ],
};

async function mountPanel(time = 0, routes = [{ path: /\/sections$/, body }] as Parameters<typeof routeFetch>[0]) {
  routeFetch(routes);
  const p = fakePlayer(time);
  const scroll = vi.fn();
  Element.prototype.scrollIntoView = scroll;
  const wrapper = mount(SectionsPanel, { props: { streamId: 7, slug: "ws24", player: p.player } });
  await flushPromises();
  return { wrapper, scroll, ...p };
}

const rows = (w: Awaited<ReturnType<typeof mountPanel>>["wrapper"]) => w.findAll("li button");

describe("SectionsPanel", () => {
  it("shows each section with its image from the stream's files", async () => {
    const { wrapper } = await mountPanel();
    expect(rows(wrapper).map((r) => r.text())).toEqual(["0:00Intro", "2:00Hops", "5:00Malt"]);
    expect(wrapper.findAll("img").map((i) => i.attributes("src"))).toEqual([
      "/api/v2/streams/7/files/11",
      "/api/v2/streams/7/files/12",
      "/api/v2/streams/7/files/13",
    ]);
  });

  it("emits the sections for the seekbar", async () => {
    const { wrapper } = await mountPanel();
    expect(wrapper.emitted("sections")![0][0]).toHaveLength(3);
  });

  it("highlights and scrolls to the section playback is in", async () => {
    const { wrapper, tick, scroll } = await mountPanel(10);
    expect(rows(wrapper)[0].attributes("aria-current")).toBe("true");
    tick(130);
    await flushPromises();
    expect(rows(wrapper)[0].attributes("aria-current")).toBeUndefined();
    expect(rows(wrapper)[1].attributes("aria-current")).toBe("true");
    expect(scroll).toHaveBeenCalled();
  });

  it("does not scroll while follow is off, but still highlights", async () => {
    const { wrapper, tick, scroll } = await mountPanel(10);
    scroll.mockClear();
    await wrapper.find("header button").trigger("click");
    tick(400);
    await flushPromises();
    expect(rows(wrapper)[2].attributes("aria-current")).toBe("true");
    expect(scroll).not.toHaveBeenCalled();
  });

  it("seeks on click", async () => {
    const { wrapper, player } = await mountPanel();
    await rows(wrapper)[2].trigger("click");
    expect(player.seek).toHaveBeenCalledWith(300);
  });

  it("says so when there are no sections, and when loading fails", async () => {
    expect((await mountPanel(0, [{ path: /\/sections$/, body: {} }])).wrapper.text()).toContain("no sections");
    const failed = await mountPanel(0, [{ path: /\/sections$/, status: 500, body: { message: "down" } }]);
    expect(failed.wrapper.find("[role=alert]").text()).toBe("down");
  });

  it("stops listening when it goes away", async () => {
    const { wrapper, listeners } = await mountPanel();
    expect(listeners("timeupdate")).toBe(1);
    wrapper.unmount();
    expect(listeners("timeupdate")).toBe(0);
  });
});
