import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";

import TranscriptPanel from "./TranscriptPanel.vue";
import { fakePlayer, routeFetch } from "./testing";

const saveTextFile = vi.hoisted(() => vi.fn());
vi.mock("@/lib/watch-panels-transcript", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/watch-panels-transcript")>()),
  saveTextFile,
}));

afterEach(() => vi.unstubAllGlobals());

const EN = "WEBVTT\n\n00:00:01.000 --> 00:00:03.000\nHello\n\n00:00:04.000 --> 00:00:06.000\nWorld";
const DE = "WEBVTT\n\n00:00:01.000 --> 00:00:03.000\nHallo";

async function mountPanel(languages = ["en", "de"], time = 0) {
  const fetch = routeFetch([
    { path: /subtitles\/en$/, text: EN },
    { path: /subtitles\/de$/, text: DE },
  ]);
  const p = fakePlayer(time);
  Element.prototype.scrollIntoView = vi.fn();
  const wrapper = mount(TranscriptPanel, { props: { streamId: 7, slug: "ws24", languages, player: p.player } });
  await flushPromises();
  return { wrapper, fetch, ...p };
}

describe("TranscriptPanel", () => {
  it("loads the first language and shows its cues with times", async () => {
    const { wrapper, fetch } = await mountPanel();
    expect(fetch.calls[0].url).toBe("/api/v2/streams/ws24/7/subtitles/en");
    expect(wrapper.findAll("li button").map((b) => b.text())).toEqual(["0:01Hello", "0:04World"]);
  });

  it("switches language", async () => {
    const { wrapper } = await mountPanel();
    await wrapper.find("select").setValue("de");
    await flushPromises();
    expect(wrapper.findAll("li button").map((b) => b.text())).toEqual(["0:01Hallo"]);
  });

  it("offers no language choice for a single language", async () => {
    const { wrapper } = await mountPanel(["en"]);
    expect(wrapper.find("select").exists()).toBe(false);
  });

  it("highlights the cue being spoken and follows it", async () => {
    const { wrapper, tick } = await mountPanel(["en"], 2);
    const rows = () => wrapper.findAll("li button");
    expect(rows()[0].attributes("aria-current")).toBe("true");
    tick(5);
    await flushPromises();
    expect(rows()[0].attributes("aria-current")).toBeUndefined();
    expect(rows()[1].attributes("aria-current")).toBe("true");
    expect(Element.prototype.scrollIntoView).toHaveBeenCalled();
  });

  it("seeks to the cue on click", async () => {
    const { wrapper, player } = await mountPanel();
    await wrapper.findAll("li button")[1].trigger("click");
    expect(player.seek).toHaveBeenCalledWith(4);
  });

  it("exports the text", async () => {
    const { wrapper } = await mountPanel();
    const exportButton = wrapper.findAll("button").find((b) => b.text() === "Export transcript")!;
    await exportButton.trigger("click");
    expect(saveTextFile).toHaveBeenCalledWith("Hello\n\nWorld\n\n", "transcript.txt");
  });

  it("explains a language without subtitles", async () => {
    routeFetch([]);
    const wrapper = mount(TranscriptPanel, {
      props: { streamId: 7, slug: "ws24", languages: ["fr"], player: fakePlayer().player },
    });
    await flushPromises();
    expect(wrapper.find("[role=alert]").exists()).toBe(true);
    expect(wrapper.findAll("button").find((b) => b.text() === "Export transcript")!.attributes("disabled")).toBeDefined();
  });
});
