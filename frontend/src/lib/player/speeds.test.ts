import { describe, expect, it, vi } from "vitest";

import { fakePlayer } from "@/test/fake-player";

import { keepRateAcrossSources, menuRates, stepRate } from "./speeds";

describe("menuRates", () => {
  it("sorts, de-duplicates and drops non-positive speeds", () => {
    expect(menuRates([2, 0.5, 1, 1, 0, -1], false)).toEqual([0.5, 1, 2]);
  });
  it("caps Safari at 2x", () => {
    expect(menuRates([1, 2, 2.5, 3], true)).toEqual([1, 2]);
    expect(menuRates([1, 2, 2.5, 3], false)).toEqual([1, 2, 2.5, 3]);
  });
});

describe("stepRate", () => {
  const rates = [0.5, 1, 1.5, 2];
  it("steps to the neighbour", () => {
    expect(stepRate(rates, 1, true)).toBe(1.5);
    expect(stepRate(rates, 1, false)).toBe(0.5);
  });
  it("stops at the ends", () => {
    expect(stepRate(rates, 2, true)).toBeUndefined();
    expect(stepRate(rates, 0.5, false)).toBeUndefined();
  });
  it("steps from a rate that is not in the list", () => {
    expect(stepRate(rates, 1.1, true)).toBe(1.5);
    expect(stepRate(rates, 1.1, false)).toBe(1);
  });
});

describe("keepRateAcrossSources", () => {
  it("keeps the default rate in step with the rate", () => {
    const { player } = fakePlayer();
    const setDefault = vi.fn();
    const p = Object.assign(player, { defaultPlaybackRate: setDefault });
    p.playbackRate(1.5);
    const stop = keepRateAcrossSources(p);
    expect(setDefault).toHaveBeenLastCalledWith(1.5);
    p.playbackRate(2);
    expect(setDefault).toHaveBeenLastCalledWith(2);
    stop();
    p.playbackRate(1);
    expect(setDefault).toHaveBeenLastCalledWith(2);
  });
  it("tolerates a player without it", () => {
    expect(() => keepRateAcrossSources(fakePlayer().player)).not.toThrow();
  });
});
