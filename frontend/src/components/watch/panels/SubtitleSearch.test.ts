import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import SubtitleSearch from "./SubtitleSearch.vue";
import { fakePlayer, routeFetch, type Route } from "./testing";

beforeEach(() => vi.useFakeTimers());
afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

const hit = { timestamp: "65000", textPrev: "before", text: "hopfen is key", textNext: "after" };

function mountSearch(routes: Route[]) {
  const fetch = routeFetch(routes);
  const p = fakePlayer();
  const wrapper = mount(SubtitleSearch, { props: { streamId: 5, player: p.player } });
  return { wrapper, fetch, ...p };
}

async function type(wrapper: ReturnType<typeof mountSearch>["wrapper"], text: string) {
  await wrapper.find("input").setValue(text);
}

async function settle() {
  await vi.advanceTimersByTimeAsync(400);
  await flushPromises();
}

describe("SubtitleSearch", () => {
  it("searches once after the viewer stops typing and shows the context", async () => {
    const { wrapper, fetch } = mountSearch([{ path: "/streams/5/subtitles/search", body: { hits: [hit] } }]);
    await type(wrapper, "hop");
    await type(wrapper, "hopf");
    await type(wrapper, "hopfen");
    expect(fetch.calls).toHaveLength(0);
    await settle();

    expect(fetch.calls).toHaveLength(1);
    expect(fetch.calls[0].url).toContain("query=hopfen");
    const result = wrapper.find("li button");
    expect(result.text()).toContain("1:05");
    expect(result.text()).toContain("before");
    expect(result.text()).toContain("hopfen is key");
    expect(result.text()).toContain("after");
  });

  it("seeks to a result", async () => {
    const { wrapper, player } = mountSearch([{ path: "/streams/5/subtitles/search", body: { hits: [hit] } }]);
    await type(wrapper, "hopfen");
    await settle();
    await wrapper.find("li button").trigger("click");
    expect(player.seek).toHaveBeenCalledWith(65);
  });

  it("does not search for fewer than three characters, and clears the results", async () => {
    const { wrapper, fetch } = mountSearch([{ path: "/streams/5/subtitles/search", body: { hits: [hit] } }]);
    await type(wrapper, "ho");
    await settle();
    expect(fetch.calls).toHaveLength(0);

    await type(wrapper, "hopfen");
    await settle();
    expect(wrapper.findAll("li")).toHaveLength(1);
    await type(wrapper, "h");
    expect(wrapper.findAll("li")).toHaveLength(0);
  });

  it("says nothing was found", async () => {
    const { wrapper } = mountSearch([{ path: "/streams/5/subtitles/search", body: {} }]);
    await type(wrapper, "zzzz");
    await settle();
    expect(wrapper.text()).toContain("Nothing found");
  });

  it("handles search being unavailable (503) without calling it an error", async () => {
    const { wrapper } = mountSearch([{ path: "/streams/5/subtitles/search", status: 503, body: { message: "off" } }]);
    await type(wrapper, "hopfen");
    await settle();
    expect(wrapper.text()).toContain("not available");
    expect(wrapper.find("[role=alert]").exists()).toBe(false);
    expect(wrapper.text()).not.toContain("Nothing found");
  });

  it("shows other failures as errors", async () => {
    const { wrapper } = mountSearch([{ path: "/streams/5/subtitles/search", status: 500, body: { message: "boom" } }]);
    await type(wrapper, "hopfen");
    await settle();
    expect(wrapper.find("[role=alert]").text()).toBe("boom");
  });

  it("recovers once search is back", async () => {
    let up = false;
    const fetch = routeFetch([]);
    fetch.mock.mockImplementation((url: string) => {
      if (url.endsWith("/auth/token")) return Promise.resolve(new Response(JSON.stringify({ access_token: "t" })));
      return Promise.resolve(
        up ? new Response(JSON.stringify({ hits: [hit] })) : new Response("{}", { status: 503 }),
      );
    });
    const wrapper = mount(SubtitleSearch, { props: { streamId: 5, player: fakePlayer().player } });
    await type(wrapper, "hopfen");
    await settle();
    expect(wrapper.text()).toContain("not available");
    up = true;
    await type(wrapper, "hopfens");
    await settle();
    expect(wrapper.text()).not.toContain("not available");
    expect(wrapper.findAll("li")).toHaveLength(1);
  });

  it("ignores an answer for a query the viewer has typed past", async () => {
    const resolvers: ((r: Response) => void)[] = [];
    const fetch = routeFetch([]);
    fetch.mock.mockImplementation((url: string) =>
      url.endsWith("/auth/token")
        ? Promise.resolve(new Response(JSON.stringify({ access_token: "t" })))
        : new Promise((resolve) => resolvers.push(resolve)),
    );
    const wrapper = mount(SubtitleSearch, { props: { streamId: 5, player: fakePlayer().player } });
    await type(wrapper, "first");
    await settle();
    await type(wrapper, "second");
    await settle();
    expect(resolvers).toHaveLength(2);
    resolvers[1](new Response(JSON.stringify({ hits: [{ ...hit, text: "second hit" }] })));
    await flushPromises();
    resolvers[0](new Response(JSON.stringify({ hits: [{ ...hit, text: "first hit" }] })));
    await flushPromises();
    expect(wrapper.text()).toContain("second hit");
    expect(wrapper.text()).not.toContain("first hit");
  });
});
