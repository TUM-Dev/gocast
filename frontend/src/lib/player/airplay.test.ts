import { describe, expect, it, vi } from "vitest";

import { enableAirPlay, registerAirPlay } from "./airplay";

describe("airplay", () => {
  it("registers the plugin once", () => {
    const plugin = vi.fn();
    const vjs = {};
    registerAirPlay(vjs, plugin);
    registerAirPlay(vjs, plugin);
    expect(plugin).toHaveBeenCalledOnce();
    expect(plugin).toHaveBeenCalledWith(vjs);
  });

  it("puts the button second from the right", () => {
    const airPlay = vi.fn();
    enableAirPlay({ airPlay });
    expect(airPlay).toHaveBeenCalledWith({ addButtonToControlBar: true, buttonPositionIndex: -2 });
    expect(() => enableAirPlay({})).not.toThrow();
  });
});
