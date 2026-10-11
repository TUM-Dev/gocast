import { mount } from "@vue/test-utils";
import { afterEach, describe, expect, it } from "vitest";
import { defineComponent, h, nextTick, ref } from "vue";

import { canonicalUrl, setCanonicalLink, useCanonicalLink } from "./canonical-link";

const links = () => [...document.head.querySelectorAll('link[rel="canonical"]')] as HTMLLinkElement[];

afterEach(() => links().forEach((l) => l.remove()));

describe("setCanonicalLink", () => {
  it("adds the tag and removes it again", () => {
    const undo = setCanonicalLink("https://tum.live/w/a/1");
    expect(links().map((l) => l.href)).toEqual(["https://tum.live/w/a/1"]);
    undo();
    expect(links()).toHaveLength(0);
  });

  it("restores a tag the document already had", () => {
    const own = document.createElement("link");
    own.rel = "canonical";
    own.href = "https://tum.live/";
    document.head.appendChild(own);
    const undo = setCanonicalLink("https://tum.live/w/a/1");
    expect(links()).toHaveLength(1);
    expect(links()[0].href).toBe("https://tum.live/w/a/1");
    undo();
    expect(links()[0].href).toBe("https://tum.live/");
  });
});

describe("useCanonicalLink", () => {
  it("follows the value and goes away with the component", async () => {
    const href = ref<string | undefined>(undefined);
    const wrapper = mount(
      defineComponent({
        setup() {
          useCanonicalLink(href);
          return () => h("div");
        },
      }),
    );
    expect(links()).toHaveLength(0);
    href.value = "https://tum.live/w/a/1";
    await nextTick();
    expect(links().map((l) => l.href)).toEqual(["https://tum.live/w/a/1"]);
    href.value = "https://tum.live/w/a/2";
    await nextTick();
    expect(links().map((l) => l.href)).toEqual(["https://tum.live/w/a/2"]);
    wrapper.unmount();
    expect(links()).toHaveLength(0);
  });
});

describe("canonicalUrl", () => {
  it("joins escaped segments and drops empty ones", () => {
    expect(canonicalUrl("https://tum.live/", "w", "ws 24/x", 7, undefined)).toBe("https://tum.live/w/ws%2024%2Fx/7");
  });
  it("falls back to the page's origin", () => {
    expect(canonicalUrl("", "w", "a", 1)).toBe(`${window.location.origin}/w/a/1`);
  });
});
