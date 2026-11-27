import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";

import OfflineThumbnail from "./OfflineThumbnail.vue";

describe("OfflineThumbnail", () => {
  it("offers to play, with the playable length in the corner", () => {
    const w = mount(OfflineThumbnail, { props: { src: "blob:x", seconds: 5400 } });
    expect(w.find(".fa-play").exists()).toBe(true);
    expect(w.find("img").attributes("src")).toBe("blob:x");
    expect(w.text()).toContain("01:30:00");
  });

  it("shows a lock once the licence ran out, and a plain tile without a picture", () => {
    const w = mount(OfflineThumbnail, { props: { expired: true } });
    expect(w.find(".fa-lock").exists()).toBe(true);
    expect(w.find(".fa-play").exists()).toBe(false);
    expect(w.find("img").exists()).toBe(false);
  });

  it("leaves the badge out of the side list's small tiles", () => {
    const w = mount(OfflineThumbnail, { props: { compact: true, seconds: 60 } });
    expect(w.find(".fa-play").exists()).toBe(true);
    expect(w.text()).not.toContain("00:01:00");
  });
});
