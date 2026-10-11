import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, describe, expect, it } from "vitest";
import { nextTick } from "vue";

import SeekbarOverlay from "./SeekbarOverlay.vue";

let wrapper: ReturnType<typeof mount> | undefined;
afterEach(() => wrapper?.unmount());

/** A stand-in for the video.js DOM: the progress holder appears after the player loads. */
function playerDom(withHolder = true) {
  const root = document.createElement("div");
  document.body.append(root);
  const holder = document.createElement("div");
  holder.className = "vjs-progress-holder";
  if (withHolder) root.append(holder);
  return { root, holder };
}

const base = { duration: 1000 };

describe("SeekbarOverlay", () => {
  it("draws separators between sections (not before the first) and bookmark markers", async () => {
    const { root, holder } = playerDom();
    wrapper = mount(SeekbarOverlay, {
      props: {
        ...base,
        container: root,
        sections: [
          { title: "Intro", start: 0 },
          { title: "Main", start: 250 },
          { title: "End", start: 900 },
        ],
        bookmarks: [{ hours: 0, minutes: 5, seconds: 0, description: "recap" }],
      },
    });
    await flushPromises();
    const seps = holder.querySelectorAll<HTMLElement>("[data-testid=seekbar-separator]");
    expect([...seps].map((s) => s.style.left)).toEqual(["25%", "90%"]);
    expect(seps[0].title).toBe("Main");
    const marks = holder.querySelectorAll<HTMLElement>("[data-testid=seekbar-bookmark]");
    expect(marks).toHaveLength(1);
    expect(marks[0].style.left).toBe("30%");
    expect(marks[0].title).toBe("recap");
    expect(holder.querySelector("[data-testid=seekbar-heatmap]")).toBeNull();
  });

  it("renders the heatmap when there are hits", async () => {
    const { root, holder } = playerDom();
    wrapper = mount(SeekbarOverlay, { props: { ...base, container: root, heatmap: [0, 3, 1] } });
    await flushPromises();
    expect(holder.querySelector("[data-testid=seekbar-heatmap] path")?.getAttribute("d")).toMatch(/^M 0,100/);
  });

  it("leaves out what is outside a clip and shifts the rest", async () => {
    const { root, holder } = playerDom();
    wrapper = mount(SeekbarOverlay, {
      props: {
        container: root,
        duration: 100,
        offset: 500,
        sections: [
          { title: "a", start: 0 },
          { title: "b", start: 400 },
          { title: "c", start: 550 },
        ],
        bookmarks: [
          { hours: 0, minutes: 1, seconds: 0, description: "before" },
          { hours: 0, minutes: 9, seconds: 0, description: "inside" },
        ],
      },
    });
    await flushPromises();
    const seps = holder.querySelectorAll<HTMLElement>("[data-testid=seekbar-separator]");
    expect([...seps].map((s) => s.style.left)).toEqual(["50%"]);
    expect([...holder.querySelectorAll<HTMLElement>("[data-testid=seekbar-bookmark]")].map((m) => m.title)).toEqual([
      "inside",
    ]);
  });

  it("waits for video.js to build the seekbar", async () => {
    const { root, holder } = playerDom(false);
    wrapper = mount(SeekbarOverlay, {
      props: { ...base, container: root, sections: [{ title: "a", start: 0 }, { title: "b", start: 500 }] },
    });
    await flushPromises();
    expect(document.querySelector("[data-testid=seekbar-overlay]")).toBeNull();
    root.append(holder);
    await new Promise((r) => setTimeout(r, 0)); // MutationObserver callback
    await nextTick();
    expect(holder.querySelectorAll("[data-testid=seekbar-separator]")).toHaveLength(1);
  });

  it("renders nothing without a container", async () => {
    wrapper = mount(SeekbarOverlay, { props: { ...base, container: null } });
    await flushPromises();
    expect(document.querySelector("[data-testid=seekbar-overlay]")).toBeNull();
  });
});
