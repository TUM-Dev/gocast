import { describe, expect, it } from "vitest";

import { markTimeTooltip, spriteOptions } from "./sprites";

const sprite = { url: "/s.jpg", interval: 5 };

describe("spriteOptions", () => {
  it("fills in the server's frame geometry", () => {
    expect(spriteOptions(sprite, { mobile: false })).toEqual({
      url: "/s.jpg",
      interval: 5,
      width: 160,
      height: 90,
      columns: 17,
    });
  });
  it("takes a different geometry", () => {
    expect(spriteOptions({ ...sprite, width: 80, height: 45, columns: 5 }, { mobile: false })).toMatchObject({
      width: 80,
      height: 45,
      columns: 5,
    });
  });
  it("is off without a usable sprite, on phones, and for a clip starting after 0", () => {
    expect(spriteOptions(undefined, { mobile: false })).toBeUndefined();
    expect(spriteOptions({ url: "", interval: 5 }, { mobile: false })).toBeUndefined();
    expect(spriteOptions({ url: "/s.jpg", interval: 0 }, { mobile: false })).toBeUndefined();
    expect(spriteOptions(sprite, { mobile: true })).toBeUndefined();
    expect(spriteOptions(sprite, { mobile: false, clip: { start: 30 } })).toBeUndefined();
    expect(spriteOptions(sprite, { mobile: false, clip: { start: 0, end: 30 } })).toBeDefined();
  });
});

describe("markTimeTooltip", () => {
  it("classes the time tooltip, and tolerates a player without one", () => {
    const el = document.createElement("div");
    const leaf = { getChild: () => undefined, el: () => el };
    markTimeTooltip({ getChild: () => ({ getChild: () => ({ getChild: () => ({ getChild: () => ({ getChild: () => leaf, el: () => el }), el: () => el }), el: () => el }), el: () => el }), el: () => el });
    expect(el.classList.contains("vjs-sprite-thumb")).toBe(true);
    expect(() => markTimeTooltip({ getChild: () => undefined, el: () => el })).not.toThrow();
  });
});
