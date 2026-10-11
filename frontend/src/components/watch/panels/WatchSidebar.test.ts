import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { h } from "vue";

import WatchSidebar from "./WatchSidebar.vue";
import { fakePlayer, routeFetch } from "./testing";

beforeEach(() => {
  localStorage.clear();
  Element.prototype.scrollIntoView = vi.fn();
  routeFetch([
    { path: "/bookmarks", body: { bookmarks: [{ bookmarkId: 1, description: "intro", seconds: 30 }] } },
    { path: /\/sections$/, body: {} },
    { path: /\/playlist$/, body: {} },
  ]);
});
afterEach(() => vi.unstubAllGlobals());

function mountSidebar(props: Record<string, unknown> = {}) {
  const p = fakePlayer();
  const wrapper = mount(WatchSidebar, {
    props: { streamId: 7, slug: "ws24", player: p.player, ...props },
    slots: { chat: () => h("div", { "data-testid": "chat" }, "chat here") },
    global: { stubs: { RouterLink: true } },
    attachTo: document.body,
  });
  return wrapper;
}

const tabs = (w: ReturnType<typeof mountSidebar>) => w.findAll("[role=tab]").map((t) => t.text());

describe("WatchSidebar", () => {
  it("offers every tab it can and starts on the first", async () => {
    const w = mountSidebar({ subtitleLanguages: ["en"] });
    await flushPromises();
    expect(tabs(w)).toEqual(["Chat", "Bookmarks", "Sections", "Playlist", "Transcript", "Search"]);
    expect(w.find("[role=tab][aria-selected=true]").text()).toBe("Chat");
    expect(w.find("[data-testid=chat]").isVisible()).toBe(true);
    w.unmount();
  });

  it("hides what the shell says is not there", () => {
    const w = mountSidebar({ showChat: false, showBookmarks: false });
    expect(tabs(w)).toEqual(["Sections", "Playlist", "Search"]);
    expect(w.find("[data-testid=chat]").exists()).toBe(false);
    expect(w.text()).not.toContain("Transcript");
    w.unmount();
  });

  it("switches panels, mounting the others lazily and keeping them", async () => {
    const w = mountSidebar();
    await flushPromises();
    expect(w.find("#watch-panel-sections").exists()).toBe(false);

    await w.findAll("[role=tab]")[2].trigger("click");
    await flushPromises();
    expect(w.find("#watch-panel-sections").isVisible()).toBe(true);
    expect(w.find("#watch-panel-chat").isVisible()).toBe(false);

    await w.findAll("[role=tab]")[0].trigger("click");
    expect(w.find("#watch-panel-sections").exists()).toBe(true);
    expect(w.find("#watch-panel-sections").isVisible()).toBe(false);
    w.unmount();
  });

  it("remembers the tab across visits", async () => {
    const first = mountSidebar();
    await first.findAll("[role=tab]")[3].trigger("click");
    first.unmount();

    const second = mountSidebar();
    await flushPromises();
    expect(second.find("[role=tab][aria-selected=true]").text()).toBe("Playlist");
    second.unmount();
  });

  it("falls back to the first tab when the remembered one is hidden", () => {
    localStorage.setItem("watch.sidebarTab", "transcript");
    const w = mountSidebar();
    expect(w.find("[role=tab][aria-selected=true]").text()).toBe("Chat");
    w.unmount();
  });

  it("moves between tabs with the arrow keys", async () => {
    const w = mountSidebar();
    await w.find("[role=tab]").trigger("keydown", { key: "ArrowRight" });
    expect(w.find("[role=tab][aria-selected=true]").text()).toBe("Bookmarks");
    await w.find("[role=tab]").trigger("keydown", { key: "ArrowLeft" });
    await w.find("[role=tab]").trigger("keydown", { key: "ArrowLeft" });
    expect(w.find("[role=tab][aria-selected=true]").text()).toBe("Search");
    w.unmount();
  });

  it("emits bookmarks while their tab is not showing, and reports the tab", async () => {
    const w = mountSidebar();
    await flushPromises();
    expect(w.emitted("bookmarks")!.at(-1)![0]).toEqual([{ id: 1, description: "intro", time: 30 }]);
    expect(w.emitted("tab")![0]).toEqual(["chat"]);
    w.unmount();
  });

  it("passes sections up once their tab has been opened", async () => {
    routeFetch([
      { path: "/bookmarks", body: {} },
      { path: /\/sections$/, body: { sections: [{ description: "Intro", fileId: 1 }] } },
    ]);
    const w = mountSidebar();
    await w.findAll("[role=tab]")[2].trigger("click");
    await flushPromises();
    expect(w.emitted("sections")![0][0]).toEqual([{ description: "Intro", time: 0, fileId: 1 }]);
    w.unmount();
  });

  it("shows the bookmarks and opens the add form on request", async () => {
    const w = mountSidebar();
    await flushPromises();
    expect(w.find("[role=tab][aria-selected=true]").text()).toBe("Chat");
    await (w.vm as unknown as { addBookmark(): Promise<void> }).addBookmark();
    await flushPromises();
    expect(w.find("[role=tab][aria-selected=true]").text()).toBe("Bookmarks");
    expect(w.find("#watch-panel-bookmarks form").exists()).toBe(true);
    w.unmount();
  });

  it("does nothing for a viewer without bookmarks", async () => {
    const w = mountSidebar({ showBookmarks: false });
    await flushPromises();
    await (w.vm as unknown as { addBookmark(): Promise<void> }).addBookmark();
    expect(w.find("[role=tab][aria-selected=true]").text()).toBe("Chat");
    w.unmount();
  });
});

describe("WatchSidebar after the transcript has been opened", () => {
  const VTT = "WEBVTT\n\n00:00:01.000 --> 00:00:03.000\nHello\n";

  const visiblePanels = (w: ReturnType<typeof mountSidebar>) =>
    w.findAll("[role=tabpanel]").filter((p) => p.isVisible()).map((p) => p.attributes("id"));

  async function expectEveryTabReachable(w: ReturnType<typeof mountSidebar>) {
    for (const name of ["Chat", "Bookmarks", "Sections", "Transcript", "Search", "Chat"]) {
      const tab = w.findAll("[role=tab]").find((t) => t.text() === name)!;
      await tab.trigger("click");
      await flushPromises();
      expect(visiblePanels(w)).toEqual([`watch-panel-${name.toLowerCase()}`]);
      expect(w.find("[role=tab][aria-selected=true]").text()).toBe(name);
    }
  }

  it("still switches to every other tab", async () => {
    routeFetch([
      { path: /\/subtitles\/en$/, text: VTT },
      { path: "/bookmarks", body: {} },
      { path: /\/sections$/, body: {} },
    ]);
    const w = mountSidebar({ subtitleLanguages: ["en"] });
    await expectEveryTabReachable(w);
    w.unmount();
  });

  it("still switches when the subtitle fetch fails", async () => {
    routeFetch([
      { path: /\/subtitles\/en$/, status: 500 },
      { path: "/bookmarks", body: {} },
      { path: /\/sections$/, body: {} },
    ]);
    const w = mountSidebar({ subtitleLanguages: ["en"] });
    await expectEveryTabReachable(w);
    expect(w.find("#watch-panel-transcript [role=alert]").exists()).toBe(true);
    w.unmount();
  });

  it("opens on the remembered transcript tab and leaves it again", async () => {
    localStorage.setItem("watch.sidebarTab", "transcript");
    routeFetch([{ path: /\/subtitles\/en$/, status: 500 }, { path: "/bookmarks", body: {} }]);
    const w = mountSidebar({ subtitleLanguages: ["en"] });
    await flushPromises();
    expect(visiblePanels(w)).toEqual(["watch-panel-transcript"]);
    await w.findAll("[role=tab]")[0].trigger("click");
    await flushPromises();
    expect(visiblePanels(w)).toEqual(["watch-panel-chat"]);
    w.unmount();
  });

  it("keeps the tab strip from being squeezed out by a tall panel", () => {
    // A transcript taller than the sidebar made the flex column shrink the strip (an
    // overflow container's minimum height is 0) until the other tabs were unreachable.
    const w = mountSidebar({ subtitleLanguages: ["en"] });
    expect(w.find("[role=tablist]").classes()).toContain("shrink-0");
    w.unmount();
  });

  it("drops a remembered transcript tab once the subtitles are gone", async () => {
    localStorage.setItem("watch.sidebarTab", "transcript");
    const w = mountSidebar({ subtitleLanguages: ["en"] });
    await w.setProps({ subtitleLanguages: [] });
    expect(visiblePanels(w)).toEqual(["watch-panel-chat"]);
    w.unmount();
  });
});
