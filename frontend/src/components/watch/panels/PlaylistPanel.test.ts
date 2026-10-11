import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";

import PlaylistPanel from "./PlaylistPanel.vue";
import { routeFetch } from "./testing";

afterEach(() => vi.unstubAllGlobals());

const entry = (id: number, extra: Record<string, unknown> = {}) => ({
  streamId: id,
  courseSlug: "ws24",
  streamName: `Lecture ${id}`,
  start: "2026-01-01T10:00:00Z",
  ...extra,
});

async function mountPanel(entries: unknown[]) {
  routeFetch([{ path: /\/playlist$/, body: { entries } }]);
  const scroll = vi.fn();
  Element.prototype.scrollIntoView = scroll;
  const wrapper = mount(PlaylistPanel, {
    props: { streamId: 2, slug: "ws24" },
    global: { stubs: { RouterLink: { props: ["to"], template: '<a :href="to"><slot /></a>' } } },
  });
  await flushPromises();
  return { wrapper, scroll };
}

describe("PlaylistPanel", () => {
  it("lists the lectures with thumbnails, linking to the watch page", async () => {
    const { wrapper } = await mountPanel([entry(1), entry(2), entry(3)]);
    const links = wrapper.findAll("li a");
    expect(links.map((a) => a.attributes("href"))).toEqual(["/w/ws24/1", "/w/ws24/2", "/w/ws24/3"]);
    expect(links[0].find("img").attributes("src")).toBe("/api/v2/streams/ws24/1/thumbs");
    expect(links[1].text()).toContain("Lecture 2");
  });

  it("marks the current lecture and scrolls it into view", async () => {
    const { wrapper, scroll } = await mountPanel([entry(1), entry(2), entry(3)]);
    const links = wrapper.findAll("li a");
    expect(links[1].attributes("aria-current")).toBe("page");
    expect(links[0].attributes("aria-current")).toBeUndefined();
    expect(scroll).toHaveBeenCalledWith({ block: "center" });
  });

  it("links previous and next, and disables them at the ends", async () => {
    const { wrapper } = await mountPanel([entry(1), entry(2), entry(3)]);
    const nav = wrapper.find("nav");
    expect(nav.findAll("a").map((a) => a.attributes("href"))).toEqual(["/w/ws24/1", "/w/ws24/3"]);

    const first = await mountPanel([entry(2), entry(3)]);
    expect(first.wrapper.find("nav").findAll("a").map((a) => a.attributes("href"))).toEqual(["/w/ws24/3"]);
  });

  it("draws watch progress", async () => {
    const { wrapper } = await mountPanel([entry(1, { streamProgress: { progress: 0.4 } }), entry(2)]);
    const bars = wrapper.findAll("[data-testid=progress]");
    expect(bars).toHaveLength(1);
    expect(bars[0].attributes("style")).toContain("width: 40%");
  });

  it("reports a failure", async () => {
    routeFetch([{ path: /\/playlist$/, status: 403, body: { message: "forbidden" } }]);
    const wrapper = mount(PlaylistPanel, {
      props: { streamId: 2, slug: "ws24" },
      global: { stubs: { RouterLink: true } },
    });
    await flushPromises();
    expect(wrapper.find("[role=alert]").text()).toBe("forbidden");
  });
});
