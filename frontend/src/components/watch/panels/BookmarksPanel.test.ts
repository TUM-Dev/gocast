import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";

import BookmarksPanel from "./BookmarksPanel.vue";
import { fakePlayer, routeFetch } from "./testing";

afterEach(() => vi.unstubAllGlobals());

const stored = [
  { bookmarkId: 1, description: "intro", hours: 0, minutes: 0, seconds: 30 },
  { bookmarkId: 2, description: "proof", hours: 0, minutes: 10, seconds: 0 },
];

async function mountPanel(routes = [{ path: "/bookmarks", body: { bookmarks: stored } }] as Parameters<typeof routeFetch>[0]) {
  const fetch = routeFetch(routes);
  const p = fakePlayer(125);
  const wrapper = mount(BookmarksPanel, { props: { streamId: 7, player: p.player } });
  await flushPromises();
  return { wrapper, fetch, ...p };
}

describe("BookmarksPanel", () => {
  it("lists the bookmarks and emits them for the seekbar", async () => {
    const { wrapper } = await mountPanel();
    expect(wrapper.findAll("[data-testid=bookmark]")).toHaveLength(2);
    const events = wrapper.emitted("bookmarks")!;
    expect(events.at(-1)![0]).toEqual([
      { id: 1, description: "intro", time: 30 },
      { id: 2, description: "proof", time: 600 },
    ]);
  });

  it("seeks when a bookmark is clicked", async () => {
    const { wrapper, player } = await mountPanel();
    await wrapper.findAll("[data-testid=bookmark] button")[0].trigger("click");
    expect(player.seek).toHaveBeenCalledWith(30);
  });

  it("adds a bookmark at the current time and emits the new list", async () => {
    const { wrapper, fetch } = await mountPanel([
      { path: "/bookmarks", body: { bookmarks: stored } },
      {
        method: "POST",
        path: "/bookmarks",
        body: { bookmark: { bookmarkId: 3, description: "here", hours: 0, minutes: 2, seconds: 5 } },
      },
    ]);
    await wrapper.find("header button").trigger("click");
    const [description, time] = wrapper.findAll("form input");
    expect((time.element as HTMLInputElement).value).toBe("2:05");
    await description.setValue("here");
    await wrapper.find("form").trigger("submit");
    await flushPromises();

    expect(fetch.calls.find((c) => c.method === "POST")!.url).toContain("description=here&hours=0&minutes=2&seconds=5");
    expect(wrapper.findAll("[data-testid=bookmark]")).toHaveLength(3);
    expect((wrapper.emitted("bookmarks")!.at(-1)![0] as unknown[]).length).toBe(3);
    expect(wrapper.find("form").exists()).toBe(false);
  });

  it("refuses a time it cannot read without calling the server", async () => {
    const { wrapper, fetch } = await mountPanel();
    await wrapper.find("header button").trigger("click");
    const [description, time] = wrapper.findAll("form input");
    await description.setValue("x");
    await time.setValue("soon");
    await wrapper.find("form").trigger("submit");
    expect(wrapper.find("[role=alert]").text()).toContain("m:ss");
    expect(fetch.calls.some((c) => c.method === "POST")).toBe(false);
  });

  it("edits a bookmark", async () => {
    const { wrapper, fetch } = await mountPanel([
      { path: "/bookmarks", body: { bookmarks: stored } },
      {
        method: "PUT",
        path: "/bookmarks/1",
        body: { bookmark: { bookmarkId: 1, description: "opening", hours: 0, minutes: 0, seconds: 30 } },
      },
    ]);
    await wrapper.find('[aria-label="Edit bookmark intro"]').trigger("click");
    await wrapper.find('input[aria-label="Description"]').setValue("opening");
    await wrapper.find("[data-testid=bookmark] form").trigger("submit");
    await flushPromises();

    expect(fetch.calls.find((c) => c.method === "PUT")!.url).toBe(
      "/api/v2/bookmarks/1?description=opening&hours=0&minutes=0&seconds=30",
    );
    expect(wrapper.text()).toContain("opening");
  });

  it("deletes a bookmark", async () => {
    const { wrapper } = await mountPanel([
      { path: "/bookmarks", body: { bookmarks: stored } },
      { method: "DELETE", path: "/bookmarks/2", status: 204 },
    ]);
    await wrapper.find('[aria-label="Delete bookmark proof"]').trigger("click");
    await flushPromises();
    expect(wrapper.findAll("[data-testid=bookmark]")).toHaveLength(1);
    expect((wrapper.emitted("bookmarks")!.at(-1)![0] as unknown[]).length).toBe(1);
  });

  it("asks a signed-out viewer to sign in", async () => {
    const { wrapper } = await mountPanel([{ path: "/bookmarks", status: 401, body: { message: "no" } }]);
    expect(wrapper.text()).toContain("Sign in");
    expect(wrapper.find("header button").exists()).toBe(false);
  });
});
