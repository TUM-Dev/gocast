import { flushPromises, mount } from "@vue/test-utils";
import { beforeEach, describe, expect, it, vi } from "vitest";

import InfoPageView from "./InfoPageView.vue";

/**
 * The three info pages share this component, so vue-router reuses the instance and
 * only the prop changes. That makes response ordering the thing to get right.
 */

const { fetchInfoPage } = vi.hoisted(() => ({ fetchInfoPage: vi.fn() }));
vi.mock("@/lib/info-pages", () => ({ fetchInfoPage }));

beforeEach(() => vi.clearAllMocks());

describe("InfoPageView", () => {
  it("renders the HTML the server sent", async () => {
    fetchInfoPage.mockResolvedValue("<h1>Privacy</h1>");
    const wrapper = mount(InfoPageView, { props: { name: "privacy" } });
    await flushPromises();

    expect(wrapper.html()).toContain("<h1>Privacy</h1>");
  });

  it("renders nothing for a page nobody has written yet", async () => {
    // The template rendered an empty div for an unseeded row; visible copy here
    // would be a behaviour change dressed up as a nicety.
    fetchInfoPage.mockResolvedValue("");
    const wrapper = mount(InfoPageView, { props: { name: "privacy" } });
    await flushPromises();

    expect(wrapper.text()).toBe("");
  });

  it("says so when the request fails", async () => {
    fetchInfoPage.mockRejectedValue(new Error("boom"));
    const wrapper = mount(InfoPageView, { props: { name: "privacy" } });
    await flushPromises();

    expect(wrapper.text()).toContain("not available");
  });

  it("drops a response that arrives after the route moved on", async () => {
    // The whole point: /privacy resolving late must not overwrite /imprint.
    let resolvePrivacy: (html: string) => void = () => {};
    fetchInfoPage.mockImplementationOnce(
      () => new Promise<string>((resolve) => (resolvePrivacy = resolve)),
    );

    const wrapper = mount(InfoPageView, { props: { name: "privacy" } });

    fetchInfoPage.mockResolvedValueOnce("<h1>Imprint</h1>");
    await wrapper.setProps({ name: "imprint" });
    await flushPromises();

    resolvePrivacy("<h1>Privacy</h1>");
    await flushPromises();

    expect(wrapper.html()).toContain("<h1>Imprint</h1>");
    expect(wrapper.html()).not.toContain("<h1>Privacy</h1>");
  });

  it("clears the previous page while the next one loads", async () => {
    fetchInfoPage.mockResolvedValue("<h1>Privacy</h1>");
    const wrapper = mount(InfoPageView, { props: { name: "privacy" } });
    await flushPromises();
    expect(wrapper.html()).toContain("<h1>Privacy</h1>");

    fetchInfoPage.mockImplementationOnce(() => new Promise<string>(() => {}));
    await wrapper.setProps({ name: "imprint" });
    await flushPromises();

    expect(wrapper.html()).not.toContain("<h1>Privacy</h1>");
  });
});
