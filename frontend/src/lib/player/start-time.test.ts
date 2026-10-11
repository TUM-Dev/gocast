import { describe, expect, it } from "vitest";

import { fakePlayer } from "@/test/fake-player";

import { applyStartTime, parseNumberParam } from "./start-time";

describe("applyStartTime", () => {
  it("seeks on loadedmetadata, every time a source loads", () => {
    const { player, state } = fakePlayer();
    applyStartTime(player, 42, false);
    expect(state.time).toBe(0);
    player.trigger("loadedmetadata");
    expect(state.time).toBe(42);
    state.time = 5;
    player.trigger("loadedmetadata");
    expect(state.time).toBe(42);
  });

  it("on iOS seeks once more when it can play, but only once", () => {
    const { player, state } = fakePlayer();
    applyStartTime(player, 42, true);
    player.trigger("loadedmetadata");
    state.time = 0;
    player.trigger("canplaythrough");
    expect(state.time).toBe(42);
    state.time = 60;
    player.trigger("canplaythrough");
    expect(state.time).toBe(60);
  });

  it("ignores canplaythrough elsewhere and stops when released", () => {
    const { player, state } = fakePlayer();
    const stop = applyStartTime(player, 42, false);
    player.trigger("canplaythrough");
    expect(state.time).toBe(0);
    stop();
    player.trigger("loadedmetadata");
    expect(state.time).toBe(0);
  });
});

describe("parseNumberParam", () => {
  it("accepts non-negative numbers, also as a first array entry", () => {
    expect(parseNumberParam("90")).toBe(90);
    expect(parseNumberParam("1.5")).toBe(1.5);
    expect(parseNumberParam(["30", "40"])).toBe(30);
    expect(parseNumberParam("0")).toBe(0);
  });
  it("rejects the rest", () => {
    for (const bad of [undefined, null, "", " ", "abc", "-3", "Infinity", []]) expect(parseNumberParam(bad)).toBeUndefined();
  });
});
