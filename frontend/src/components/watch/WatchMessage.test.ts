import { describe, expect, it } from "vitest";
import { mount } from "@vue/test-utils";

import WatchMessage from "./WatchMessage.vue";

describe("WatchMessage", () => {
  it("explains that an expired lecture is no longer available", () => {
    const message = mount(WatchMessage, { props: { kind: "expired" } });

    expect(message.text()).toContain("This lecture has expired.");
    expect(message.text()).toContain("It is no longer available to watch.");
  });
});
