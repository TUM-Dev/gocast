import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";

import KeyboardShortcuts from "./KeyboardShortcuts.vue";

describe("KeyboardShortcuts", () => {
  it("lists the bookmark key next to the player's own", () => {
    const w = mount(KeyboardShortcuts, { props: { open: true }, global: { stubs: { Teleport: true } } });
    const rows = w.findAll("li").map((li) => li.text());
    expect(rows).toContain("Add a bookmark (signed in) B / b");
    expect(rows.some((r) => r.startsWith("Toggle play/pause"))).toBe(true);
  });
});
